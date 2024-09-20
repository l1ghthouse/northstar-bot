package util

import (
	"al.essio.dev/pkg/shellescape"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"golang.org/x/crypto/ssh"
	"log"
	"regexp"

	"github.com/l1ghthouse/northstar-bootstrap/src/mod"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/lucasepe/codename"
)

// CreateFunnyName generates a docker like name.
func CreateFunnyName() string {
	rng, err := codename.DefaultRNG()
	if err != nil {
		log.Fatalf("Error creating random number generator: %v", err)
	}
	return codename.Generate(rng, 0)
}

type DockerVersion struct {
	IsLatest    bool
	DockerImage string
}

func LatestStableDockerNorthstar() (string, string) {
	for k, v := range NorthstarVersions {
		if v.IsLatest {
			return k, v.DockerImage
		}
	}
	return "", ""
}

const NorthstarDedicatedRepo = "ghcr.io/pg9182/"

var DockerTagRegexp = regexp.MustCompile("^(northstar-dedicated|northstar-dedicated-ci|northstar-dedicated-dev):([a-zA-Z0-9_.-]{1,128})$")

var NorthstarVersions = map[string]DockerVersion{
	"1.28.1": {
		IsLatest:    true,
		DockerImage: NorthstarDedicatedRepo + "northstar-dedicated:1-tf2.0.11.0-ns1.28.1",
	},
}

// checks that one, and only one latest version is set to true
func init() {
	hasLatest := false
	for _, v := range NorthstarVersions {
		if v.IsLatest {
			if hasLatest {
				log.Fatalf("Multiple latest versions found")
			}
			hasLatest = true
		}
	}
	if !hasLatest {
		log.Fatalf("No latest version found")
	}
}

const containerName = "northstar-dedicated"
const VersionPostfix = "_version"
const LinkPostfix = "_link"
const RequiredByClientPostfix = "_clientRequired"

const optimizedServerFiles = "https://ghcr.io/v2/nsres/titanfall/manifests/2.0.11.0-dedicated-mp-vpkoptim.430d3bb"

func RestartServerScript() string {
	return fmt.Sprintf("docker restart %s", containerName)
}

func Btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func FormatStartupScript(ctx context.Context, server *nsserver.NSServer, serverDesc string, insecure bool) (string, error) {
	OptionalCmd := ""
	DockerArgs := ""
	var mergeOptions = make(map[string]interface{})

	var modsMap = make(map[string]mod.Mod)

	for option, value := range server.ModOptions {
		knownMod := false
		for modName, generator := range mod.ByName {
			if option == modName {
				knownMod = true
				if value.(bool) {
					modsMap[modName] = generator()
				}
			}
		}
		if !knownMod {
			modsMap[option] = mod.ThunderstoreMod{
				Enabled: false,
				Name:    option,
			}
		}
	}

	var modsArr []mod.Mod
	for _, m := range modsMap {
		modsArr = append(modsArr, m)
	}

	for name, m := range modsMap {
		err := m.Validate(modsArr)
		if err != nil {
			return "", err
		}
		cmd, args, link, tag, requiredByClient, err := m.ModParams(ctx)
		if err != nil {
			return "", fmt.Errorf("error generating mod: %w", err)
		}
		OptionalCmd = OptionalCmd + "\n" + cmd
		DockerArgs = DockerArgs + " " + args + " "
		mergeOptions[name+VersionPostfix] = tag
		mergeOptions[name+LinkPostfix] = link
		mergeOptions[name+RequiredByClientPostfix] = requiredByClient
	}

	var extraArgs string

	if server.TickRate != 0 {
		extraArgs += fmt.Sprintf(" +cl_updaterate_mp %d +sv_updaterate_mp %d +cl_cmdrate %d +sv_minupdaterate %d +sv_maxupdaterate %d +sv_max_snapshots_multiplayer %d +base_tickinterval_mp %.5f",
			server.TickRate, server.TickRate, server.TickRate, server.TickRate, server.TickRate, server.TickRate*15, 1/float64(server.TickRate))
	}

	if server.EnableCheats {
		extraArgs += fmt.Sprintf(" +sv_cheats 1")
	}

	if server.ExtraArgs != "" {
		extraArgs += " " + server.ExtraArgs
	}

	extraArgs += " +ns_allow_spectators 1"

	extraArgs = shellescape.Quote(extraArgs)

	for k, v := range mergeOptions {
		server.ModOptions[k] = v
	}

	serverFiles := optimizedServerFiles

	return fmt.Sprintf(`#!/bin/bash
export IMAGE=%s
export NS_AUTH_PORT="%d"
export NS_PORT="%d"
export NS_MASTERSERVER_URL="%s"
export NS_SERVER_PASSWORD="%s"
export NS_INSECURE="%d"
export NS_SERVER_REGION="%s"
export NS_NAME="%s"
export NS_SERVER_NAME="[$NS_SERVER_REGION]$NS_NAME"
export NS_SERVER_DESC="%s"
export NS_EXTRA_ARGUMENTS=%s

docker pull $IMAGE

apt update -y
apt install parallel jq unzip zip -y

curl -fsSL https://get.docker.com -o get-docker.sh
sh ./get-docker.sh &

mkdir /mods

%s

echo "Downloading Titanfall2 Files"

curl -L "%s" -s -H "Accept: application/vnd.oci.image.manifest.v1+json" -H "Authorization: Bearer QQ==" | jq -r '.layers[]|[.digest, .annotations."org.opencontainers.image.title"] | @tsv' |
{
  paths=()
  uri=()
  while read -r line; do
    while IFS=$'\t' read -r digest path; do
      path="/titanfall2/$path"
      folder=${path%%/*}
      mkdir -p "$folder"
      touch "$path"
      paths+=("$path")
      uri+=("https://ghcr.io/v2/nsres/titanfall/blobs/$digest")
    done <<< "$line" ;
  done
  parallel --link --jobs 8 'wget -O {1} {2} --header="Authorization: Bearer QQ==" -nv' ::: "${paths[@]}" ::: "${uri[@]}"
}

#Wait for docker to finish downloading
wait

docker ps -a

#Some random sleep
sleep 5

docker run -d --pull always --restart always --log-driver json-file --log-opt max-size=200m --publish $NS_AUTH_PORT:$NS_AUTH_PORT/tcp --publish $NS_PORT:$NS_PORT/udp --mount "type=bind,source=/titanfall2,target=/mnt/titanfall,readonly" --mount "type=bind,source=/mods,target=/mnt/mods,readonly" %s --env NS_SERVER_NAME --env NS_MASTERSERVER_URL --env NS_SERVER_DESC --env NS_EXTRA_ARGUMENTS --env NS_AUTH_PORT --env NS_PORT --env NS_SERVER_PASSWORD --env NS_INSECURE --name "%s" $IMAGE
`, server.DockerImageVersion, server.AuthTCPPort, server.GameUDPPort, server.MasterServer, server.Pin, Btoi(insecure), server.Region, server.Name, serverDesc, extraArgs, OptionalCmd, serverFiles, DockerArgs, containerName), nil
}

var RemoteFile = "/extract.zip"

func FormatLogExtractionScript() string {
	return fmt.Sprintf(`#!/bin/bash
set -e
rm -rf /extract*
CONTAINER_NAME=%s
mkdir -p /extract-tmp/
docker logs --details --timestamps $CONTAINER_NAME &> /extract-tmp/northstar.log
zip -j %s /extract-tmp/*
`, containerName, RemoteFile)
}

type CappedBuffer struct {
	Cap   int
	MyBuf *bytes.Buffer
}

var ErrBufferCapacityExceeded = fmt.Errorf("buffer capacity exceeded. File too large")

func (b *CappedBuffer) Write(content []byte) (n int, err error) {
	if len(content)+b.MyBuf.Len() > b.Cap {
		return 0, ErrBufferCapacityExceeded
	}
	b.MyBuf.Write(content)
	return len(content), nil
}

// GeneratePrivateKey creates a RSA Private Key of specified byte size
func GeneratePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	log.Println("Private Key generated")
	return privateKey, nil
}

// EncodePrivateKeyToPEM encodes Private Key from RSA to PEM format
func EncodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	// Get ASN.1 DER format
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// pem.Block
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	// Private key in PEM format
	privatePEM := pem.EncodeToMemory(&privBlock)

	return privatePEM
}

// GeneratePublicKey take a rsa.PublicKey and return bytes suitable for writing to .pub file
// returns in the format "ssh-rsa ..."
func GeneratePublicKey(privatekey *rsa.PublicKey) ([]byte, error) {
	publicRsaKey, err := ssh.NewPublicKey(privatekey)
	if err != nil {
		return nil, err
	}

	pubKeyBytes := ssh.MarshalAuthorizedKey(publicRsaKey)

	log.Println("Public key generated")
	return pubKeyBytes, nil
}
