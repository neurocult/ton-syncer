package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/sourcegraph/conc/panics"
	"github.com/vgarvardt/gue/v5"
	"github.com/vgarvardt/gue/v5/adapter/pgxv5"
	adapter "github.com/vgarvardt/gue/v5/adapter/zap"
	"github.com/xssnick/tonutils-go/liteclient"
	tonutils "github.com/xssnick/tonutils-go/ton"
	"go.uber.org/zap"

	"github.com/eqtlab/ton-syncer/config"
	"github.com/eqtlab/ton-syncer/pkg/db"
	"github.com/eqtlab/ton-syncer/pkg/logger"
	"github.com/eqtlab/ton-syncer/pkg/postgres"
	"github.com/eqtlab/ton-syncer/pkg/ton"
	storage "github.com/eqtlab/ton-syncer/storage/postgres"
	"github.com/eqtlab/ton-syncer/syncer"
)

func main() {
	ctx := context.Background()
	log := logger.New(true)

	cfg, err := config.ParseEnv(ctx)
	if err != nil {
		log.Fatal("can't parse configuration", zap.Error(err))
	}

	log = logger.New(cfg.Debug)

	pool, err := postgres.Connect(ctx, cfg.DB)
	if err != nil {
		log.Fatal("can't connect to db", zap.Error(err))
	}
	defer pool.Close()

	database := db.NewDB(pool, log)
	store := storage.New(database)

	tonCfg, err := liteclient.GetConfigFromUrl(ctx, "https://ton-blockchain.github.io/global.config.json")
	if err != nil {
		log.Fatal("liteclient get config from url", zap.Error(err))
	}

	ip, key := ton.FindArchiveNode(ctx, tonCfg)
	if ip == "" || key == "" {
		log.Fatal("archive node not found")
	}

	tonPool := liteclient.NewConnectionPool()
	err = tonPool.AddConnection(ctx, ip, key)
	if err != nil {
		log.Fatal("liteclient new connection pool", zap.Error(err))
	}

	poolAdapter := pgxv5.NewConnPool(pool)
	q, err := gue.NewClient(poolAdapter, gue.WithClientLogger(adapter.New(log.Logger)))
	if err != nil {
		log.Fatal("pgx adapter for gue", zap.Error(err))
	}

	api := tonutils.NewAPIClient(tonPool)
	tonSyncer := syncer.New(store, q, api, tonPool, log.Logger, cfg.Syncer)

	runForever(
		log,
		func() { tonSyncer.Sync(ctx) },
	)

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	<-exit
	log.Info("bot has been stopped")
}

// runForever spawns goroutine for every f in ff. Each f is logged and restarted if panic occurs. It's non-blocking.
func runForever(log *logger.Logger, ff ...func()) {
	for i := range ff {
		f := ff[i]
		go func() {
			var pc panics.Catcher
			pc.Try(f)
			if err := pc.Recovered().AsError(); err != nil {
				log.Error("panic", zap.Error(err))
				time.Sleep(time.Minute)
				runForever(log, f)
			}
		}()
	}
}
