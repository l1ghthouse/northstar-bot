package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/l1ghthouse/northstar-bootstrap/src/mod"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers/util"

	"github.com/paulbellamy/ratecounter"

	"github.com/sethvargo/go-password/password"

	"github.com/bwmarrin/discordgo"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

const (
	CreateServer   = "create_server"
	ListServer     = "list_servers"
	DeleteServer   = "delete_server"
	ExtractLogs    = "extract_logs"
	RestartServer  = "restart_server"
	ServerMetadata = "server_metadata"
)

func modApplicationCommand() (options []*discordgo.ApplicationCommandOption) {
	for k := range mod.ByName {
		options = append(options, &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        k,
			Description: fmt.Sprintf("Indicated that the server should include %s mod", k),
		})
	}
	return
}

func serverCreateVersionChoices() (options []*discordgo.ApplicationCommandOptionChoice) {
	for k, v := range util.NorthstarVersions {
		options = append(options, &discordgo.ApplicationCommandOptionChoice{
			Value: v.DockerImage,
			Name:  k,
		})
	}
	return
}

const CreateServerOptInsecure = "insecure"
const CreateServerOptMasterServer = "master_server"
const CreateServerVersionOpt = "server_version"
const CreateServerCustomDockerContainerOpt = "custom_container"
const ListServerVerbosityOpt = "verbosity"
const ExtendLifetime = "extend_lifetime"

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        CreateServer,
			Description: "Command to create a server",
			Options: append([]*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "region",
					Description: "region in which the server will be created",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        CreateServerOptInsecure,
					Description: "Whether the server should be created with insecure mode(exposes IP address)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        CreateServerOptMasterServer,
					Description: "Custom Master Server",
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        CreateServerVersionOpt,
					Description: "Version of the server to create. If not specified, the latest version will be used",
					Choices:     serverCreateVersionChoices(),
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        CreateServerCustomDockerContainerOpt,
					Description: "The Custom Docker Container must be under ghcr.io/pg9182/. Format: NAME:TAG",
				},
			}, modApplicationCommand()...),
		},
		{
			Name:        DeleteServer,
			Description: "Command to delete a server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "server name to delete",
					Required:    true,
				},
			},
		},
		{
			Name:        RestartServer,
			Description: "Command to restart the server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "server name to restart",
					Required:    true,
				},
			},
		},
		{
			Name:        ExtractLogs,
			Description: "Command to extract logs from a server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "server name from which logs are to be extracted",
					Required:    true,
				},
			},
		},
		{
			Name:        ListServer,
			Description: "Command to list servers",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        ListServerVerbosityOpt,
					Description: "indicates if the list should be verbose",
					Required:    false,
				},
			},
		},
		{
			Name:        ServerMetadata,
			Description: "Metadata associated with the server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "server name associated with metadata",
					Required:    true,
				},
			},
		},
		{
			Name:        ExtendLifetime,
			Description: "Extends lifetime of the server by a given amount",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "server name associated with metadata",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "duration",
					Description: "Duration by which the server lifetime would be extended",
					Required:    true,
				},
			},
		},
	}
)

type handler struct {
	p                    providers.Provider
	maxConcurrentServers uint
	autoDeleteDuration   time.Duration
	nsRepo               nsserver.Repo
	maxExtendDuration    time.Duration
	maxServerCreateRate  uint
	rateCounter          *ratecounter.RateCounter
	createLock           *sync.Mutex
	notifyer             *Notifyer
}

const unknown = "unknown"
const PinLength = 4

func optionValue(options []*discordgo.ApplicationCommandInteractionDataOption, name string) (*discordgo.ApplicationCommandInteractionDataOption, bool) {
	for _, option := range options {
		if option.Name == name {
			return option, true
		}
	}
	return nil, false
}

