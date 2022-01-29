package util

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v42/github"
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

const dockerImage = "ghcr.io/pg9182/northstar-dedicated:1-tf2.0.11.0-ns1.4.0"
const LTSRebalancedRepoOwner = "Dinorush"
const LTSRebalancedRepoName = "LTSRebalance"
const OptionLTSRebalancedVersion = "lts_rebalanced_version"

var ErrNoLTSRebalancedTags = fmt.Errorf("no LTSRebalanced tags found")

func FormatScript(ctx context.Context, server *nsserver.NSServer, serverDesc string, insecure string) (string, error) {
	OptionalCmd := ""
	DockerArgs := ""
	if server.Options[nsserver.OptionRebalancedLTSMod].(bool) {
		var latestTag *github.RepositoryTag
		client := github.NewClient(nil)
		tags, _, err := client.Repositories.ListTags(ctx, LTSRebalancedRepoOwner, LTSRebalancedRepoName, nil)
		if err != nil {
			return "", fmt.Errorf("error listing tags: %w", err)
		}
		if len(tags) > 0 {
			latestTag = tags[0]
		} else {
			return "", ErrNoLTSRebalancedTags
		}

		server.Options[OptionLTSRebalancedVersion] = latestTag.GetName()

		builder := strings.Builder{}
		builder.WriteString("apt install -y unzip")
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("wget https://github.com/Dinorush/LTSRebalance/releases/download/%s/Dinorush.LTSRebalance_%s.zip", latestTag.GetName(), latestTag.GetName()))
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("unzip Dinorush.LTSRebalance_%s.zip -d /", latestTag.GetName()))
		builder.WriteString("\n")
		OptionalCmd = builder.String()
		DockerArgs = "--mount \"type=bind,source=/Dinorush.LTSRebalance,target=/mnt/mods/Dinorush.LTSRebalance\""
	}

	return fmt.Sprintf(`#!/bin/bash
docker pull %s

apt update -y
apt install parallel jq -y

echo "Downloading Titanfall2 Files"

curl -L "https://ghcr.io/v2/nsres/titanfall/manifests/2.0.11.0-dedicated-mp" -s -H "Accept: application/vnd.oci.image.manifest.v1+json" -H "Authorization: Bearer QQ==" | jq -r '.layers[]|[.digest, .annotations."org.opencontainers.image.title"] | @tsv' |
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

docker run --rm -d --pull always --publish 8081:8081/tcp --publish 37015:37015/udp --mount "type=bind,source=/titanfall2,target=/mnt/titanfall" %s --env NS_SERVER_NAME="[%s]%s" --env NS_SERVER_DESC="%s" --env NS_SERVER_PASSWORD="%d" --env NS_INSECURE="%s" ghcr.io/pg9182/northstar-dedicated:1-tf2.0.11.0
`, dockerImage, OptionalCmd, DockerArgs, server.Region, server.Name, serverDesc, *server.Pin, insecure), nil
}
