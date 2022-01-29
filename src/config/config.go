package config

import (
	"github.com/l1ghthouse/northstar-bootstrap/src/bot"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
	"github.com/l1ghthouse/northstar-bootstrap/src/storage"
)

type Config struct {
	Provider               providers.Config
	Bot                    bot.Config
	DB                     storage.Config
	MaxConcurrentInstances uint `default:"1"`
	MaxLifetimeSeconds     uint `default:"7200"` // 2 hours
}
