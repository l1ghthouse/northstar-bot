package config

import (
	"github.com/l1ghthouse/northstar-bootstrap/src/bot"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

type Config struct {
	Provider                    providers.Config
	Bot                         bot.Config
	MaxConcurrentInstances      uint `default:"1"`
	ContainerMaxLifetimeSeconds uint `default:"7200"` // 2 hours
}