func defaultServer(name string, interaction *discordgo.InteractionCreate) (*nsserver.NSServer, error) {
	var modOptions = make(map[string]interface{})
	{
		for modName := range mod.ByName {
			modOptions[modName] = mod.ByName[modName]().EnabledByDefault()
			val, ok := optionValue(interaction.ApplicationCommandData().Options, modName)
			if ok {
				modOptions[modName] = val.BoolValue()
			}
		}
	}

	var isInsecure bool
	{
		val, ok := optionValue(interaction.ApplicationCommandData().Options, CreateServerOptInsecure)
		if ok {
			isInsecure = val.BoolValue()
		} else {
			isInsecure = false
		}
	}

	var masterServer string
	{
		val, ok := optionValue(interaction.ApplicationCommandData().Options, CreateServerOptMasterServer)
		if ok {
			masterServer = val.StringValue()
		} else {
			masterServer = DefaultMasterServer
		}
	}

	var serverVersion string
	var dockerImageVersion string
	{
		valServerVersion, okServerVersion := optionValue(interaction.ApplicationCommandData().Options, CreateServerVersionOpt)
		valTagVersion, okTagVersion := optionValue(interaction.ApplicationCommandData().Options, CreateServerCustomDockerContainerOpt)

		switch {
		case okServerVersion && okTagVersion:
			return nil, fmt.Errorf("cannot specify both /%s and /%s", CreateServerVersionOpt, CreateServerCustomDockerContainerOpt)
		case okServerVersion:
			dockerImageVersion = valServerVersion.StringValue()
			for k, v := range util.NorthstarVersions {
				if v.DockerImage == dockerImageVersion {
					serverVersion = k
					break
				}
			}
		case okTagVersion:
			if util.DockerTagRegexp.MatchString(valTagVersion.StringValue()) {
				dockerImageVersion = util.NorthstarDedicatedRepo + valTagVersion.StringValue()
				serverVersion = unknown
			} else {
				return nil, fmt.Errorf("invalid docker tag: %s. Must match following regex: %s", valTagVersion.StringValue(), util.DockerTagRegexp)
			}
		default:
			serverVersion, dockerImageVersion = util.LatestStableDockerNorthstar()
		}
	}

	pin := password.MustGenerate(PinLength, PinLength, 0, false, true)

	return &nsserver.NSServer{
		Region:             interaction.ApplicationCommandData().Options[0].StringValue(),
		RequestedBy:        interaction.Member.User.ID,
		Name:               name,
		Pin:                pin,
		Options:            modOptions,
		Insecure:           isInsecure,
		ServerVersion:      serverVersion,
		GameUDPPort:        37015,
		AuthTCPPort:        8081,
		DockerImageVersion: dockerImageVersion,
		MasterServer:       masterServer,
	}, nil
}

const DefaultMasterServer = "https://northstar.tf"

func (h *handler) handleCreateServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()

	sendInteractionDeferred(session, interaction)

	h.createLock.Lock()
	defer h.createLock.Unlock()
	if h.maxServerCreateRate != 0 && h.rateCounter.Rate() > int64(h.maxServerCreateRate) {
		editDeferredInteractionReply(session, interaction.Interaction, "You have exceeded the maximum number of servers you can create per hour. Please try again later.")

		return
	}
	servers, err := h.p.GetRunningServers(ctx)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("unable to list running servers: %v", err))

		return
	}
	if len(servers) >= int(h.maxConcurrentServers) {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("You can't create more than %d servers", h.maxConcurrentServers))

		return
	}
	cachedServers, err := h.nsRepo.GetAll(ctx)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("unable to list servers: %v", err))

		return
	}

	name, err := generateUniqueName(servers, cachedServers)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("unable to generate unique server name: %v", err))

		return
	}

	server, err := defaultServer(name, interaction)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("unable to create server: %v", err))
	}

	err = h.p.CreateServer(ctx, server)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to create the target server. error: %v", err))

		return
	}

	if h.maxServerCreateRate != 0 {
		h.rateCounter.Incr(1)
	}

	err = h.nsRepo.Store(ctx, []*nsserver.NSServer{server})
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("unable to save server to the database: %v", err))

		return
	}

	note := strings.Builder{}
	note.WriteString(fmt.Sprintf("Created server **%s** in **%s**, with password: **%s**.", server.Name, server.Region, server.Pin))
	note.WriteString("\n")

	if server.Insecure {
		note.WriteString(fmt.Sprintf("Insecure mode is enabled. If master server is offline, use: `connect %s:%d`", server.MainIP, server.GameUDPPort))
		note.WriteString("\n")
	}

	note.WriteString(fmt.Sprintf("Server version: **%s**", server.ServerVersion))
	timeToSpinUp := 2
	note.WriteString(fmt.Sprintf(". Server will be up in: **%d** minutes", timeToSpinUp))
	if h.autoDeleteDuration != time.Duration(0) {
		note.WriteString(fmt.Sprintf(", and autodeleted in in **%s**", h.autoDeleteDuration.String()))
	}
	note.WriteString("\n")

	modInfo := ""
	modInfoMisc := ""

	for option := range server.Options {
		for modName := range mod.ByName {
			if option == modName && server.Options[option].(bool) {
				if modInfo == "" {
					modInfo = "Following Mods Are Enabled:\n"
				}
				modInfo += fmt.Sprintf(" - %s(version: %s)\n", modName, server.Options[modName+util.VersionPostfix])

				if server.Options[modName+util.RequiredByClientPostfix] == true {
					if modInfoMisc == "" {
						modInfoMisc = "Following mods are **REQUIRED TO BE DOWNLOADED BY CLIENT**:\n"
					}
					modInfoMisc += fmt.Sprintf(" - %s: <%s>\n", modName, server.Options[modName+util.LinkPostfix])
				}
			}
		}
	}

	if modInfo != "" {
		note.WriteString("\n")
		note.WriteString(modInfo)
		if modInfoMisc != "" {
			note.WriteString(modInfoMisc)
		}
	}

	editDeferredInteractionReply(session, interaction.Interaction, note.String())
}

