package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
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
	CreateServer         = "create_server"
	ListServer           = "list_servers"
	DeleteServer         = "delete_server"
	ExtractLogs          = "extract_logs"
	RestartServer        = "restart_server"
	ServerMetadata       = "server_metadata"
	CommandFlagOverrides = "list_command_flag_overrides"
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
const CreateServerOptBareMetal = "bare_metal"
const CreateServerOptCheatsEnabled = "cheats_enabled"
const CreateServerVersionOpt = "server_version"
const CreateServerCustomDockerContainerOpt = "custom_container"
const CreateServerCustomThunderstoreMods = "custom_thunderstore_mods"
const CreateServerTickRate = "tick_rate"
const ListServerVerbosityOpt = "verbosity"
const AdditionalExtraArgs = "additional_extra_args"
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
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        CreateServerCustomThunderstoreMods,
					Description: "Comma separated list of custom thunderstore mods for server to install",
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        CreateServerTickRate,
					Description: "Custom TickRate to use for the server",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        CreateServerOptBareMetal,
					Description: "Whether the server should be created on bare metal.",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        CreateServerOptCheatsEnabled,
					Description: "Whether the server should be created with cheats enabled.",
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        AdditionalExtraArgs,
					Description: "Additional extra args to pass to the server",
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
			Name:        CommandFlagOverrides,
			Description: "Command flag overrides for current discord server",
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
			Description: "Extends lifetime of the server by a given amount. Ex: 1h, 30m, 1h30m50s",
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
	CommandOverrides     []CommandOverrides
	notifier             *Notifier
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

func (h *handler) extractCustomMods(interaction *discordgo.InteractionCreate) string {
	thunderstoreMods := ""
	val, ok := optionValue(interaction.ApplicationCommandData().Options, CreateServerCustomThunderstoreMods)
	if ok {
		thunderstoreMods = val.StringValue()
	} else {
		val, ok := h.getGlobalOverrideStringValue(interaction.ApplicationCommandData().Name, CreateServerCustomThunderstoreMods)
		if ok {
			thunderstoreMods = val
		}
	}

	return thunderstoreMods
}

func (h *handler) getGlobalOverrideBoolValue(command, flag string) (bool, bool) {
	for _, commandOverride := range h.CommandOverrides {
		if command == commandOverride.Command && flag == commandOverride.Flag {
			value, err := strconv.ParseBool(commandOverride.Value)
			if err != nil {
				log.Println(fmt.Sprintf("Unable to interpret global command overwrite as bool: %v", commandOverride.Value))
			}
			return value, true
		}
	}
	return false, false
}

func (h *handler) getGlobalOverrideStringValue(command, flag string) (string, bool) {
	for _, commandOverride := range h.CommandOverrides {
		if command == commandOverride.Command && flag == commandOverride.Flag {
			return commandOverride.Value, true
		}
	}
	return "", false
}

