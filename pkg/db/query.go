package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/eqtlab/ton-syncer/pkg/logger"
)

type conn interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
}

func NewDB(pool *pgxpool.Pool, log *logger.Logger) *DB {
	return &DB{
		log:  log,
		pool: pool,
		conn: pool,
	}
}

type DB struct {
	log  *logger.Logger
	pool *pgxpool.Pool
	conn conn
}

func (db *DB) Select(ctx context.Context, query squirrel.SelectBuilder, handler func(rows pgx.Rows) error) error {
	sql, args, err := query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return fmt.Errorf("build select query: %w", err)
	}

	if err := db.query(ctx, sql, args, handler); err != nil {
		return fmt.Errorf("exec select query: %w", err)
	}

	return nil
}

func (db *DB) Insert(ctx context.Context, query squirrel.InsertBuilder, handler func(rows pgx.Rows) error) error {
	sql, args, err := query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return fmt.Errorf("build insert query: %w", err)
	}

	if err := db.query(ctx, sql, args, handler); err != nil {
		return fmt.Errorf("exec insert query: %w", err)
	}

	return nil
}

func (db *DB) Update(ctx context.Context, query squirrel.UpdateBuilder, handler func(rows pgx.Rows) error) error {
	sql, args, err := query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return fmt.Errorf("build update query: %w", err)
	}

	if err := db.query(ctx, sql, args, handler); err != nil {
		return fmt.Errorf("exec update query: %w", err)
	}

	return nil
}

func (db *DB) Delete(ctx context.Context, query squirrel.DeleteBuilder, handler func(rows pgx.Rows) error) error {
	sql, args, err := query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return fmt.Errorf("build delete query: %w", err)
	}

	if err := db.query(ctx, sql, args, handler); err != nil {
		return fmt.Errorf("exec delete query: %w", err)
	}

	return nil
}

func (db *DB) query(ctx context.Context, sql string, args []any, scanner rowScanner) error {
	start := time.Now()
	defer func() {
		go db.logQuery(time.Since(start), sql, args) // don't block flow
	}()

	rows, err := db.conn.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("exec query: %w", err)
	}
	defer rows.Close()

	var isAnyRowProcessed bool
	for rows.Next() {
		if scanner == nil {
			continue
		}

		if err = scanner(rows); err != nil {
			return fmt.Errorf("handle row: %w", err)
		}

		isAnyRowProcessed = true
	}

	// Err must only be called after the Rows is closed (either by calling Close or by Next returning false)
	if err = rows.Err(); err != nil {
		return fmt.Errorf("reading query result: %w", err)
	}

	if scanner != nil && !isAnyRowProcessed {
		return pgx.ErrNoRows
	}

	return nil
}

func (db *DB) RawQuery(ctx context.Context, handler func(rows pgx.Rows) error, sql string, args ...interface{}) error {
	return db.query(ctx, sql, args, handler)
}

func (db *DB) RunInTransaction(ctx context.Context, f func(ctx context.Context, txDB *DB) error) error {
	err := pgx.BeginTxFunc(ctx, db.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		txDB := &DB{
			log:  db.log,
			pool: db.pool,
			conn: tx,
		}

		if err := f(ctx, txDB); err != nil {
			return fmt.Errorf("run in transaction: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	return nil
}

func (db *DB) logQuery(dur time.Duration, sql string, args ...any) {
	sql = strings.ReplaceAll(sql, "\t", " ")
	sql = strings.ReplaceAll(sql, "\n", " ")
	sql = strings.Trim(sql, " ")
	db.log.Debug(
		"DB Request",
		zap.String("SQL", sql),
		zap.Any("Args", args),
		zap.Int32("Connection Limit", db.pool.Stat().MaxConns()),
		zap.Int32("Connection Used", db.pool.Stat().TotalConns()),
		zap.Duration("dur", dur),
	)
}
