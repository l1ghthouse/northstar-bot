package providers

import (
	"context"
	"fmt"
	"io"

	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers/vultr"
)

type Provider interface {
	CreateServer(context.Context, *nsserver.NSServer) error
	RestartServer(context.Context, *nsserver.NSServer) error
	GetRunningServers(context.Context) ([]*nsserver.NSServer, error)
	DeleteServer(context.Context, *nsserver.NSServer) error
	ExtractServerLogs(context.Context, *nsserver.NSServer) (io.Reader, error)
}

type Config struct {
	Use   string `default:"vultr"`
	Vultr vultr.Config
}

func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Use {
	case "vultr":
		p, err := vultr.NewVultrProvider(cfg.Vultr)
		if err != nil {
			return nil, fmt.Errorf("failed to create vultr provider: %w", err)
		}
		return p, nil
	default:
		return nil, fmt.Errorf("provider %s not supported", cfg.Use)
	}
}
