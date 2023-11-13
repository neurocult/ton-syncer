package syncer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"go.uber.org/zap"

	timeutils "github.com/eqtlab/ton-syncer/pkg/time"
)

// actualizer finds account that must be synced, sets the lock for it by setting its start time equal to now
// and checks if the last transaction from blockchain already exists in storage.
// If its not there it enqueues a task for updater queue. Actualizer never fail.
func (s *Syncer) actualizer(ctx context.Context) {
	time.Sleep(s.cfg.ActualizerStartDelay)
	for range timeutils.TickWithCtx(ctx, s.cfg.AccountsCheckInterval) {
		err := s.iteration(ctx)
		if err != nil {
			s.logger.Error("actualizer worker failed", zap.Error(err)) // if one fail we continue
		}
	}
}

var ErrAccountWithoutAddr = errors.New("account found but has not crypto address")

func (s *Syncer) iteration(ctx context.Context) error {
	start, end := getSyncTime(time.Now(), s.cfg.UpdaterLock, s.cfg.AccountSyncInterval)

	account, err := s.storage.GetAndLockAccountBySyncTime(ctx, time.Now(), start, end)
	if err != nil {
		return fmt.Errorf("storage get and lock account by sync time: %w", err)
	}

	if account == nil {
		s.logger.Debug("actualizer: account not found by sync times - all are up to date or being updated right now")
		return nil
	}

	if account.CryptoAddress == nil {
		return fmt.Errorf("%w: %v", ErrAccountWithoutAddr, account)
	}

	tonAccount, err := s.getTonAccount(*account.CryptoAddress, ctx)
	if errors.Is(err, errTonAccNotInitialized) {
		s.logger.Debug(
			"actualizer: ton account found but it's not initialized",
			zap.String("ton_address", *account.CryptoAddress),
		)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get ton account: %w", err)
	}

	lastTxHashStr := txHashToString(tonAccount.LastTxHash)

	ok, err := s.storage.IsExistingCryptoTransaction(ctx, account.ID, lastTxHashStr)
	if err != nil {
		return fmt.Errorf("is existing crypto transaction: %w", err)
	}
	if ok {
		s.logger.Debug(
			"actualizer: account is already up to date",
			zap.Int("account_id", account.ID),
			zap.String("transaction_hash", lastTxHashStr),
		)
		if err := s.storage.SetAccountEndSynсTime(ctx, account.ID, time.Now()); err != nil {
			s.logger.Error(
				"actualizer: failed to update account end synk time, account was up to date already",
				zap.Error(err),
				zap.Int("account_id", account.ID),
			)
		}
		return nil
	}

	if err := s.enqueue(
		ctx,
		tonAccount.State.Address,
		account.ID,
		tonAccount.LastTxHash,
		tonAccount.LastTxLT,
	); err != nil {
		return fmt.Errorf("enqueue: %w", err)
	}

	return nil
}

var errTonAccNotInitialized = errors.New("ton account not initialized")

// getTonAccount returns errTonAccNotInitialized if account is found but not initialized.
func (s *Syncer) getTonAccount(strAddr string, ctx context.Context) (*tlb.Account, error) {
	addr, err := address.ParseAddr(strAddr)
	if err != nil {
		return nil, fmt.Errorf("ton get account: %w", err)
	}

	block, err := s.ton.CurrentMasterchainInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("ton get account: %w", err)
	}

	tonAcc, err := s.ton.GetAccount(ctx, block, addr)
	if err != nil {
		return nil, fmt.Errorf("ton get account: %w", err)
	}

	if tonAcc == nil || tonAcc.State == nil {
		return nil, errTonAccNotInitialized
	}

	return tonAcc, nil
}

// getSyncTime returns newest possible start and end synk times for account that needs to be synced.
func getSyncTime(now time.Time, lockDur, syncDelay time.Duration) (start time.Time, end time.Time) {
	end = now.Add(-syncDelay) // if account was synked before this time, it's old enough to be synced again
	start = now.Add(-lockDur) // also make sure no worker is pricessing this account now by checking start sync time
	return start, end
}
