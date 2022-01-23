package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

type Config struct {
	DcBotToken string `required:"true"`
	DcGuildID  string `required:"true"`
}

type Discord struct {
	config Config
	close  chan struct{}
}

func (d *Discord) Start(provider providers.Provider, maxConcurrentServers int) error {
	discordClient, err := discordgo.New("Bot " + d.config.DcBotToken)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	h := handler{p: provider, maxConcurrentServers: maxConcurrentServers}

	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){}
	commandHandlers[CreateServer] = h.handleCreateServer
	commandHandlers[ListServer] = h.handleListServer
	commandHandlers[DeleteServer] = h.handleDeleteServer

	discordClient.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})

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

	cmd, err := discordClient.ApplicationCommands(discordClient.State.User.ID, d.config.DcGuildID)
	if err != nil {
		return fmt.Errorf("error getting commands: %w", err)
	}

	for _, c := range cmd {
		err = discordClient.ApplicationCommandDelete(discordClient.State.User.ID, d.config.DcGuildID, c.ID)
		if err != nil {
			return fmt.Errorf("error deleting command: %w", err)
		}
	}

	for _, v := range commands {
		_, err = discordClient.ApplicationCommandCreate(discordClient.State.User.ID, d.config.DcGuildID, v)
		if err != nil {
			return fmt.Errorf("cannot create '%v' command: %w", v.Name, err)
		}
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
		config: config,
	}, nil
}
