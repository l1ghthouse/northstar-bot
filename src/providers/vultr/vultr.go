package vultr

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/bramvdbogaerde/go-scp"
	"github.com/vultr/govultr/v2"
	"log"
	"net"
	"strings"
	"time"

	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers/util"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

const (
	ubuntuDockerImageID = 37
)

type Config struct {
	APIKey   string `required:"true"`
	Tag      string `default:"ephemeral"`
	LogLimit uint   `default:"7340032"`
}

type Vultr struct {
	key      string
	Tags     []string
	LogLimit uint
}

func (v Vultr) CreateServer(ctx context.Context, server *nsserver.NSServer) error {
	vClient := newVultrClient(ctx, v.key)
	region, err := vClient.getVultrRegionByCity(ctx, server.Region)
	if err != nil {
		return err
	}
	server.Region = region.City
	err = vClient.createNorthstarInstance(ctx, server, region.ID, v.Tags)
	if err != nil {
		return err
	}
	return nil
}

func (v Vultr) RestartServer(ctx context.Context, server *nsserver.NSServer) error {
	c := newVultrClient(ctx, v.key)

	return c.restartNorthstarInstance(ctx, server.Name, server.DefaultPassword, v.Tags, server.BareMetal)
}

func (v Vultr) DeleteServer(ctx context.Context, server *nsserver.NSServer) error {
	c := newVultrClient(ctx, v.key)

	err := c.deleteNorthstarInstance(ctx, server.Name, v.Tags)
	var err2 error
	if err != nil {
		err2 = c.deleteBareMetalInstance(ctx, server.Name, v.Tags)
		if err2 != nil {
			return fmt.Errorf("failed to delete server: Error during instance delete:%v.\n Error during bare metal server delete:%v", err, err2)
		}
	}
	return nil
}

func (v Vultr) GetRunningServers(ctx context.Context) ([]*nsserver.NSServer, error) {
	vClient := newVultrClient(ctx, v.key)
	instances, err := vClient.getVultrInstances(ctx, v.Tags)
	if err != nil {
		return nil, err
	}
	bareMetalInstances, err := vClient.getVultrBareMetalServers(ctx, v.Tags)
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

	for _, instance := range bareMetalInstances {
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
					BareMetal: true,
				})
			}
		}
	}

	return ns, nil
}

func (v Vultr) ExtractServerLogs(ctx context.Context, server *nsserver.NSServer) (*bytes.Buffer, error) {
	vClient := newVultrClient(ctx, v.key)

	return vClient.extractServerLogs(ctx, server.Name, server.DefaultPassword, v.Tags, v.LogLimit, server.BareMetal)
}

