package discord

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/l1ghthouse/northstar-bootstrap/src/autodelete"

	"github.com/bwmarrin/discordgo"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
	"github.com/paulbellamy/ratecounter"
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

func (d *discordBot) Start(provider providers.Provider, nsRepo nsserver.Repo, maxConcurrentServers, maxServersPerHour uint, autoDeleteDuration time.Duration, maxExtendDuration time.Duration) (*autodelete.Manager, error) {
	discordClient, err := discordgo.New("Bot " + d.config.DcBotToken)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}
	var counter *ratecounter.RateCounter
	if maxServersPerHour != 0 {
		counter = ratecounter.NewRateCounter(time.Hour)
	}
	var n *Notifyer
	if d.config.BotReportChannel != "" {
		n = &Notifyer{
			discordClient: discordClient,
			reportChannel: d.config.BotReportChannel,
		}
	}

	botHandler := handler{
		p:                    provider,
		maxConcurrentServers: maxConcurrentServers,
		autoDeleteDuration:   autoDeleteDuration,
		nsRepo:               nsRepo,
		maxServerCreateRate:  maxServersPerHour,
		maxExtendDuration:    maxExtendDuration,
		rateCounter:          counter,
		createLock:           &sync.Mutex{},
		notifyer:             n,
	}

	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){}
	commandHandlers[CreateServer] = botHandler.handleCreateServer
	commandHandlers[ListServer] = botHandler.handleListServer
	commandHandlers[DeleteServer] = botHandler.handleDeleteServer
	commandHandlers[ExtractLogs] = botHandler.handleExtractLogs
	commandHandlers[RestartServer] = botHandler.handleRestartServer
	commandHandlers[ServerMetadata] = botHandler.handleServerMetadata
	commandHandlers[ExtendLifetime] = botHandler.handleServerExtendLifetime

	discordClient.AddHandler(func(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
		if handlerFunc, ok := commandHandlers[interaction.ApplicationCommandData().Name]; ok {
			handlerFunc(session, interaction)
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
		if _, ok := commandHandlers[c.Name]; !ok {
			err = discordClient.ApplicationCommandDelete(discordClient.State.User.ID, d.config.DcGuildID, c.ID)
			if err != nil {
				return nil, fmt.Errorf("error deleting command: %w", err)
			}
		}
	}

	for _, newCommand := range commands {
		var oldCommand *discordgo.ApplicationCommand
		for _, c := range cmd {
			if c.Name == newCommand.Name {
				oldCommand = c
			}
		}

		if oldCommand == nil {
			_, err = discordClient.ApplicationCommandCreate(discordClient.State.User.ID, d.config.DcGuildID, newCommand)
			if err != nil {
				return nil, fmt.Errorf("cannot create '%v' command: %w", newCommand.Name, err)
			}
		} else {
			_, err = discordClient.ApplicationCommandEdit(discordClient.State.User.ID, d.config.DcGuildID, oldCommand.ID, newCommand)
			if err != nil {
				return nil, fmt.Errorf("cannot update '%v' command: %w", newCommand.Name, err)
			}
		}
	}

	return autodelete.NewAutoDeleteManager(nsRepo, provider, n, autoDeleteDuration), nil
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
