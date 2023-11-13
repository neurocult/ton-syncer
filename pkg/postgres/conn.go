package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const pgMaxConns = 500

type Config struct {
	URL            string `env:"URL, required"`
	MigrationsPath string `env:"MIGRATIONS_PATH, default=file://./migrations"`
}

func Connect(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pgcfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}

	pgcfg.MaxConns = pgMaxConns

	pool, err := pgxpool.NewWithConfig(ctx, pgcfg)
	if err != nil {
		return nil, err
	}

	return pool, nil
}