func NewVultrProvider(cfg Config) (*Vultr, error) {
	return &Vultr{key: cfg.APIKey, Tags: []string{cfg.Tag}, LogLimit: cfg.LogLimit}, nil
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

var user = "root"
var activeStatus = "active"
var sshPort = "22"

func generateSSHClient(mainIP, password string) (*ssh.Client, error) {
	if password == "" {
		return nil, fmt.Errorf("password is empty")
	}

	//nolint:gosec
	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	var sshClient *ssh.Client
	var err error

	for i := 1; i <= 5; i++ {
		sshClient, err = ssh.Dial("tcp", fmt.Sprintf("%s:%s", mainIP, sshPort), sshConfig)
		if err != nil {
			log.Printf("failed to dial: %v", err)
			if i != 10 {
				log.Printf("retrying in 5 seconds")
				time.Sleep(5 * time.Second)
			}
		} else {
			return sshClient, nil
		}
	}
	return nil, fmt.Errorf("unable to connect to vultr instance: %w", err)
}

func (v *vultrClient) extractServerLogs(ctx context.Context, serverName string, password string, tags []string, logLimit uint, bareMetal bool) (*bytes.Buffer, error) {
	var status string
	var mainIP string
	if bareMetal {
		server, err := v.getBareMetalByName(ctx, serverName, tags)
		if err != nil {
			return nil, fmt.Errorf("unable to get bare metal server by name: %w", err)
		}
		status = server.Status
		mainIP = server.MainIP
	} else {
		instance, err := v.getVultrInstanceByName(ctx, serverName, tags)
		if err != nil {
			return nil, fmt.Errorf("unable to get vultr instance by name: %w", err)
		}
		status = instance.Status
		mainIP = instance.MainIP
	}

	if status != activeStatus {
		return nil, fmt.Errorf("vultr instance is not active")
	}

	sshClient, err := generateSSHClient(mainIP, password)
	if err != nil {
		return nil, err
	}
	defer func(sshClient *ssh.Client) {
		err := sshClient.Close()
		if err != nil {
			log.Printf("failed to close ssh client: %v", err)
		}
	}(sshClient)

	sshSession, err := sshClient.NewSession()
	if err != nil {
		return nil, fmt.Errorf("unable to create ssh session: %w", err)
	}
	output, err := sshSession.CombinedOutput(util.FormatLogExtractionScript())
	if err != nil {
		return nil, fmt.Errorf("unable to extract logs: %w, output: %s", err, string(output))
	}

	buffer := bytes.NewBuffer(nil)

	file := &util.CappedBuffer{
		Cap:   int(logLimit), // 7MB
		MyBuf: buffer,
	}

	scpClient, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		return nil, fmt.Errorf("unable to create scp client: %w", err)
	}
	defer scpClient.Close()
	err = scpClient.CopyFromRemotePassThru(ctx, file, util.RemoteFile, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to copy logs from remote: %w", err)
	}

	return file.MyBuf, nil
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

func (v *vultrClient) getVultrInstances(ctx context.Context, tags []string) ([]govultr.Instance, error) {
	list, _, err := v.client.Instance.List(ctx, &govultr.ListOptions{Tag: tags[0]})
	if err != nil {
		return nil, fmt.Errorf("unable to list instances: %w", err)
	}
	return list, nil
}

func (v *vultrClient) getVultrBareMetalServers(ctx context.Context, tags []string) ([]govultr.BareMetalServer, error) {
	list, _, err := v.client.BareMetalServer.List(ctx, &govultr.ListOptions{Tag: tags[0]})
	if err != nil {
		return nil, fmt.Errorf("unable to list instances: %w", err)
	}
	return list, nil
}

func (v *vultrClient) getVultrInstanceByName(ctx context.Context, serverName string, tags []string) (*govultr.Instance, error) {
	instances, err := v.getVultrInstances(ctx, tags)
	if err != nil {
		return nil, err
	}
	for _, instance := range instances {
		if instance.Label == serverName {
			return &instance, nil
		}
	}
	return nil, fmt.Errorf("no instance found for %s", serverName)
}

func (v *vultrClient) getBareMetalByName(ctx context.Context, serverName string, tags []string) (*govultr.BareMetalServer, error) {
	instances, err := v.getVultrBareMetalServers(ctx, tags)
	if err != nil {
		return nil, err
	}
	for _, instance := range instances {
		if instance.Label == serverName {
			return &instance, nil
		}
	}
	return nil, fmt.Errorf("no instance found for %s", serverName)
}

var errTimedOutToReceivePublicIP = errors.New("timed out to receive public IP")
var vultrPlans = []string{"vc2-4c-8gb", "vhp-4c-8gb-intel", "vhp-4c-8gb-amd"}
var bareMetalPlans = []string{"vbm-4c-32gb", "vbm-6c-32gb"}

func (v *vultrClient) createNorthstarInstance(ctx context.Context, server *nsserver.NSServer, regionID string, tags []string) error {
	// Create a base64 encoded script that will: Download northstar container, and Titanfall2 files from git, to startup the server

	s, err := util.FormatStartupScript(ctx, server, "Northstar bot managed by https://github.com/l1ghthouse/northstar-bot", server.Insecure)
	if err != nil {
		return fmt.Errorf("failed to generate formatted script: %w", err)
	}

	cmd := base64.StdEncoding.EncodeToString([]byte(s))

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

	var dateCreated string
	var defaultPassword string

	if server.BareMetal {
		var bareMetalInstance *govultr.BareMetalServer
		for _, plan := range bareMetalPlans {
			instanceOptions := &govultr.BareMetalCreate{
				Region:          regionID,
				Plan:            plan, // One of low-end bare metal server plans
				Label:           server.Name,
				AppID:           ubuntuDockerImageID,
				UserData:        cmd,          // Command to pull docker container, and create a server
				StartupScriptID: resScript.ID, // Startup script
				Tags:            tags,         // ephemeral is used to autodelete the instance after some time
			}

			bareMetalInstance, err = v.client.BareMetalServer.Create(ctx, instanceOptions)
			if err == nil {
				break
			}
		}

		if err != nil {
			return fmt.Errorf("unable to create bare metal instance: %w", err)
		}

		dateCreated = bareMetalInstance.DateCreated
		defaultPassword = bareMetalInstance.DefaultPassword

	} else {
		var instance *govultr.Instance
		for _, plan := range vultrPlans {
			instanceOptions := &govultr.InstanceCreateReq{
				Region:   regionID,
				Plan:     plan, // One of: 4cpu, 8gb plan until single core is supported. More info: https://www.vultr.com/api/#operation/list-os
				Label:    server.Name,
				AppID:    ubuntuDockerImageID,
				UserData: cmd,          // Command to pull docker container, and create a server
				ScriptID: resScript.ID, // Startup script
				Tags:     tags,         // ephemeral is used to autodelete the instance after some time
			}

			instance, err = v.client.Instance.Create(ctx, instanceOptions)
			if err == nil {
				break
			}
		}

		if err != nil {
			return fmt.Errorf("unable to create instance: %w", err)
		}

		dateCreated = instance.DateCreated
		defaultPassword = instance.DefaultPassword

	}

	server.CreatedAt, err = time.Parse(time.RFC3339, dateCreated)
	if err != nil {
		return fmt.Errorf("failed to parse date: %w", err)
	}

	server.DefaultPassword = defaultPassword

	var maxWait <-chan time.Time

	ticker := time.NewTicker(30 * time.Second)
	if server.BareMetal {
		maxWait = time.After(time.Minute * 14)
	} else {
		maxWait = time.After(time.Minute * 5)
	}

	defer ticker.Stop()
	for {
		select {
		case <-maxWait:
			return errTimedOutToReceivePublicIP
		case <-ticker.C:
			var mainIP string
			if server.BareMetal {
				bareMetalInstance, err := v.getBareMetalByName(ctx, server.Name, tags)
				if err != nil {
					return err
				}
				mainIP = bareMetalInstance.MainIP

			} else {
				instance, err := v.getVultrInstanceByName(ctx, server.Name, tags)
				if err != nil {
					return err
				}
				mainIP = instance.MainIP
			}
			ip := net.ParseIP(mainIP)
			if ip.IsUnspecified() {
				continue
			}
			server.MainIP = ip.String()
			return nil
		}
	}

}

func (v *vultrClient) listStartupScripts(ctx context.Context) ([]govultr.StartupScript, error) {
	scripts, _, err := v.client.StartupScript.List(ctx, &govultr.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to list startup scripts: %w", err)
	}
	return scripts, nil
}

func (v *vultrClient) restartNorthstarInstance(ctx context.Context, serverName string, password string, tags []string, isBareMetal bool) error {
	var status string
	var mainIP string
	if isBareMetal {
		bareMetal, err := v.getBareMetalByName(ctx, serverName, tags)
		if err != nil {
			return fmt.Errorf("unable to get vultr instance by name: %w", err)
		}
		status = bareMetal.Status
		mainIP = bareMetal.MainIP
	} else {
		instance, err := v.getVultrInstanceByName(ctx, serverName, tags)
		if err != nil {
			return fmt.Errorf("unable to get vultr instance by name: %w", err)
		}
		status = instance.Status
		mainIP = instance.MainIP
	}

	if status != activeStatus {
		return fmt.Errorf("vultr instance is not active")
	}

	sshClient, err := generateSSHClient(mainIP, password)
	if err != nil {
		return err
	}

	defer func(sshClient *ssh.Client) {
		err := sshClient.Close()
		if err != nil {
			log.Printf("failed to close ssh client: %s", err)
		}
	}(sshClient)

	sshSession, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("unable to create ssh session: %w", err)
	}
	err = sshSession.Run(util.RestartServerScript())
	if err != nil {
		return fmt.Errorf("unable to restart the server: %w", err)
	}

	return nil
}

func (v *vultrClient) deleteNorthstarInstance(ctx context.Context, serverName string, tags []string) error {

	err := v.deleteBootstrapScripts(ctx, serverName)

	if err != nil {
		return err
	}

	instance, err := v.getVultrInstanceByName(ctx, serverName, tags)
	if err != nil {
		return fmt.Errorf("unable to list running instances: %w", err)
	}
	err = v.client.Instance.Delete(ctx, instance.ID)
	if err != nil {
		return fmt.Errorf("unable to delete instance: %w", err)
	}
	return nil
}

func (v *vultrClient) deleteBareMetalInstance(ctx context.Context, serverName string, tags []string) error {

	err := v.deleteBootstrapScripts(ctx, serverName)
	if err != nil {
		return err
	}

	instance, err := v.getBareMetalByName(ctx, serverName, tags)
	if err != nil {
		return fmt.Errorf("unable to list running bare metal server: %w", err)
	}
	err = v.client.BareMetalServer.Delete(ctx, instance.ID)
	if err != nil {
		return fmt.Errorf("unable to delete bare metal server: %w", err)
	}
	return nil
}

func (v *vultrClient) deleteBootstrapScripts(ctx context.Context, serverName string) error {
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

	return nil
}
