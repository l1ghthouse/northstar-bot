package orm

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"gorm.io/gorm"
)

type nsserverRepo struct {
	db *gorm.DB
}

func NewNSServerRepo(db *gorm.DB) nsserver.Repo {
	return &nsserverRepo{db}
}

var ErrNoRowsAffected = fmt.Errorf("no rows affected")

func (h *nsserverRepo) DeleteByID(ctx context.Context, id uuid.UUID) error {
	result := h.db.WithContext(ctx).Delete(&nsserver.NSServer{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("error deleting nsserver with id: %s, err: %w", id.String(), result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNoRowsAffected
	}
	return nil
}

func (h *nsserverRepo) DeleteByName(ctx context.Context, name string) error {
	result := h.db.WithContext(ctx).Delete(&nsserver.NSServer{}, "name = ?", name)
	if result.Error != nil {
		return fmt.Errorf("error deleting nsserver with name: %s, err: %w", name, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNoRowsAffected
	}
	return nil
}

func (h *nsserverRepo) GetAll(ctx context.Context) ([]*nsserver.NSServer, error) {
	nsservers := make([]*nsserver.NSServer, 0)
	err := h.db.WithContext(ctx).Find(&nsservers).Error
	if err != nil {
		return nil, err
	}
	return nsservers, nil
}

func (h *nsserverRepo) GetByID(ctx context.Context, id uuid.UUID) (*nsserver.NSServer, error) {
	server := &nsserver.NSServer{}
	err := h.db.WithContext(ctx).Where("id = ?", id).First(server).Error
	if err != nil {
		return nil, err
	}
	return server, nil
}

func (h *nsserverRepo) GetByName(ctx context.Context, name string) (*nsserver.NSServer, error) {
	server := &nsserver.NSServer{}
	err := h.db.WithContext(ctx).Where("name = ?", name).First(server).Error
	if err != nil {
		return nil, err
	}
	return server, nil
}

func (h *nsserverRepo) Store(ctx context.Context, server []*nsserver.NSServer) error {
	err := h.db.WithContext(ctx).Create(server).Error
	if err != nil {
		return err
	}
	return nil
}

func (h *nsserverRepo) Update(ctx context.Context, server *nsserver.NSServer) error {
	err := h.db.WithContext(ctx).Model(server).Updates(nsserver.NSServer{
		ExtendLifetime: server.ExtendLifetime,
	}).Error
	if err != nil {
		return err
	}
	return nil
}
