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
	DcBotToken     string `required:"true"`
	DcClientID     string `required:"true"`
	DcClientSecret string `required:"true"`
	DcOwnerID      string `required:"true"`
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
func (h handler) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	ctx := context.Background() //TODO: add timeout

	switch {
	case strings.HasPrefix(m.Content, CreateServer):

		str := strings.Split(m.Content, " ")
		if len(str) != 2 {
			_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s command takes only one argument. Make sure to specify region to which the server should be deployed", CreateServer))
			if err != nil {
				log.Println("Error sending message: ", err)
			}
			return
		}

		servers, err := h.p.GetRunningServers(ctx)
		if err != nil {
			log.Println("Error getting running servers: ", err)
			return
		}
		if len(servers) >= h.maxConcurrentServers {
			_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You can't create more than %d servers", h.maxConcurrentServers))
			if err != nil {
				log.Println("Error sending message: ", err)
			}
			return
		}

		server, err := h.p.CreateServer(ctx, nsserver.NSServer{
			Name:     "",
			Region:   str[1],
			Password: "",
		})
		if err != nil {
			_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("failed to create the target server. error: %v", err))
			if err != nil {
				log.Println("Error sending message: ", err)
			}
		} else {
			_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("created server %s in %s, with password: `%s`. It will take the server around 8 minutes to come online", server.Name, server.Region, server.Password))
			if err != nil {
				log.Println("Error sending message: ", err)
			}
		}

	case strings.HasPrefix(m.Content, DeleteServer):
		_, err := s.ChannelMessageSend(m.ChannelID, "Deleting Server...")
		//TODO: delete server
		if err != nil {
			log.Println("Error sending message: ", err)
		}

	case strings.HasPrefix(m.Content, ListServer):
		_, err := s.ChannelMessageSend(m.ChannelID, "Listing Servers...")
		//TODO: list server
		if err != nil {
			log.Println("Error sending message: ", err)
		}
	}

}