func (h *handler) defaultServer(name string, interaction *discordgo.InteractionCreate) (*nsserver.NSServer, error) {
	var modOptions = make(map[string]interface{})
	{
		for modName := range mod.ByName {
			modOptions[modName] = mod.ByName[modName]().EnabledByDefault()
			val, ok := optionValue(interaction.ApplicationCommandData().Options, modName)
			if ok {
				modOptions[modName] = val.BoolValue()
			} else {
				val, ok := h.getGlobalOverrideBoolValue(interaction.ApplicationCommandData().Name, modName)
				if ok {
					modOptions[modName] = val
				}
			}
		}
		thunderstoreMods := h.extractCustomMods(interaction)

		for _, m := range strings.Split(thunderstoreMods, ",") {
			modName := strings.TrimSpace(m)
			if modName != "" {
				modOptions[modName] = true
			}
		}
	}

	var tickRate uint64
	{
		val, ok := optionValue(interaction.ApplicationCommandData().Options, CreateServerTickRate)
		if ok {
			tickRate = val.UintValue()
		}
	}

	var extraArgs string
	{
		val, ok := optionValue(interaction.ApplicationCommandData().Options, AdditionalExtraArgs)
		if ok {
			extraArgs = val.StringValue()
		} else {
			val, ok := h.getGlobalOverrideStringValue(interaction.ApplicationCommandData().Name, AdditionalExtraArgs)
			if ok {
				extraArgs = val
			}
		}
	}

	if tickRate != 0 && (tickRate < 20 || tickRate > 120) {
		return nil, fmt.Errorf("tick_rate must be between 20, and 120. Following value is not supported: %d", tickRate)
	}

	var isInsecure bool
	{
		val, ok := optionValue(interaction.ApplicationCommandData().Options, CreateServerOptInsecure)
		if ok {
			isInsecure = val.BoolValue()
		} else {
			val, ok := h.getGlobalOverrideBoolValue(interaction.ApplicationCommandData().Name, CreateServerOptInsecure)
			if ok {
				isInsecure = val
			} else {
				isInsecure = false
			}
		}
	}

	var isBareMetal bool
	{
		val, ok := optionValue(interaction.ApplicationCommandData().Options, CreateServerOptBareMetal)
		if ok {
			isBareMetal = val.BoolValue()
		} else {
			val, ok := h.getGlobalOverrideBoolValue(interaction.ApplicationCommandData().Name, CreateServerOptBareMetal)
			if ok {
				isBareMetal = val
			} else {
				isBareMetal = false
			}
		}
	}

	var masterServer string
	{
		val, ok := optionValue(interaction.ApplicationCommandData().Options, CreateServerOptMasterServer)
		if ok {
			masterServer = val.StringValue()
		} else {
			val, ok := h.getGlobalOverrideStringValue(interaction.ApplicationCommandData().Name, CreateServerOptMasterServer)
			if ok {
				masterServer = val
			} else {
				masterServer = DefaultMasterServer
			}
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

	var cheatsEnabled bool
	{
		val, ok := optionValue(interaction.ApplicationCommandData().Options, CreateServerOptCheatsEnabled)
		if ok {
			cheatsEnabled = val.BoolValue()
		} else {
			val, ok := h.getGlobalOverrideBoolValue(interaction.ApplicationCommandData().Name, CreateServerOptCheatsEnabled)
			if ok {
				cheatsEnabled = val
			} else {
				cheatsEnabled = false
			}
		}
	}

	return &nsserver.NSServer{
		Region:             interaction.ApplicationCommandData().Options[0].StringValue(),
		RequestedBy:        interaction.Member.User.ID,
		Name:               name,
		Pin:                pin,
		ModOptions:         modOptions,
		Insecure:           isInsecure,
		BareMetal:          isBareMetal,
		ServerVersion:      serverVersion,
		GameUDPPort:        37015,
		AuthTCPPort:        8081,
		TickRate:           tickRate,
		DockerImageVersion: dockerImageVersion,
		MasterServer:       masterServer,
		EnableCheats:       cheatsEnabled,
		ExtraArgs:          extraArgs,
	}, nil
}

const DefaultMasterServer = "https://northstar.tf"

func (h *handler) handleCreateServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()

	sendInteractionDeferred(session, interaction)

	h.createLock.Lock()
	defer h.createLock.Unlock()
	if h.maxServerCreateRate != 0 && h.rateCounter.Rate() > int64(h.maxServerCreateRate) {
		editDeferredInteractionReply(session, interaction.Interaction, "You have exceeded the maximum number of servers you can create per hour. Please try again later.", nil)

		return
	}
	servers, err := h.p.GetRunningServers(ctx)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("unable to list running servers: %v", err), nil)

		return
	}
	if len(servers) >= int(h.maxConcurrentServers) {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("You can't create more than %d servers", h.maxConcurrentServers), nil)

		return
	}
	cachedServers, err := h.nsRepo.GetAll(ctx)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("unable to list servers: %v", err), nil)

		return
	}

	name, err := generateUniqueName(servers, cachedServers)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("unable to generate unique server name: %v", err), nil)

		return
	}

	server, err := h.defaultServer(name, interaction)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("unable to create server: %v", err), nil)

		return
	}

	err = h.p.CreateServer(ctx, server)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to create the target server. error: %v", err), nil)

		return
	}

	if h.maxServerCreateRate != 0 {
		h.rateCounter.Incr(1)
	}

	err = h.nsRepo.Store(ctx, []*nsserver.NSServer{server})
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("unable to save server to the database: %v", err), nil)

		return
	}

	note := strings.Builder{}
	note.WriteString(fmt.Sprintf("Created server **%s** in **%s**, with password: **%s**.", server.Name, server.Region, server.Pin))
	note.WriteString("\n")

	if server.Insecure {
		note.WriteString(fmt.Sprintf("Insecure mode is enabled. If master server is offline, use: `connect %s:%d`", server.MainIP, server.GameUDPPort))
		note.WriteString("\n")
	}
	timeToSpinUp := 5 // Now that we have to deal with broken vultr servers
	if server.BareMetal {
		timeToSpinUp = 20
		note.WriteString(fmt.Sprintf("**This is a bare metal server. It will take longer to spin up, but will be more performant. Ideally, you should only use this if you are hosting a tournament.**"))
		note.WriteString("\n")
	}

	note.WriteString(fmt.Sprintf("Server version: **%s**", server.ServerVersion))
	note.WriteString(fmt.Sprintf(". Server will be up in: **%d** minutes(This could be affected by slowness in vultr regions, or github API)", timeToSpinUp))
	if h.autoDeleteDuration != time.Duration(0) {
		note.WriteString(fmt.Sprintf(", and autodeleted at <t:%d:R>", time.Now().Add(h.autoDeleteDuration).UnixNano()/1e9))
	}
	note.WriteString("\n")

	modInfo := ""
	modInfoMisc := ""

	for modName := range server.ModOptions {
		_, knownMod := mod.ByName[modName]
		thunderstoreMods := h.extractCustomMods(interaction)
		if knownMod || strings.Contains(thunderstoreMods, modName) {
			enabled, ok := server.ModOptions[modName].(bool)
			if ok && enabled {
				if modInfo == "" {
					modInfo = "Following Mods Are Enabled:\n"
				}
				modInfo += fmt.Sprintf(" - %s(version: %s)\n", modName, server.ModOptions[modName+util.VersionPostfix])

				if server.ModOptions[modName+util.RequiredByClientPostfix] == true {
					if modInfoMisc == "" {
						modInfoMisc = "Following mods are **REQUIRED TO BE DOWNLOADED BY CLIENT**:\n"
					}
					modInfoMisc += fmt.Sprintf(" - %s: <%s>\n", modName, server.ModOptions[modName+util.LinkPostfix])
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

	if server.EnableCheats {
		note.WriteString("\n")
		note.WriteString("**Cheats are enabled.**")
		note.WriteString("\n")
	}

	if server.TickRate != 0 {
		note.WriteString("\n")
		note.WriteString("Custom tick_rate value supplied.\n**Make sure clients have the following console variables:**")
		note.WriteString("\n")
		note.WriteString(fmt.Sprintf("cl_updaterate_mp %d", server.TickRate))
	}

	editDeferredInteractionReply(session, interaction.Interaction, note.String(), nil)
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

func (h *handler) handleCommandFlagOverrides(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	sendInteractionDeferred(session, interaction)
	b, err := json.Marshal(h.CommandOverrides)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("unable to marshal command flag overrides: %v", err), nil)
		return
	}
	editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("```%s```", string(b)), nil)
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

	if h.notifier != nil {
		logs, err := h.p.ExtractServerLogs(ctx, server)
		if err != nil {
			log.Println(fmt.Sprintf("unable to extract logs for server: %v", err))
			h.notifier.NotifyServer(server, fmt.Sprintf("unable to extract logs for server: %v", err))
		} else {
			go h.notifier.NotifyAndAttachServerData(server, "Deleted, logs:", fmt.Sprintf("%s.log.zip", server.Name), logs)
		}
	}

	err = h.p.DeleteServer(ctx, &nsserver.NSServer{
		Name: serverName,
	})
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to delete the target server. error: %v", err), nil)

		return
	}

	err = h.nsRepo.DeleteByName(ctx, serverName)
	if err != nil {
		log.Println(fmt.Sprintf("unable to delete server from the database: %v", err))
	}

	editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("deleted server %s", serverName), nil)
}

