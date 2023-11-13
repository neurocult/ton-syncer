package config

import (
	"context"

	"github.com/sethvargo/go-envconfig"

	"github.com/eqtlab/ton-syncer/pkg/postgres"
	"github.com/eqtlab/ton-syncer/syncer"
)

type Config struct {
	Debug  bool            `env:"APP_DEBUG"`
	DB     postgres.Config `env:",prefix=DB_"`
	Syncer syncer.Config   `env:",prefix=SYNCER_"`
}

func ParseEnv(ctx context.Context) (Config, error) {
	cfg := Config{}
	return cfg, envconfig.Process(ctx, &cfg)
}
