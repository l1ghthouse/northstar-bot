package discord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
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
	CreateServer  = "create_server"
	ListServer    = "list_servers"
	DeleteServer  = "delete_server"
	ExtractLogs   = "extract_logs"
	RestartServer = "restart_server"
)

func modApplicationCommand() (options []*discordgo.ApplicationCommandOption) {
	for k := range mod.ModByName {
		options = append(options, &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        k,
			Description: fmt.Sprintf("Indicated that the server should include %s mod", k),
		})
	}
	return
}

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
		},
	}
)

type handler struct {
	p                    providers.Provider
	maxConcurrentServers uint
	autoDeleteDuration   time.Duration
	nsRepo               nsserver.Repo
	maxServerCreateRate  uint
	rateCounter          *ratecounter.RateCounter
}

const unknown = "unknown"
const PinLength = 4

func (h *handler) handleCreateServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()

	if h.maxServerCreateRate != 0 && h.rateCounter.Rate() > int64(h.maxServerCreateRate) {
		sendMessage(session, interaction, "You have exceeded the maximum number of servers you can create per hour. Please try again later.")
	}
	servers, err := h.p.GetRunningServers(ctx)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("Unable to list running servers: %v", err))

		return
	}
	if len(servers) >= int(h.maxConcurrentServers) {
		sendMessage(session, interaction, fmt.Sprintf("You can't create more than %d servers", h.maxConcurrentServers))

		return
	}
	cachedServers, err := h.nsRepo.GetAll(ctx)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("Unable to list servers: %v", err))

		return
	}
	name, err := generateUniqueName(servers, cachedServers)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("Unable to generate unique server name: %v", err))

		return
	}
	var modOptions = make(map[string]interface{})
	for modName := range mod.ModByName {
		modOptions[modName] = false
		for _, option := range interaction.ApplicationCommandData().Options {
			if option.Name == modName {
				modOptions[modName] = true
				modOptions[modName] = option.BoolValue()
				break
			}
		}
	}

	pin := password.MustGenerate(PinLength, PinLength, 0, false, true)

	server := &nsserver.NSServer{
		Region:      interaction.ApplicationCommandData().Options[0].StringValue(),
		RequestedBy: interaction.Member.User.ID,
		Name:        name,
		Pin:         pin,
		Options:     modOptions,
	}
	err = h.p.CreateServer(ctx, server)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to create the target server. error: %v", err))
		return
	}

	err = h.nsRepo.Store(ctx, []*nsserver.NSServer{server})
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("Unable to save server to the database: %v", err))

		return
	}

	autodeleteMessage := ""
	if h.autoDeleteDuration != time.Duration(0) {
		autodeleteMessage = fmt.Sprintf("\nThis server will be deleted in %s", h.autoDeleteDuration.String())
	}

	modInfo := ""

	for option := range server.Options {
		for modName := range mod.ModByName {
			if option == modName && server.Options[option].(bool) {
				modInfo += fmt.Sprintf("%s: version: **%s**. Download link: <%s>\n", modName, server.Options[modName+util.VersionPostfix], server.Options[modName+util.LinkPostfix])
			}
		}
	}

	modNotice := ""
	if modInfo != "" {
		modNotice = "\nNOTE: This server includes the following mods:\n" + modInfo
	}

	sendMessage(session, interaction, fmt.Sprintf("created server **%s** in **%s**, with password: **%s**. \nIt will take the server around 5 minutes to come online", server.Name, server.Region, server.Pin)+autodeleteMessage+modNotice)

	if h.maxServerCreateRate != 0 {
		h.rateCounter.Incr(1)
	}
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

func isAdministrator(permissions int64) bool {
	return permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator
}

