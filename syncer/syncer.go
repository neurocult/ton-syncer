package syncer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/pool"
	"github.com/vgarvardt/gue/v5"
	adapter "github.com/vgarvardt/gue/v5/adapter/zap"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/ton"
	"go.uber.org/zap"
)

// Syncer keeps accounts in sync by polling ton api and inserting missing transactions into the storage
type Syncer struct {
	cfg     Config
	storage Storage
	q       *gue.Client
	ton     *ton.APIClient
	tonpool *liteclient.ConnectionPool
	logger  *zap.Logger
}

type Storage interface {
	// GetAndLockAccountBySyncTime returns account where start and end sync times are less or equal to the given ones, also sets start_sync_time to the given `now`
	GetAndLockAccountBySyncTime(ctx context.Context, now time.Time, start time.Time, end time.Time) (*Account, error)
	// IsExistingCryptoTransaction checks whether given account has transaction with the given hash
	IsExistingCryptoTransaction(ctx context.Context, accountID int, cryptoHash string) (bool, error)
	// SetAccountEndSynсTime sets account's sync time to the given one
	SetAccountEndSynсTime(ctx context.Context, accountID int, syncTime time.Time) error
	// CreateTonTransactions inserts transactions into storage
	CreateTonTransactions(context.Context, []Transaction) error
}

func New(
	s Storage,
	q *gue.Client,
	t *ton.APIClient,
	tonpool *liteclient.ConnectionPool,
	l *zap.Logger,
	cfg Config,
) *Syncer {
	return &Syncer{
		storage: s,
		q:       q,
		ton:     t,
		tonpool: tonpool,
		logger:  l,
		cfg:     cfg,
	}
}

const queueType = "update"

// Sync panics if it can't initialize worker queue
func (s *Syncer) Sync(ctx context.Context) {
	newCtx, cancel := context.WithCancel(ctx)

	actualizers := pool.New().WithMaxGoroutines(s.cfg.WorkerPoolSize)
	for i := 0; i < s.cfg.WorkerPoolSize; i++ {
		actualizers.Go(func() { s.actualizer(newCtx) })
	}

	updaters, err := gue.NewWorkerPool(
		s.q,
		gue.WorkMap{queueType: s.updater},
		s.cfg.WorkerPoolSize,
		gue.WithPoolLogger(adapter.New(s.logger)),
	)
	if err != nil {
		s.logger.Fatal("gue new worker pool", zap.Error(err))
	}

	s.logger.Info("syncer has started")

	// run actualizers and updaters concurrently and cancel ctx as soon one of them exit so another exit too
	var wg conc.WaitGroup
	wg.Go(func() {
		defer cancel()
		actualizers.Wait()
	})
	wg.Go(func() {
		defer cancel()
		if err := updaters.Run(newCtx); err != nil {
			s.logger.Fatal("updaters run", zap.Error(err))
		}
	})
	wg.Wait()
}

type jobArgs struct {
	Addr      string `json:"addr"` // we don't store address.Address because of it's not-marshallable private fields
	AccountID int    `json:"accountId"`
	TxHash    []byte `json:"TxHash"`
	TxLT      uint64 `json:"txLt"`
}

func (s *Syncer) enqueue(ctx context.Context, addr *address.Address, accountID int, hash []byte, lt uint64) error {
	args := jobArgs{
		Addr:      addr.String(),
		AccountID: accountID,
		TxHash:    hash,
		TxLT:      lt,
	}

	bb, err := json.Marshal(&args)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	if err := s.q.Enqueue(ctx, &gue.Job{Type: queueType, Args: bb}); err != nil {
		return fmt.Errorf("gue enqueue: %w", err)
	}

	return nil
}

func txHashToString(bb []byte) string {
	return base64.StdEncoding.EncodeToString(bb)
}

// nolint:lll
type Config struct {
	WorkerPoolSize        int           `env:"WORKER_POOL_SIZE, default=1"`          // How many actualizers and updaters to spawn
	AccountsCheckInterval time.Duration `env:"ACCOUNTS_CHECK_INTERVAL, default=10s"` // How long one actualizer wait before new account lookup
	ActualizerStartDelay  time.Duration `env:"ACTUALIZER_START_DELAY, default=1s"`   // How much time to wait before spawn next actualizer in a pool
	AccountSyncInterval   time.Duration `env:"ACCOUNT_SYNC_INTERVAL, default=10m"`   // How frequently each account must be synced
	UpdaterLock           time.Duration `env:"UPDATER_LOCK_TIMEOUT, default=10s"`    // How much time updater have to process one account
	AssetID int `env:"UPDATER_ASSET_ID, default=0"`    // AssetID that updater will use when inserting new transactions into the storage
}