func (h *handler) handleRestartServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()

	sendInteractionDeferred(session, interaction)

	server, err := h.nsRepo.GetByName(ctx, serverName)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to get server from cache database. error: %v", err), nil)

		return
	}

	err = h.p.RestartServer(ctx, server)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to restart the target server. error: %v", err), nil)

		return
	}

	editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("restarted server %s", serverName), nil)
}

func (h *handler) handleServerExtendLifetime(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	sendInteractionDeferred(session, interaction)
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()
	extend, err := time.ParseDuration(interaction.ApplicationCommandData().Options[1].StringValue())
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to parse duration. error: %v", err), nil)

		return
	}

	if extend <= 0 {
		editDeferredInteractionReply(session, interaction.Interaction, "duration should not be negative, or 0", nil)

		return
	}

	server, err := h.nsRepo.GetByName(ctx, serverName)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to get server from cache database. error: %v", err), nil)

		return
	}

	if server.ExtendLifetime != nil {
		extend += *server.ExtendLifetime
	}

	if extend > h.maxExtendDuration {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("extended lifetime exceeded maximum allowed extended duration. Extended duration: %s, Max extended duration: %s", extend.String(), h.maxExtendDuration.String()), nil)

		return
	}

	server.ExtendLifetime = &extend
	err = h.nsRepo.Update(ctx, server)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("Failed to update ExtendLifetime field in database, error: %v", err), nil)

		return
	}

	editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("server lifetime successfully updated to: %s", server.ExtendLifetime), nil)
}

