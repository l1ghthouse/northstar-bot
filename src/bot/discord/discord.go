package discord

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
	"log"
	"strings"
)

type Config struct {
	DcBotToken string `required:"true"`
}

type Discord struct {
	botToken string
	close    chan struct{}
}

func (d *Discord) Start(provider providers.Provider, maxConcurrentServers int) error {
	dg, err := discordgo.New("Bot " + d.botToken)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	h := handler{p: provider, maxConcurrentServers: maxConcurrentServers}

	dg.AddHandler(h.messageCreate)
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	d.close = make(chan struct{})

	go func() {
		<-d.close
		fmt.Println("Closing Discord connection")
		err := dg.Close()
		if err != nil {
			log.Println("Error closing Discord session: ", err)
		}
		d.close <- struct{}{}
	}()

	return dg.Open()
}

func (d *Discord) Stop() {
	fmt.Println("attempting to close Discord connection")
	d.close <- struct{}{}
	<-d.close
}

func NewDiscordBot(config Config) (*Discord, error) {
	return &Discord{
		botToken: config.DcBotToken,
	}, nil
}

const (
	CreateServer string = "!create_server "
	ListServer   string = "!list_servers "
	DeleteServer string = "!delete_server "
)

type handler struct {
	p                    providers.Provider
	maxConcurrentServers int
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func (h handler) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	ctx := context.Background() //TODO: add timeout
	msg := strings.Split(m.Content, " ")
	cmd := msg[0]
	args := ""
	if len(msg) > 1 {
		args = strings.Join(msg[1:], " ")
	}

	switch {
	case cmd == CreateServer:
		if args == "" {
			sendMessage(s, m, fmt.Sprintf("%s command is missing Region", CreateServer))
			return
		}

		servers, err := h.p.GetRunningServers(ctx)
		if err != nil {
			sendMessage(s, m, fmt.Sprintf("Unable to list running servers: %v", err))
			return
		}
		if len(servers) >= h.maxConcurrentServers {
			sendMessage(s, m, fmt.Sprintf("You can't create more than %d servers", h.maxConcurrentServers))
			return
		}

		server, err := h.p.CreateServer(ctx, nsserver.NSServer{
			Name:     "",
			Region:   args,
			Password: "",
		})
		if err != nil {
			sendMessage(s, m, fmt.Sprintf("failed to create the target server. error: %v", err))
		} else {
			sendMessage(s, m, fmt.Sprintf("created server %s in %s, with password: `%s`. It will take the server around 5 minutes to come online", server.Name, server.Region, server.Password))
		}

	case cmd == DeleteServer:
		if args == "" {
			sendMessage(s, m, fmt.Sprintf("%s command is missing server name", DeleteServer))
			return
		}
		err := h.p.DeleteServer(ctx, nsserver.NSServer{
			Name:     args,
			Region:   "",
			Password: "",
		})
		if err != nil {
			sendMessage(s, m, fmt.Sprintf("failed to delete the target server. error: %v", err))
		} else {
			sendMessage(s, m, fmt.Sprintf("deleted server %s", args))
		}

	case cmd == ListServer:
		nsservers, err := h.p.GetRunningServers(ctx)
		if err != nil {
			sendMessage(s, m, fmt.Sprintf("failed to list running servers. error: %v", err))
			if err != nil {
				log.Println("Error sending message: ", err)
			}
			return
		}

		servers := make([]string, len(nsservers))

		if len(nsservers) == 0 {
			sendMessage(s, m, "No servers running")
			return
		} else {
			for _, server := range nsservers {
				servers = append(servers, fmt.Sprintf("%s in %s", server.Name, server.Region))
			}
		}

		sendMessage(s, m, strings.Join(servers, "\n"))
		if err != nil {
			log.Println("Error sending message: ", err)
		}
	}

}

func sendMessage(s *discordgo.Session, m *discordgo.MessageCreate, msg string) {
	_, err := s.ChannelMessageSend(m.ChannelID, msg)
	if err != nil {
		log.Println("Error sending message: ", err)
	}
}
