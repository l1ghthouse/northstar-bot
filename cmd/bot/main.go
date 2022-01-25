package main

import (
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jinzhu/configor"
	"github.com/l1ghthouse/northstar-bootstrap/src/bot"
	"github.com/l1ghthouse/northstar-bootstrap/src/config"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

func main() {
	cfg := config.Config{}
	err := configor.Load(&cfg, "config.yml")
	if err != nil {
		log.Fatal("Failed to load config: ", err)
	}

	rand.Seed(time.Now().UnixNano())

	newBot, err := bot.NewBot(cfg.Bot)
	if err != nil {
		log.Fatal("Failed to create bot: ", err)
	}

	provider, err := providers.NewProvider(cfg.Provider)
	if err != nil {
		log.Fatal("Failed to create provider: ", err)
	}
	var autoDeleteDuration time.Duration
	if cfg.ContainerMaxLifetimeSeconds != 0 {
		autoDeleteDuration = time.Duration(cfg.ContainerMaxLifetimeSeconds) * time.Second
	}

	if autoDeleteDuration != time.Duration(0) && autoDeleteDuration <= time.Minute*10 {
		log.Fatal("ContainerMaxLifetimeSeconds must be greater than 10 minutes, or 0 to disable auto-delete")
	}

	err = newBot.Start(provider, cfg.MaxConcurrentInstances, autoDeleteDuration)
	if err != nil {
		log.Fatal("Error starting the bot: ", err)
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	newBot.Stop()
}