var ErrUnableToGenerateUniqueName = errors.New("unable to generate unique name")

func generateUniqueName(servers []*nsserver.NSServer, cachedServers []*nsserver.NSServer) (string, error) {
	var unique bool
	name := ""
	for i := 0; i < 5; i++ {
		unique = true
		name = util.CreateFunnyName()
		for _, server := range servers {
			if server.Name == name {
				unique = false
			}
		}

		for _, server := range cachedServers {
			if server.Name == name {
				unique = false
			}
		}
		if unique {
			break
		}
	}

	if !unique {
		return "", ErrUnableToGenerateUniqueName
	}
	return name, nil
}

func (h *handler) handleDeleteServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()
	sendInteractionDeferred(session, interaction)

	server, err := h.nsRepo.GetByName(ctx, serverName)
	if err != nil {
		log.Println(fmt.Sprintf("unable to get server by name: %v", err))

		server = &nsserver.NSServer{
			Name: serverName,
		}
	}

	if h.notifyer != nil {
		logs, err := h.p.ExtractServerLogs(ctx, server)
		if err != nil {
			log.Println(fmt.Sprintf("unable to extract logs for server: %v", err))
		} else {
			go h.notifyer.NotifyAndAttach(fmt.Sprintf("Server %s is due for deletion. Logs:", server.Name), fmt.Sprintf("%s.log.zip", server.Name), logs)
		}
	}

	err = h.p.DeleteServer(ctx, &nsserver.NSServer{
		Name: serverName,
	})
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to delete the target server. error: %v", err))

		return
	}

	err = h.nsRepo.DeleteByName(ctx, serverName)
	if err != nil {
		log.Println(fmt.Sprintf("unable to delete server from the database: %v", err))
	}

	editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("deleted server %s", serverName))
}

func (h *handler) handleRestartServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()

	sendInteractionDeferred(session, interaction)

	server, err := h.nsRepo.GetByName(ctx, serverName)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to get server from cache database. error: %v", err))

		return
	}

	err = h.p.RestartServer(ctx, server)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to restart the target server. error: %v", err))

		return
	}

	editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("restarted server %s", serverName))
}

func (h *handler) handleServerExtendLifetime(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	sendInteractionDeferred(session, interaction)
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()
	extend, err := time.ParseDuration(interaction.ApplicationCommandData().Options[1].StringValue())
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to parse duration. error: %v", err))

		return
	}

	if extend <= 0 {
		editDeferredInteractionReply(session, interaction.Interaction, "duration should not be negative, or 0")

		return
	}

	server, err := h.nsRepo.GetByName(ctx, serverName)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to get server from cache database. error: %v", err))

		return
	}

	if server.ExtendLifetime != nil {
		extend += *server.ExtendLifetime
	}

	if extend > h.maxExtendDuration {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("extended lifetime exceeded maximum allowed extended duration. Extended duration: %s, Max extended duration: %s", extend.String(), h.maxExtendDuration.String()))

		return
	}

	server.ExtendLifetime = &extend
	err = h.nsRepo.Update(ctx, server)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("Failed to update ExtendLifetime field in database, error: %v", err))

		return
	}

	editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("server lifetime successfully updated to: %s", server.ExtendLifetime))
}