func (h *handler) handleServerMetadata(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()

	sendInteractionDeferred(session, interaction)

	server, err := h.nsRepo.GetByName(ctx, serverName)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to get server from cache database. error: %v", err), nil)

		return
	}

	serverMetadata, err := json.Marshal(server)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to marshal server struct. error: %v", err), nil)

		return
	}

	files := []*discordgo.File{{
		Name:        fmt.Sprintf("%s.metadata.json", server.Name),
		ContentType: "application/octet-stream",
		Reader:      strings.NewReader(string(serverMetadata)),
	}}

	go sendMessageWithFilesDM(session, interaction.Member.User.ID, "server metadata:", files)

	editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("metadata for %s was sent to you privately", serverName), nil)
}

func (h *handler) handleListServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()

	sendInteractionDeferred(session, interaction)

	nsservers, err := h.p.GetRunningServers(ctx)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to list running servers. error: %v", err), nil)

		return
	}

	cachedServers, err := h.nsRepo.GetAll(ctx)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to list running servers from database. error: %v", err), nil)

		return
	}

	var verbose bool
	{
		val, ok := optionValue(interaction.ApplicationCommandData().Options, ListServerVerbosityOpt)
		if ok {
			verbose = val.BoolValue()
		} else {
			val, ok := h.getGlobalOverrideBoolValue(interaction.ApplicationCommandData().Name, ListServerVerbosityOpt)
			if ok {
				verbose = val
			} else {
				verbose = false
			}
		}
	}

	for _, cached := range cachedServers {
		for _, server := range nsservers {
			if server.Name == cached.Name {
				server.Pin = cached.Pin
				server.RequestedBy = cached.RequestedBy
				server.ModOptions = cached.ModOptions
				server.ExtendLifetime = cached.ExtendLifetime
				break
			}
		}
	}

	servers := make([]string, len(nsservers))

	if len(nsservers) == 0 {
		editDeferredInteractionReply(session, interaction.Interaction, "No servers running", nil)

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
		if server.ModOptions != nil {
			j, err := server.ModOptions.MarshalJSON()
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

	message := strings.Join(servers, "\n")
	var files []*discordgo.File
	if len(message) > 1900 {
		files = []*discordgo.File{{
			Name:        "list_server.txt",
			ContentType: "application/octet-stream",
			Reader:      strings.NewReader(message),
		}}

		message = "List of servers is too long to be sent in a message. Sending as a file instead."
	}

	editDeferredInteractionReply(session, interaction.Interaction, message, files)
}

func (h *handler) handleExtractLogs(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()
	sendInteractionDeferred(session, interaction)
	server, err := h.nsRepo.GetByName(ctx, serverName)
	if err != nil {
		editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("failed to get server from cache database. error: %v", err), nil)

		return
	}

	file, err := h.p.ExtractServerLogs(ctx, server)
	if err != nil {
		failure := fmt.Sprintf("failed to extract logs from target server. error: %v", err)
		editDeferredInteractionReply(session, interaction.Interaction, failure, nil)
		return
	}

	files := []*discordgo.File{{
		Name:        fmt.Sprintf("%s.log.zip", server.Name),
		ContentType: "application/octet-stream",
		Reader:      file,
	}}

	go sendMessageWithFilesDM(session, interaction.Member.User.ID, fmt.Sprintf("logs extracted from server %s", serverName), files)
	editDeferredInteractionReply(session, interaction.Interaction, fmt.Sprintf("logs extraction for server %s is completed, and are sent privately to you", serverName), nil)
}
