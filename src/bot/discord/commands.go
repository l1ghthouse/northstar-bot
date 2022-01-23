package discord

import (
	"context"
	"fmt"
	"log"
	"strings"

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
	maxConcurrentServers int
}

func (h *handler) handleCreateServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	servers, err := h.p.GetRunningServers(ctx)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("Unable to list running servers: %v", err))

		return
	}
	if len(servers) >= h.maxConcurrentServers {
		sendMessage(session, interaction, fmt.Sprintf("You can't create more than %d servers", h.maxConcurrentServers))

		return
	}

	server, err := h.p.CreateServer(ctx, nsserver.NSServer{
		Name:     "",
		Region:   interaction.ApplicationCommandData().Options[0].StringValue(),
		Password: "",
	})
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to create the target server. error: %v", err))
	} else {
		sendMessage(session, interaction, fmt.Sprintf("created server %s in %s, with password: `%s`. It will take the server around 5 minutes to come online", server.Name, server.Region, server.Password))
	}
}

func (h *handler) handleDeleteServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	permissions := interaction.Member.Permissions
	if permissions&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator {
		sendMessage(session, interaction, "You don't have permission to delete servers")

		return
	}

	serverName := interaction.ApplicationCommandData().Options[0].StringValue()
	ctx := context.Background()
	err := h.p.DeleteServer(ctx, nsserver.NSServer{
		Name:     serverName,
		Region:   "",
		Password: "",
	})
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to delete the target server. error: %v", err))
	} else {
		sendMessage(session, interaction, fmt.Sprintf("deleted server %s", serverName))
	}
}

func (h *handler) handleListServer(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	ctx := context.Background()
	nsservers, err := h.p.GetRunningServers(ctx)
	if err != nil {
		sendMessage(session, interaction, fmt.Sprintf("failed to list running servers. error: %v", err))
		if err != nil {
			log.Println("Error sending message: ", err)
		}

		return
	}

	servers := make([]string, len(nsservers))

	if len(nsservers) == 0 {
		sendMessage(session, interaction, "No servers running")

		return
	}
	for i, server := range nsservers {
		servers[i] = fmt.Sprintf("%s in %s", server.Name, server.Region)
	}

	sendMessage(session, interaction, strings.Join(servers, "\n"))
	if err != nil {
		log.Println("Error sending message: ", err)
	}
}

func sendMessage(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
		},
	}); err != nil {
		log.Println("Error sending message: ", err)
	}
}
