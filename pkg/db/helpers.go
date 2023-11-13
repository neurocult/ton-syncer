package db

import (
	"github.com/jackc/pgx/v5"
)

type rowScanner func(rows pgx.Rows) error

func ScanOnce(dest ...any) rowScanner {
	var scanner rowScanner

	if len(dest) > 0 {
		scanner = func(rows pgx.Rows) error {
			return rows.Scan(dest...)
		}
	}

	return scanner
}

type ScanArgs []any

func ScanAll[T any](objs *[]*T, getArgs func(obj *T) ScanArgs) rowScanner {
	return func(rows pgx.Rows) error {
		var obj = new(T)

		if err := rows.Scan(getArgs(obj)...); err != nil {
			return err
		}

		*objs = append(*objs, obj)

		return nil
	}
}
