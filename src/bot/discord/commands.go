package discord

import (
	"context"
	"fmt"
	"github.com/sethvargo/go-password/password"
	"log"
	"strconv"
	"strings"
	"time"

	"gorm.io/datatypes"

	"github.com/l1ghthouse/northstar-bootstrap/src/providers/util"

	"github.com/bwmarrin/discordgo"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

const (
	CreateServer string = "create_server"
	ListServer   string = "list_servers"
	DeleteServer string = "delete_server"
)

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        CreateServer,
			Description: "Command to create a server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "region",
					Description: "region in which the server will be created",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        nsserver.OptionRebalancedLTSMod,
					Description: "Indicated that the server should include Dinorush's rebalanced mod",
				},
			},
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
}

const PinLength = 5

func (h *handler) handleCreateServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
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
		sendMessage(session, interaction, "Unable to generate a unique name for the server, please try again")

		return
	}

	isRebalanced := false
	if len(interaction.ApplicationCommandData().Options) == 2 {
		isRebalanced = interaction.ApplicationCommandData().Options[1].BoolValue()
	}

	pin, err := strconv.Atoi(password.MustGenerate(PinLength, PinLength, 0, false, true))
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to generate pin: %v", err))
		return
	}

	server := &nsserver.NSServer{
		Region:      interaction.ApplicationCommandData().Options[0].StringValue(),
		RequestedBy: interaction.Member.User.ID,
		Name:        name,
		Pin:         &pin,
		Options: datatypes.JSONMap{
			nsserver.OptionRebalancedLTSMod: isRebalanced,
		},
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

	rebalancedLTSModNotice := ""
	if isRebalanced {
		version := server.Options[util.OptionLTSRebalancedVersion].(string)
		rebalancedLTSModNotice = fmt.Sprintf("\nNOTE: This server includes the rebalanced LTS mod version: **%s**.\nEnsure you have the latest version of the mod installed.", version)
	}

	sendMessage(session, interaction, fmt.Sprintf("created server %s in %s, with password: `%d`. \nIt will take the server around 5 minutes to come online", server.Name, server.Region, *server.Pin)+autodeleteMessage+rebalancedLTSModNotice)
}

func (h *handler) handleDeleteServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	serverName := interaction.ApplicationCommandData().Options[0].StringValue()
	permissions := interaction.Member.Permissions
	if permissions&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator {
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
		pin := "unknown"
		if server.Pin != nil {
			pin = strconv.Itoa(*server.Pin)
		}
		user := "unknown"
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
