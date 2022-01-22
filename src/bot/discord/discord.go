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

type Config struct {
	DcBotToken string `required:"true"`
}

type Discord struct {
	botToken string
	close    chan struct{}
}

func (d *Discord) Start(provider providers.Provider, maxConcurrentServers int) error {
	discordClient, err := discordgo.New("Bot " + d.botToken)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	h := handler{p: provider, maxConcurrentServers: maxConcurrentServers}

	discordClient.AddHandler(h.messageCreate)
	discordClient.Identify.Intents = discordgo.IntentsGuildMessages

	d.close = make(chan struct{})

	go func() {
		<-d.close
		log.Println("Closing Discord connection")
		err := discordClient.Close()
		if err != nil {
			log.Println("Error closing Discord session: ", err)
		}
		d.close <- struct{}{}
	}()

	err = discordClient.Open()
	if err != nil {
		return fmt.Errorf("error opening Discord connection: %w", err)
	}
	return nil
}

func (d *Discord) Stop() {
	log.Println("attempting to close Discord connection")
	d.close <- struct{}{}
	<-d.close
}

func NewDiscordBot(config Config) (*Discord, error) {
	return &Discord{
		botToken: config.DcBotToken,
	}, nil
}

const (
	CreateServer string = "!create_server"
	ListServer   string = "!list_servers"
	DeleteServer string = "!delete_server"
)

type handler struct {
	p                    providers.Provider
	maxConcurrentServers int
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func (h *handler) messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if message.Author.ID == session.State.User.ID {
		return
	}

	ctx := context.Background() // TODO: add timeout
	msg := strings.Split(message.Content, " ")
	cmd := msg[0]
	args := ""
	if len(msg) > 1 {
		args = strings.Join(msg[1:], " ")
	}

	h.handleMessage(ctx, session, message, cmd, args)
}

func (h *handler) handleMessage(ctx context.Context, session *discordgo.Session, message *discordgo.MessageCreate, cmd string, args string) {
	permissions, err := session.State.MessagePermissions(message.Message)
	if err != nil {
		sendMessage(session, message, fmt.Sprintf("Error getting permissions: %v", err.Error()))

		return
	}

	switch {
	case cmd == CreateServer:
		h.handleCreateServer(ctx, session, message, args)

	case cmd == DeleteServer:
		if permissions&discordgo.PermissionAdministrator == 0 {
			sendMessage(session, message, "You don't have permission to delete servers")
			return
		}
		h.handleDeleteServer(ctx, session, message, args)

	case cmd == ListServer:
		h.handleListServer(ctx, session, message)
	}
}

func (h *handler) handleCreateServer(ctx context.Context, session *discordgo.Session, message *discordgo.MessageCreate, args string) {
	if args == "" {
		sendMessage(session, message, fmt.Sprintf("%s command is missing Region", CreateServer))

		return
	}

	servers, err := h.p.GetRunningServers(ctx)
	if err != nil {
		sendMessage(session, message, fmt.Sprintf("Unable to list running servers: %v", err))

		return
	}
	if len(servers) >= h.maxConcurrentServers {
		sendMessage(session, message, fmt.Sprintf("You can't create more than %d servers", h.maxConcurrentServers))

		return
	}

	server, err := h.p.CreateServer(ctx, nsserver.NSServer{
		Name:     "",
		Region:   args,
		Password: "",
	})
	if err != nil {
		sendMessage(session, message, fmt.Sprintf("failed to create the target server. error: %v", err))
	} else {
		sendMessage(session, message, fmt.Sprintf("created server %s in %s, with password: `%s`. It will take the server around 5 minutes to come online", server.Name, server.Region, server.Password))
	}
}

func (h *handler) handleDeleteServer(ctx context.Context, session *discordgo.Session, message *discordgo.MessageCreate, args string) {
	if args == "" {
		sendMessage(session, message, fmt.Sprintf("%s command is missing server name", DeleteServer))

		return
	}
	err := h.p.DeleteServer(ctx, nsserver.NSServer{
		Name:     args,
		Region:   "",
		Password: "",
	})
	if err != nil {
		sendMessage(session, message, fmt.Sprintf("failed to delete the target server. error: %v", err))
	} else {
		sendMessage(session, message, fmt.Sprintf("deleted server %s", args))
	}
}

func (h *handler) handleListServer(ctx context.Context, session *discordgo.Session, message *discordgo.MessageCreate) {
	nsservers, err := h.p.GetRunningServers(ctx)
	if err != nil {
		sendMessage(session, message, fmt.Sprintf("failed to list running servers. error: %v", err))
		if err != nil {
			log.Println("Error sending message: ", err)
		}

		return
	}

	servers := make([]string, len(nsservers))

	if len(nsservers) == 0 {
		sendMessage(session, message, "No servers running")

		return
	}
	for i, server := range nsservers {
		servers[i] = fmt.Sprintf("%s in %s", server.Name, server.Region)
	}

	sendMessage(session, message, strings.Join(servers, "\n"))
	if err != nil {
		log.Println("Error sending message: ", err)
	}
}

func sendMessage(s *discordgo.Session, m *discordgo.MessageCreate, msg string) {
	_, err := s.ChannelMessageSend(m.ChannelID, msg)
	if err != nil {
		log.Println("Error sending message: ", err)
	}
}
