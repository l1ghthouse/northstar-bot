package config

import (
	"github.com/l1ghthouse/northstar-bootstrap/src/bot"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

type Config struct {
	Provider               providers.Config
	Bot                    bot.Config
	MaxConcurrentInstances int `default:"1"`
}
