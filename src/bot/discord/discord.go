package discord

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/l1ghthouse/northstar-bootstrap/src/bot/notifyer"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"

	"github.com/bwmarrin/discordgo"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

type Config struct {
	DcBotToken       string `required:"true"`
	DcGuildID        string `required:"true"`
	BotReportChannel string ``
}

type discordBot struct {
	config        Config
	ctx           context.Context
	close         context.CancelFunc
	closeChannels []chan struct{}
}

func (d *discordBot) Start(provider providers.Provider, nsRepo nsserver.Repo, maxConcurrentServers uint, autoDeleteDuration time.Duration) (notifyer.Notifyer, error) {
	discordClient, err := discordgo.New("Bot " + d.config.DcBotToken)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	h := handler{p: provider, maxConcurrentServers: maxConcurrentServers, autoDeleteDuration: autoDeleteDuration, nsRepo: nsRepo}

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

	discordCloseConfirmation := make(chan struct{})
	d.closeChannels = append(d.closeChannels, discordCloseConfirmation)

	go d.gracefulDiscordClose(discordClient, discordCloseConfirmation)

	err = discordClient.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening Discord connection: %w", err)
	}

	cmd, err := discordClient.ApplicationCommands(discordClient.State.User.ID, d.config.DcGuildID)
	if err != nil {
		return nil, fmt.Errorf("error getting commands: %w", err)
	}

	for _, c := range cmd {
		err = discordClient.ApplicationCommandDelete(discordClient.State.User.ID, d.config.DcGuildID, c.ID)
		if err != nil {
			return nil, fmt.Errorf("error deleting command: %w", err)
		}
	}

	for _, v := range commands {
		_, err = discordClient.ApplicationCommandCreate(discordClient.State.User.ID, d.config.DcGuildID, v)
		if err != nil {
			return nil, fmt.Errorf("cannot create '%v' command: %w", v.Name, err)
		}
	}

	return &Notifyer{
		discordClient: discordClient,
		config:        d.config,
	}, nil
}

func (d *discordBot) gracefulDiscordClose(discordClient io.Closer, callbackDone chan struct{}) {
	defer close(callbackDone)
	<-d.ctx.Done()
	log.Println("Closing Discord connection")
	if err := discordClient.Close(); err != nil {
		log.Println("Error closing Discord session: ", err)
	}
}

// autoDelete deletes servers that are not used for a autoDeleteDuration time

func (d *discordBot) Stop() {
	log.Println("attempting to close Discord connection")
	d.close()
	for _, c := range d.closeChannels {
		<-c
	}
}

// NewDiscordBot creates a new Discord bot, with cancellable context
// nolint: revive,golint
func NewDiscordBot(config Config) (*discordBot, error) {
	ctx, cf := context.WithCancel(context.Background())
	return &discordBot{
		config:        config,
		ctx:           ctx,
		close:         cf,
		closeChannels: []chan struct{}{},
	}, nil
}
