package providers

import (
	"context"
	"fmt"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers/vultr"
)

type Provider interface {
	CreateServer(context.Context, nsserver.NSServer) (nsserver.NSServer, error)
	GetRunningServers(context.Context) ([]nsserver.NSServer, error)
	DeleteServer(context.Context, nsserver.NSServer) error
}

type Config struct {
	Use   string `default:"vultr"`
	Vultr vultr.Config
}

func NewProvider(c Config) (Provider, error) {
	switch c.Use {
	case "vultr":
		return vultr.NewVultrProvider(c.Vultr)
	default:
		return nil, fmt.Errorf("provider %s not supported", c.Use)
	}
}
