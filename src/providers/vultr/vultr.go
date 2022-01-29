package vultr

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/sethvargo/go-password/password"
	"github.com/vultr/govultr/v2"
	"golang.org/x/oauth2"
)

const (
	ubuntuDockerImageID = 37
	PinLength           = 5
)

type Config struct {
	APIKey string `required:"true"`
	Tag    string `default:"ephemeral"`
}

type Vultr struct {
	key string
	Tag string
}

func (v Vultr) CreateServer(ctx context.Context, server *nsserver.NSServer) error {
	vClient := newVultrClient(ctx, v.key)
	region, err := vClient.getVultrRegionByCity(ctx, server.Region)
	if err != nil {
		return err
	}
	server.Region = region.City
	pin, err := strconv.Atoi(password.MustGenerate(PinLength, PinLength, 0, false, true))
	if err != nil {
		return fmt.Errorf("failed to generate pin: %w", err)
	}
	server.Pin = &pin
	err = vClient.createNorthstarInstance(ctx, server, region.ID, v.Tag)
	if err != nil {
		return err
	}
	return nil
}

func (v Vultr) DeleteServer(ctx context.Context, server *nsserver.NSServer) error {
	c := newVultrClient(ctx, v.key)

	return c.deleteNorthstarInstance(ctx, server.Name, v.Tag)
}

func (v Vultr) GetRunningServers(ctx context.Context) ([]*nsserver.NSServer, error) {
	vClient := newVultrClient(ctx, v.key)
	instances, err := vClient.getVultrInstances(ctx, v.Tag)
	if err != nil {
		return nil, err
	}

	regions, err := vClient.listVultrRegion(ctx)
	if err != nil {
		return nil, err
	}

	var ns []*nsserver.NSServer

	for _, instance := range instances {
		for _, region := range regions {
			if instance.Region == region.ID {
				date, err := time.Parse(time.RFC3339, instance.DateCreated)
				if err != nil {
					return nil, fmt.Errorf("failed to parse date: %w", err)
				}

				ns = append(ns, &nsserver.NSServer{
					Name:      instance.Label,
					Region:    region.City,
					CreatedAt: date,
				})
			}
		}
	}

	return ns, nil
}

func NewVultrProvider(cfg Config) (*Vultr, error) {
	return &Vultr{key: cfg.APIKey, Tag: cfg.Tag}, nil
}

func client(ctx context.Context, key string) *govultr.Client {
	// Create a new client with token from .env
	config := &oauth2.Config{}
	ts := config.TokenSource(ctx, &oauth2.Token{AccessToken: key})
	return govultr.NewClient(oauth2.NewClient(ctx, ts))
}

type vultrClient struct {
	client *govultr.Client
}

func newVultrClient(ctx context.Context, apiKey string) *vultrClient {
	return &vultrClient{
		client: client(ctx, apiKey),
	}
}

func (v *vultrClient) getVultrRegionByCity(ctx context.Context, region string) (govultr.Region, error) {
	regions, err := v.listVultrRegion(ctx)
	if err != nil {
		return govultr.Region{}, err
	}
	availableRegions := make([]string, len(regions))

	for i, r := range regions {
		availableRegions[i] = r.City
		if strings.Contains(strings.ToLower(r.City), strings.ToLower(region)) {
			return r, nil
		}
	}

	return govultr.Region{}, fmt.Errorf("no region found for %s. Available regions: %s", region, strings.Join(availableRegions, ", "))
}

func (v *vultrClient) listVultrRegion(ctx context.Context) ([]govultr.Region, error) {
	regions, _, err := v.client.Region.List(ctx, &govultr.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to list regions: %w", err)
	}
	return regions, nil
}

func (v *vultrClient) getVultrInstances(ctx context.Context, tag string) ([]govultr.Instance, error) {
	list, _, err := v.client.Instance.List(ctx, &govultr.ListOptions{Tag: tag})
	if err != nil {
		return nil, fmt.Errorf("unable to list instances: %w", err)
	}
	return list, nil
}

const dockerImage = "ghcr.io/pg9182/northstar-dedicated:1-tf2.0.11.0-ns1.4.0"

func formatScript(dockerImage, serverRegion, serverName, serverDesc string, serverPin int, insecure string) string {
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

docker run --rm -d --pull always --publish 8081:8081/tcp --publish 37015:37015/udp --mount "type=bind,source=/titanfall2,target=/mnt/titanfall" --env NS_SERVER_NAME="[%s]%s" --env NS_SERVER_DESC="%s" --env NS_SERVER_PASSWORD="%d" --env NS_INSECURE="%s" ghcr.io/pg9182/northstar-dedicated:1-tf2.0.11.0
`, dockerImage, serverRegion, serverName, serverDesc, serverPin, insecure)
}

func (v *vultrClient) createNorthstarInstance(ctx context.Context, server *nsserver.NSServer, regionID string, tag string) error {
	// Create a base64 encoded script that will: Download northstar container, and Titanfall2 files from git, to startup the server
	cmd := base64.StdEncoding.EncodeToString([]byte(formatScript(dockerImage, server.Region, server.Name, "Competitive LTS!! Yay!", *server.Pin, "1")))

	script := &govultr.StartupScriptReq{
		Name:   server.Name,
		Type:   "boot",
		Script: cmd,
	}

	// Docker image doesn't have cloud-init, so we will instead create a custom script first
	resScript, err := v.client.StartupScript.Create(ctx, script)
	if err != nil {
		return fmt.Errorf("unable to create startup script: %w", err)
	}

	instanceOptions := &govultr.InstanceCreateReq{
		Region:   regionID,
		Plan:     "vc2-4c-8gb", // 4cpu, 8gb plan until single core is supported. More info: https://www.vultr.com/api/#operation/list-os
		Label:    server.Name,
		AppID:    ubuntuDockerImageID,
		UserData: cmd,          // Command to pull docker container, and create a server
		ScriptID: resScript.ID, // Startup script
		Tag:      tag,          // ephemeral is used to autodelete the instance after some time
	}

	instance, err := v.client.Instance.Create(ctx, instanceOptions)
	if err != nil {
		return fmt.Errorf("unable to create instance: %w", err)
	}

	server.CreatedAt, err = time.Parse(time.RFC3339, instance.DateCreated)
	if err != nil {
		return fmt.Errorf("failed to parse date: %w", err)
	}
	return nil
}

func (v *vultrClient) listStartupScripts(ctx context.Context) ([]govultr.StartupScript, error) {
	scripts, _, err := v.client.StartupScript.List(ctx, &govultr.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to list startup scripts: %w", err)
	}
	return scripts, nil
}

func (v *vultrClient) deleteNorthstarInstance(ctx context.Context, serverName string, tag string) error {
	instances, err := v.getVultrInstances(ctx, tag)
	if err != nil {
		return fmt.Errorf("unable to list running instances: %w", err)
	}

	scripts, err := v.listStartupScripts(ctx)
	if err != nil {
		return fmt.Errorf("unable to list startup scripts: %w", err)
	}

	for _, script := range scripts {
		if script.Name == serverName {
			err = v.client.StartupScript.Delete(ctx, script.ID)
			if err != nil {
				log.Printf("unable to delete startup script: %v", err)
			}
		}
	}

	for _, instance := range instances {
		if instance.Label == serverName {
			err = v.client.Instance.Delete(ctx, instance.ID)
			if err != nil {
				return fmt.Errorf("unable to delete instance: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("no instance found for %s", serverName)
}
