package nsserver

import (
	"context"

	"github.com/gofrs/uuid"
)

type Repo interface {
	DeleteByID(ctx context.Context, id uuid.UUID) error
	DeleteByName(ctx context.Context, name string) error
	GetAll(ctx context.Context) ([]*NSServer, error)
	GetByID(ctx context.Context, id uuid.UUID) (*NSServer, error)
	GetByName(ctx context.Context, name string) (*NSServer, error)
	Store(ctx context.Context, u []*NSServer) error
	// Update(ctx context.Context, u *NSServer) error
}
