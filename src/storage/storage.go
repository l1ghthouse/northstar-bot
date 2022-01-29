package storage

import (
	"errors"
	"fmt"

	"github.com/l1ghthouse/northstar-bootstrap/src/storage/sqlitedb"
	"gorm.io/gorm"
)

var ErrDBNotSupported = errors.New("not supported db")

// NewDB creates an instance of database based on config
func NewDB(c Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	if c.Use == "sqlite" {
		db, err = sqlitedb.NewSqliteDB(c.SQLite)
		if err != nil {
			return nil, fmt.Errorf("failed to create sqlite db: %w", err)
		}
	} else {
		return nil, ErrDBNotSupported
	}
	return db, nil
}

type Config struct {
	Use    string `default:"sqlite"`
	Prefix string `default:""`
	SQLite sqlitedb.Config
}
