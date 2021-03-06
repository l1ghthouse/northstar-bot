package main

import (
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/l1ghthouse/northstar-bootstrap/src/storage"
	"github.com/l1ghthouse/northstar-bootstrap/src/storage/orm"

	"github.com/jinzhu/configor"
	"github.com/l1ghthouse/northstar-bootstrap/src/bot"
	"github.com/l1ghthouse/northstar-bootstrap/src/config"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

// nolint: cyclop
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

	database, err := storage.NewDB(cfg.DB)
	if err != nil {
		log.Fatal("Failed to create db: ", err)
	}

	err = database.AutoMigrate(&nsserver.NSServer{})
	if err != nil {
		log.Fatal("Failed to migrate db: ", err)
	}

	nsRepo := orm.NewNSServerRepo(database)

	var autoDeleteDuration time.Duration
	if cfg.MaxLifetimeSeconds != 0 {
		autoDeleteDuration = time.Duration(cfg.MaxLifetimeSeconds) * time.Second
	}

	if autoDeleteDuration != time.Duration(0) && autoDeleteDuration <= time.Minute*10 {
		log.Fatal("ContainerMaxLifetimeSeconds must be greater than 10 minutes, or 0 to disable auto-delete")
	}

	var maxServerRate uint

	if cfg.MaxServersPerHour < 0 {
		maxServerRate = cfg.MaxConcurrentInstances * 2
	} else {
		maxServerRate = uint(cfg.MaxServersPerHour)
	}

	var maxExtendDuration time.Duration
	if cfg.MaxServerExtendDurationSeconds != 0 {
		maxExtendDuration = time.Duration(cfg.MaxServerExtendDurationSeconds) * time.Second
	}

	autoDeleteManager, err := newBot.Start(provider, nsRepo, cfg.MaxConcurrentInstances, maxServerRate, autoDeleteDuration, maxExtendDuration)
	if err != nil {
		log.Fatal("Error starting the bot: ", err)
	}

	if autoDeleteDuration != time.Duration(0) {
		go autoDeleteManager.AutoDelete()
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	newBot.Stop()
}
