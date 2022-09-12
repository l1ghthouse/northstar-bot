package autodelete

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/l1ghthouse/northstar-bootstrap/src/bot/notifier"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

type Manager struct {
	notifier    notifier.Notifier
	provider    providers.Provider
	maxLifetime time.Duration
	repo        nsserver.Repo
}

// NewAutoDeleteManager creates a new auto delete manager
func NewAutoDeleteManager(repo nsserver.Repo, provider providers.Provider, notifier notifier.Notifier, maxLifetime time.Duration) *Manager {
	return &Manager{
		notifier:    notifier,
		provider:    provider,
		maxLifetime: maxLifetime,
		repo:        repo,
	}
}

func (d *Manager) AutoDelete() {
	ticker := time.NewTicker(time.Minute * 2)
	ctx := context.Background()
	for range ticker.C {
		servers, err := d.provider.GetRunningServers(ctx)
		if err != nil {
			log.Println("error getting running servers: ", err)
			continue
		}

		cachedServers, err := d.repo.GetAll(ctx)
		if err != nil {
			log.Println("error getting cached servers: ", err)
			continue
		}
		for _, server := range servers {
			for _, cached := range cachedServers {
				if server.Name == cached.Name {
					*server = *cached
					break
				}
			}
			maxLifetime := d.maxLifetime
			if server.ExtendLifetime != nil {
				maxLifetime += *server.ExtendLifetime
			}
			if time.Since(server.CreatedAt) > maxLifetime {
				d.deleteAndNotify(ctx, server)
			}
		}
	}
}

func (d *Manager) deleteAndNotify(ctx context.Context, server *nsserver.NSServer) {
	var logFile *bytes.Buffer
	var err error
	if d.notifier != nil {
		logFile, err = d.provider.ExtractServerLogs(ctx, server)
		if err != nil {
			log.Println("error extracting logs: ", err)
		}
	}

	err = d.provider.DeleteServer(ctx, server)
	if err != nil {
		log.Println("error deleting server: ", err)
		if d.notifier != nil {
			d.notifier.NotifyServer(server, fmt.Sprintf("error deleting server: %v", err))
		}
	}

	err = d.repo.DeleteByName(ctx, server.Name)
	if err != nil {
		log.Println("error deleting server from database: ", err)
	}

	if d.notifier != nil {
		d.notifier.NotifyAndAttachServerData(server, fmt.Sprintf("Deleted because it was up for over %s", d.maxLifetime.String()), fmt.Sprintf("%s.log.zip", server.Name), logFile)
	}
}
