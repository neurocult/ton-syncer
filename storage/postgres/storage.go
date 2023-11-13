package postgres

import (
	"github.com/eqtlab/ton-syncer/pkg/db"
)

// Storage implements syncer.Storage interface via PostgreSQL
type Storage struct {
	db *db.DB
}

func New(db *db.DB) *Storage {
	return &Storage{
		db: db,
	}
}