func (h *handler) handleServerMetadata(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()

	sendInteractionDeferred(session, interaction)

	server, err := h.nsRepo.GetByName(ctx, serverName)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to get server from cache database. error: %v", err))

		return
	}

	serverMetadata, err := json.Marshal(server)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to marshal server struct. error: %v", err))

		return
	}

	go sendMessageWithFilesDM(session, interaction.Member.User.ID, fmt.Sprintf("Server metadata:\n```%s```", string(serverMetadata)), nil)

	editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("metadata for %s was sent to you privately", serverName))
}

func (h *handler) handleListServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()

	sendInteractionDeferred(session, interaction)

	nsservers, err := h.p.GetRunningServers(ctx)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to list running servers. error: %v", err))

		return
	}

	cachedServers, err := h.nsRepo.GetAll(ctx)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to list running servers from database. error: %v", err))

		return
	}

	var verbose bool
	{
		val, ok := optionValue(interaction.ApplicationCommandData().Options, ListServerVerbosityOpt)
		if ok {
			verbose = val.BoolValue()
		} else {
			verbose = false
		}
	}

	for _, cached := range cachedServers {
		for _, server := range nsservers {
			if server.Name == cached.Name {
				server.Pin = cached.Pin
				server.RequestedBy = cached.RequestedBy
				server.Options = cached.Options
				break
			}
		}
	}

	servers := make([]string, len(nsservers))

	if len(nsservers) == 0 {
		editDeferredInteractionReply(session, interaction.Interaction, "No servers running")

		return
	}
	for idx, server := range nsservers {
		pin := unknown
		if server.Pin != "" {
			pin = server.Pin
		}
		user := unknown
		if server.RequestedBy != "" {
			user = server.RequestedBy
		}
		options := ""
		if server.Options != nil {
			j, err := server.Options.MarshalJSON()
			if err != nil {
				options = fmt.Sprintf("failed to parse servers options. error: %v", err)
			} else {
				options = string(j)
			}
		}
		builder := strings.Builder{}
		builder.WriteString(fmt.Sprintf("Name: %s", server.Name))
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Region: %s", server.Region))
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Pin: `%s`", pin))
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Server Version: %s", server.ServerVersion))
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Requested by: <@%s>", user))
		builder.WriteString("\n")
		if server.MasterServer != DefaultMasterServer {
			builder.WriteString(fmt.Sprintf("Master server: %s", server.MasterServer))
			builder.WriteString("\n")
		}

		if server.Insecure {
			builder.WriteString("Insecure: true")
			builder.WriteString("\n")
			builder.WriteString(fmt.Sprintf("IP: %s, Port: %d", server.MainIP, server.GameUDPPort))
			builder.WriteString("\n")
		}

		if options != "" && verbose {
			builder.WriteString(fmt.Sprintf("Options: \n```\n%s```\n", options))
		}
		if h.autoDeleteDuration > time.Duration(0) {
			autoDelete := h.autoDeleteDuration
			if server.ExtendLifetime != nil {
				autoDelete += *server.ExtendLifetime
			}
			builder.WriteString(fmt.Sprintf("Time until deleted: %s", (autoDelete - time.Since(server.CreatedAt)).String()))
		}
		builder.WriteString("\n\n")
		servers[idx] = builder.String()
	}

	editDeferredInteractionReply(session, interaction.Interaction, strings.Join(servers, "\n"))
}

func (h *handler) handleExtractLogs(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()
	sendInteractionDeferred(session, interaction)
	server, err := h.nsRepo.GetByName(ctx, serverName)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to get server from cache database. error: %v", err))

		return
	}

	file, err := h.p.ExtractServerLogs(ctx, server)
	if err != nil {
		failure := fmt.Sprintf("failed to extract logs from target server. error: %v", err)
		editDeferredInteractionReply(session, interaction.Interaction, failure)
		return
	}

	files := []*discordgo.File{{
		Name:        fmt.Sprintf("%s.log.zip", server.Name),
		ContentType: "application/octet-stream",
		Reader:      file,
	}}

	go sendMessageWithFilesDM(session, interaction.Member.User.ID, fmt.Sprintf("logs extracted from server %s", serverName), files)
	editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("logs extraction for server %s is completed, and are sent privately to you", serverName))
}
