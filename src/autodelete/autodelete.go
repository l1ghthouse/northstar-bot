package autodelete

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/l1ghthouse/northstar-bootstrap/src/bot/notifyer"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"github.com/l1ghthouse/northstar-bootstrap/src/providers"
)

type Manager struct {
	notifyer    notifyer.Notifyer
	provider    providers.Provider
	maxLifetime time.Duration
	repo        nsserver.Repo
}

// NewAutoDeleteManager creates a new auto delete manager
func NewAutoDeleteManager(repo nsserver.Repo, provider providers.Provider, notifyer notifyer.Notifyer, maxLifetime time.Duration) *Manager {
	return &Manager{
		notifyer:    notifyer,
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
			if time.Since(server.CreatedAt) > d.maxLifetime {
				for _, cached := range cachedServers {
					if server.Name == cached.Name {
						*server = *cached
						break
					}
				}
				d.deleteAndNotify(ctx, server)
			}
		}
	}
}

func (d *Manager) deleteAndNotify(ctx context.Context, server *nsserver.NSServer) {
	var logFile io.Reader
	var err error
	if d.notifyer != nil {
		logFile, err = d.provider.ExtractServerLogs(ctx, server)
		if err != nil {
			log.Println("error extracting logs: ", err)
		}
	}

	err = d.provider.DeleteServer(ctx, server)
	if err != nil {
		log.Println("error deleting server: ", err)
		if d.notifyer != nil {
			d.notifyer.Notify(fmt.Sprintf("error deleting server: %v", err))
		}
	}

	err = d.repo.DeleteByName(ctx, server.Name)
	if err != nil {
		log.Println("error deleting server from database: ", err)
	}

	if d.notifyer != nil {
		d.notifyer.NotifyAndAttach(fmt.Sprintf("Server '%s' has been deleted because it was up for over %s", server.Name, d.maxLifetime.String()), fmt.Sprintf("%s.log.zip", server.Name), logFile)
	}
}
