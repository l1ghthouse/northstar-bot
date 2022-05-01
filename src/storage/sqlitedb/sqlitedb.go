package sqlitedb

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Config struct {
}

func NewSqliteDB(config Config) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("./nsbot.db"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sqlite: %w", err)
	}
	return db, nil
}
