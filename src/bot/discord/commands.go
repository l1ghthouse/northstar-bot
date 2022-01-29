package discord

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

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

	server := &nsserver.NSServer{
		Region:      interaction.ApplicationCommandData().Options[0].StringValue(),
		RequestedBy: interaction.Member.User.ID,
		Name:        name,
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

	sendMessage(session, interaction, fmt.Sprintf("created server %s in %s, with password: `%d`. \nIt will take the server around 5 minutes to come online", server.Name, server.Region, *server.Pin)+autodeleteMessage)
}

func (h *handler) handleDeleteServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	permissions := interaction.Member.Permissions
	if permissions&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator {
		sendMessage(session, interaction, "You don't have permission to delete servers")

		return
	}

	serverName := interaction.ApplicationCommandData().Options[0].StringValue()
	ctx := context.Background()
	err := h.p.DeleteServer(ctx, &nsserver.NSServer{
		Name: serverName,
	})
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to delete the target server. error: %v", err))

		return
	}

	err = h.nsRepo.DeleteByName(ctx, serverName)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("Unable to delete server from the database: %v", err))

		return
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
		untilDeleted := ""
		if h.autoDeleteDuration > time.Duration(0) {
			untilDeleted = fmt.Sprintf(". Time until deleted: %s", (h.autoDeleteDuration - time.Since(server.CreatedAt)).String())
		}
		pin := "unknown"
		if server.Pin != nil {
			pin = strconv.Itoa(*server.Pin)
		}
		user := "unknown"
		if server.RequestedBy != "" {
			user = server.RequestedBy
		}

		servers[idx] = fmt.Sprintf("%s in %s. PIN: `%s`. Requested by <@%s>", server.Name, server.Region, pin, user) + untilDeleted
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
