package bot

import (
	"fmt"
	"time"

	"github.com/l1ghthouse/northstar-bootstrap/src/autodelete"

	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"

	"github.com/l1ghthouse/northstar-bootstrap/src/bot/discord"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

type Bot interface {
	Start(provider providers.Provider, repo nsserver.Repo, maxConcurrentServers uint, MaxServersPerHour uint, autoDeleteDuration time.Duration) (*autodelete.Manager, error)
	Stop()
}

type Config struct {
	Use     string
	Discord discord.Config
}

// NewBot returns a new bot instance, depending on the cfg.Use value.
// nolint: ireturn
func NewBot(cfg Config) (Bot, error) {
	switch cfg.Use {
	case "discord":
		bot, err := discord.NewDiscordBot(cfg.Discord)
		if err != nil {
			return nil, fmt.Errorf("failed to create discord bot: %w", err)
		}
		return bot, nil
	default:
		return nil, fmt.Errorf("bot %s not supported", cfg.Use)
	}
}
