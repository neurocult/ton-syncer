package postgres

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/eqtlab/ton-syncer/pkg/db"
	"github.com/eqtlab/ton-syncer/syncer"
	"github.com/jackc/pgx/v5"
)

func (s *Storage) CreateTonTransactions(ctx context.Context, txs []syncer.Transaction) error {
	query := sq.
		Insert("transactions").
		Columns(
			"account_id",
			"asset_id",
			"category_id",
			"merchant",
			"amount",
			"comment",
			"crypto_hash",
			"crypto_ton_lt",
			"effective_at",
		).
		Suffix("on conflict do nothing")

	for _, tx := range txs {
		query = query.Values(
			tx.AccountID,
			tx.AssetID,
			tx.CategoryID,
			tx.Merchant,
			tx.Amount,
			tx.Comment,
			tx.CryptoHash,
			tx.CryptoTonLT,
			tx.EffectiveAt,
		)
	}
	if err := s.db.Insert(ctx, query, nil); err != nil {
		return fmt.Errorf("insert new transaction: %w", err)
	}

	return nil
}

func (s *Storage) IsExistingCryptoTransaction(ctx context.Context, accountID int, cryptoHash string) (bool, error) {
	query := `
		select id from transactions
		where
			transactions.account_id = $1 and
			transactions.crypto_hash = $2 and
			exists(select 1 from accounts where accounts.id = $1);
	`

	var txID int
	err := s.db.RawQuery(ctx, db.ScanOnce(&txID), query, accountID, cryptoHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("db select: %w", err)
	}

	return true, nil
}
