package bot

import (
	"fmt"
	"github.com/l1ghthouse/northstar-bootstrap/src/bot/discord"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

type Bot interface {
	Start(provider providers.Provider, maxConcurrentServers int) error
	Stop()
}

type Config struct {
	Use     string
	Discord discord.Config
}

func NewBot(c Config) (Bot, error) {
	switch c.Use {
	case "discord":
		return discord.NewDiscordBot(c.Discord)
	default:
		return nil, fmt.Errorf("bot %s not supported", c.Use)
	}
}
