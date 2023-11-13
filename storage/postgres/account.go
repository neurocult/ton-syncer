package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/eqtlab/ton-syncer/pkg/db"
	"github.com/jackc/pgx/v5"

	"github.com/eqtlab/ton-syncer/syncer"
)

func (s *Storage) GetAndLockAccountBySyncTime(
	ctx context.Context,
	newStart time.Time,
	oldStart time.Time,
	end time.Time,
) (*syncer.Account, error) {
	query := `
		update accounts
		set crypto_start_sync_time = $1
		from (
			select id from accounts
			where (
				crypto_blockchain_id is not null and
				(crypto_end_sync_time is null or crypto_end_sync_time <= $2) and
				(crypto_start_sync_time is null or crypto_start_sync_time <= $3)
			)
			order by crypto_start_sync_time asc
			limit 1
		) as unupdated_account
		where accounts.id = unupdated_account.id
		returning accounts.id, user_id, name, crypto_address, crypto_blockchain_id;
`

	account := &syncer.Account{}
	err := s.db.RawQuery(
		ctx,
		db.ScanOnce(
			&account.ID,
			&account.UserID,
			&account.Name,
			&account.CryptoAddress,
			&account.CryptoBlockchainID,
		),
		query,
		newStart,
		end,
		oldStart,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db select: %w", err)
	}

	return account, nil
}

func (s *Storage) SetAccountEndSynсTime(ctx context.Context, accountID int, time time.Time) error {
	query := sq.
		Update("accounts").
		Set("crypto_end_sync_time", time).
		Where(sq.Eq{"id": accountID})

	if err := s.db.Update(ctx, query, nil); err != nil {
		return fmt.Errorf("db update: %w", err)
	}

	return nil
}
