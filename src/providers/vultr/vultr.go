package vultr

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers/util"
	"github.com/sethvargo/go-password/password"
	"github.com/vultr/govultr/v2"
	"golang.org/x/oauth2"
	"strings"
)

const ephemeral = "ephemeral"

type Config struct {
	APIKey string `required:"true"`
}

type Vultr struct {
	key string
}

func (v Vultr) CreateServer(ctx context.Context, server nsserver.NSServer) (nsserver.NSServer, error) {
	c := newVultrClient(ctx, v.key)
	region, err := c.getVultrRegionByCity(server.Region)
	if err != nil {
		return nsserver.NSServer{}, err
	}
	server.Region = region.City
	server.Name = util.CreateFunnyName()
	server.Password = password.MustGenerate(5, 5, 0, false, true)
	err = c.createNorthstarInstance(server, region.ID)
	if err != nil {
		return nsserver.NSServer{}, err
	}
	return server, nil
}

func (v Vultr) DeleteServer(ctx context.Context, server nsserver.NSServer) error {
	panic("implement me")
}

func (v Vultr) GetRunningServers(ctx context.Context) ([]nsserver.NSServer, error) {
	c := newVultrClient(ctx, v.key)
	instances, err := c.getVultrInstances()
	if err != nil {
		return nil, err
	}

	regions, err := c.listVultrRegion()
	if err != nil {
		return nil, err
	}

	var ns []nsserver.NSServer

	for _, instance := range instances {
		for _, region := range regions {
			if instance.Region == region.ID {
				ns = append(ns, nsserver.NSServer{
					Name:   instance.Label,
					Region: region.City,
				})
			}
		}
	}

	return ns, nil
}

func NewVultrProvider(cfg Config) (*Vultr, error) {
	return &Vultr{key: cfg.APIKey}, nil
}

func client(ctx context.Context, key string) *govultr.Client {
	// Create a new client with token from .env
	config := &oauth2.Config{}
	ts := config.TokenSource(ctx, &oauth2.Token{AccessToken: key})
	return govultr.NewClient(oauth2.NewClient(ctx, ts))
}

type vultrClient struct {
	client *govultr.Client
	ctx    context.Context
}

func newVultrClient(ctx context.Context, apiKey string) *vultrClient {
	return &vultrClient{
		client: client(ctx, apiKey),
		ctx:    ctx,
	}
}

func (v *vultrClient) getVultrRegionByCity(region string) (govultr.Region, error) {
	var availableRegions []string
	if regions, err := v.listVultrRegion(); err != nil {
		return govultr.Region{}, err
	} else {
		for _, r := range regions {
			availableRegions = append(availableRegions, r.City)
			if strings.Contains(strings.ToLower(region), strings.ToLower(r.City)) {
				return r, nil
			}
		}
	}

	return govultr.Region{}, fmt.Errorf("no region found for %s. Avaliable regions: %+v", region, strings.Join(availableRegions, ", "))
}

func (v *vultrClient) listVultrRegion() ([]govultr.Region, error) {
	regions, _, err := v.client.Region.List(v.ctx, &govultr.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to list regions: %w", err)
	}
	return regions, nil
}

func (v *vultrClient) getVultrInstances() ([]govultr.Instance, error) {
	list, _, err := v.client.Instance.List(v.ctx, &govultr.ListOptions{Tag: ephemeral})
	if err != nil {
		return nil, err
	}
	return list, nil

}

func (v *vultrClient) createNorthstarInstance(server nsserver.NSServer, regionID string) error {
	// Create a base64 encoded script that will: Download northstar container, and Titanfall2 files from git, to startup the server
	cmd := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`#!/bin/bash
docker pull ghcr.io/pg9182/northstar-dedicated:1-tf2.0.11.0

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

docker run --rm -d --pull always --publish 8081:8081/tcp --publish 37015:37015/udp --mount "type=bind,source=/titanfall2,target=/mnt/titanfall" --env NS_SERVER_NAME="[%s]%s" --env NS_SERVER_DESC="%s" --env NS_SERVER_PASSWORD="%s" --env NS_INSECURE="%s" ghcr.io/pg9182/northstar-dedicated:1-tf2.0.11.0

`, server.Region, server.Name, "Competitive LTS!! Yay!", server.Password, "1")))

	script := &govultr.StartupScriptReq{
		Name:   server.Name,
		Type:   "boot",
		Script: cmd,
	}

	// Docker image doesn't have cloud-init, so we will instead create a custom script first
	resScript, err := v.client.StartupScript.Create(v.ctx, script)
	if err != nil {
		return fmt.Errorf("unable to create startup script: %w", err)
	}

	instanceOptions := &govultr.InstanceCreateReq{
		Region:   regionID,
		Plan:     "vc2-4c-8gb", // 4cpu, 8gb plan until single core is supported. More info: https://www.vultr.com/api/#operation/list-os
		Label:    server.Name,
		AppID:    37,           // Ubuntu Docker
		UserData: cmd,          // Command to pull docker container, and create a server
		ScriptID: resScript.ID, // Startup script
		Tag:      ephemeral,    // ephemeral is used to autodelete the instance after some time
	}

	_, err = v.client.Instance.Create(context.Background(), instanceOptions)
	if err != nil {
		return fmt.Errorf("unable to create instance: %w", err)
	}
	return nil
}
