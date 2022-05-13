package util

import (
	"bytes"
	"context"
	"fmt"
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

var DockerTagRegexp = regexp.MustCompile("^(northstar-dedicated|northstar-dedicated-dev):([a-zA-Z0-9_.-]{1,128})$")

var NorthstarVersions = map[string]DockerVersion{
	"1.7.0": {
		IsLatest:    true,
		DockerImage: NorthstarDedicatedRepo + "northstar-dedicated:1.20220502.git04c1bf6-tf2.0.11.0-ns1.7.0", // "ghcr.io/pg9182/northstar-dedicated:1-tf2.0.11.0-ns1.7.0",
	},
	"1.6.4": {
		IsLatest:    false,
		DockerImage: NorthstarDedicatedRepo + "northstar-dedicated:1-tf2.0.11.0-ns1.6.4",
	},
	"1.6.3": {
		IsLatest:    false,
		DockerImage: NorthstarDedicatedRepo + "northstar-dedicated:1-tf2.0.11.0-ns1.6.3",
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

const vanillaServerFiles = "https://ghcr.io/v2/nsres/titanfall/manifests/2.0.11.0-dedicated-mp"
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
	for serverOptions, v := range server.Options {
		for modName, generator := range mod.ByName {
			if serverOptions == modName && v.(bool) {
				m := generator()
				cmd, args, link, tag, requiredByClient, err := m.ModParams(ctx)
				if err != nil {
					return "", fmt.Errorf("error generating mod: %w", err)
				}
				OptionalCmd = OptionalCmd + "\n" + cmd
				DockerArgs = DockerArgs + " " + args + " "
				server.Options[serverOptions+VersionPostfix] = tag
				server.Options[serverOptions+LinkPostfix] = link
				server.Options[serverOptions+RequiredByClientPostfix] = requiredByClient
			}
		}
	}

	serverFiles := vanillaServerFiles
	if server.OptimizedServerFiles {
		serverFiles = optimizedServerFiles
	}

	return fmt.Sprintf(`#!/bin/bash
export IMAGE=%s
export NS_AUTH_PORT="%d"
export NS_PORT="%d"
export NS_MASTERSERVER_URL="%s"
export NS_SERVER_PASSWORD="%s"
export NS_INSECURE="%d"
export NS_SERVER_NAME="[%s]%s"
export NS_SERVER_DESC="%s"

docker pull $IMAGE

apt update -y
apt install parallel jq unzip zip -y

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

%s

docker run -d --pull always --log-driver json-file --log-opt max-size=200m --publish $NS_AUTH_PORT:$NS_AUTH_PORT/tcp --publish $NS_PORT:$NS_PORT/udp --mount "type=bind,source=/titanfall2,target=/mnt/titanfall,readonly" %s --env NS_SERVER_NAME --env NS_MASTERSERVER_URL --env NS_SERVER_DESC --env NS_AUTH_PORT --env NS_PORT --env NS_SERVER_PASSWORD --env NS_INSECURE --name "%s" $IMAGE
`, server.DockerImageVersion, server.AuthTCPPort, server.GameUDPPort, server.MasterServer, server.Pin, Btoi(insecure), server.Region, server.Name, serverDesc, serverFiles, OptionalCmd, DockerArgs, containerName), nil
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
