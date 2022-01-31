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
	MaxServersPerHour      int  `default:"-1"`
	MaxLifetimeSeconds     uint `default:"6900"` // 2 hours - 5 minutes, since vultr charges hourly
}

// MaxLifetimeSeconds could be optimized per cloud provider, depending on the billing cycle.
// For instance, with vultr, the billing cycle is hourly, so the max lifetime could be set to the increments of
// 3600(1 hour) minus 5 minutes, since vultr to save some money on the last billed hour. (Thanks to pg9182 for the suggestion)
