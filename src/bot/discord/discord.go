package discord

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

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

var ISO8601Layout = "2006-01-02T15:04:05-0700"

func (d *discordBot) Start(provider providers.Provider, maxConcurrentServers uint, autoDeleteDuration time.Duration) error {
	discordClient, err := discordgo.New("Bot " + d.config.DcBotToken)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	h := handler{p: provider, maxConcurrentServers: maxConcurrentServers, autoDeleteDuration: autoDeleteDuration}

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

	if autoDeleteDuration != time.Duration(0) {
		providerCloseChannel := make(chan struct{})
		d.closeChannels = append(d.closeChannels, providerCloseChannel)

		go d.autoDelete(provider, autoDeleteDuration, providerCloseChannel, discordClient)
	}

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

func (d *discordBot) gracefulDiscordClose(discordClient io.Closer, callbackDone chan struct{}) {
	defer close(callbackDone)
	<-d.ctx.Done()
	log.Println("Closing Discord connection")
	if err := discordClient.Close(); err != nil {
		log.Println("Error closing Discord session: ", err)
	}
}

// autoDelete deletes servers that are not used for a autoDeleteDuration time
// nolint: gocognit,cyclop
func (d *discordBot) autoDelete(provider providers.Provider, autoDeleteDuration time.Duration, callbackDone chan struct{}, discordClient *discordgo.Session) {
	defer close(callbackDone)
	ticker := time.NewTicker(time.Minute * 120)
	for {
		select {
		case <-d.ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			servers, err := provider.GetRunningServers(d.ctx)
			if err != nil {
				log.Println("error getting running servers: ", err)
				continue
			}
			for _, server := range servers {
				date, err := time.Parse(ISO8601Layout, server.CreatedAt)
				if err != nil {
					log.Println("error parsing date: ", err)
					continue
				}
				if time.Since(date) > autoDeleteDuration {
					err = provider.DeleteServer(d.ctx, server)
					if err != nil {
						log.Println("error deleting server: ", err)
					}

					if d.config.BotReportChannel != "" {
						_, err = discordClient.ChannelMessageSend(d.config.BotReportChannel, fmt.Sprintf("Server '%s' has been deleted because it was up for over %s", server.Name, autoDeleteDuration.String()))
						if err != nil {
							log.Println("error sending message: ", err)
						}
					}
				}
			}
		}
	}
}

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