func (h *handler) handleDeleteServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()
	if !isAdministrator(interaction.Member.Permissions) {
		cachedServer, err := h.nsRepo.GetByName(ctx, serverName)
		if err != nil || cachedServer.RequestedBy != interaction.Member.User.ID {
			sendMessage(session, interaction, "Only Administrators and the person who requested the server can delete it")

			return
		}
	}

	err := h.p.DeleteServer(ctx, &nsserver.NSServer{
		Name: serverName,
	})
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to delete the target server. error: %v", err))

		return
	}

	err = h.nsRepo.DeleteByName(ctx, serverName)
	if err != nil {
		log.Println(fmt.Sprintf("unable to delete server from the database: %v", err))
	}

	sendMessage(session, interaction, fmt.Sprintf("deleted server %s", serverName))
}

func (h *handler) handleRestartServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()
	if !isAdministrator(interaction.Member.Permissions) {
		cachedServer, err := h.nsRepo.GetByName(ctx, serverName)
		if err != nil || cachedServer.RequestedBy != interaction.Member.User.ID {
			sendMessage(session, interaction, "Only Administrators and the person who requested the server can restart it")

			return
		}
	}

	server, err := h.nsRepo.GetByName(ctx, serverName)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to get server from cache database. error: %v", err))

		return
	}

	err = h.p.RestartServer(ctx, server)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to restart the target server. error: %v", err))

		return
	}

	sendMessage(session, interaction, fmt.Sprintf("restarted server %s", serverName))
}

func (h *handler) handleListServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	nsservers, err := h.p.GetRunningServers(ctx)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to list running servers. error: %v", err))

		return
	}

	cachedServers, err := h.nsRepo.GetAll(ctx)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to list running servers from database. error: %v", err))
		return
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
		sendMessage(session, interaction, "No servers running")

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
			json, err := server.Options.MarshalJSON()
			if err != nil {
				options = fmt.Sprintf("failed to parse servers options. error: %v", err)
			} else {
				options = string(json)
			}
		}
		builder := strings.Builder{}
		builder.WriteString(fmt.Sprintf("Name: %s", server.Name))
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Region: %s", server.Region))
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Pin: `%s`", pin))
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Requested by: <@%s>", user))
		builder.WriteString("\n")
		if options != "" {
			builder.WriteString(fmt.Sprintf("Options: \n```\n%s```\n", options))
		}
		if h.autoDeleteDuration > time.Duration(0) {
			builder.WriteString(fmt.Sprintf("Time until deleted: %s", (h.autoDeleteDuration - time.Since(server.CreatedAt)).String()))
		}
		builder.WriteString("\n\n")
		servers[idx] = builder.String()
	}

	sendMessage(session, interaction, strings.Join(servers, "\n"))
}

func (h *handler) handleExtractLogs(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()

	server, err := h.nsRepo.GetByName(ctx, serverName)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to get server from cache database. error: %v", err))

		return
	}

	sendMessage(session, interaction, fmt.Sprintf("Extracting logs for %s. They will be sent to you privately once they are ready", serverName))

	file, err := h.p.ExtractServerLogs(ctx, server)
	if err != nil {
		sendMessageWithFilesDM(session, interaction, fmt.Sprintf("failed to extract logs from target server. error: %v", err), nil)

		return
	}

	files := []*discordgo.File{{
		Name:        fmt.Sprintf("%s.log.zip", server.Name),
		ContentType: "application/octet-stream",
		Reader:      file,
	}}

	sendMessageWithFilesDM(session, interaction, fmt.Sprintf("logs extracted from server %s", serverName), files)
}

func sendMessageWithFilesDM(session *discordgo.Session, interaction *discordgo.InteractionCreate, msg string, file []*discordgo.File) {
	directMessageChannel, err := session.UserChannelCreate(interaction.Member.User.ID)
	if err != nil {
		log.Println(fmt.Sprintf("failed to create DM channel for user %s. error: %v", interaction.Member.User.ID, err))

		return
	}
	_, err = session.ChannelMessageSendComplex(directMessageChannel.ID, &discordgo.MessageSend{
		Content: msg,
		Files:   file,
	})

	if err != nil {
		log.Println(fmt.Sprintf("failed to send message to user %s. error: %v", interaction.Member.User.ID, err))

		return
	}
}

func sendMessage(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Users: []string{},
			},
		},
	}); err != nil {
		log.Println("Error sending message: ", err)
	}
}
