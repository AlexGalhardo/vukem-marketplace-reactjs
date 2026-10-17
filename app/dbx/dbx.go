// Package dbx holds the small SQL helpers the commerce packages share so
// hand-written statements run on Postgres (production) and SQLite (unit
// tests): placeholder rebinding and row locks.
package dbx

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/AlexGalhardo/vukem/database"
)

// Rebind rewrites "?" placeholders into the dialect's form ($1, $2… on
// Postgres). Statements must not contain literal question marks.
//
// workaround(vukem): database has no exported placeholder rebinding for raw SQL.
func Rebind(d database.Dialect, query string) string {
	if d.Name() != "postgres" {
		return query
	}

	var b strings.Builder

	b.Grow(len(query) + 8)

	n := 0

	for _, r := range query {
		if r == '?' {
			n++

			b.WriteString("$" + strconv.Itoa(n))

			continue
		}

		b.WriteRune(r)
	}

	return b.String()
}

// Exec runs a statement with "?" placeholders and returns the affected rows.
func Exec(ctx context.Context, ex database.Executor, query string, args ...any) (int64, error) {
	res, err := ex.ExecContext(ctx, Rebind(ex.Dialect(), query), args...)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

// QueryRow runs a single-row query with "?" placeholders.
func QueryRow(ctx context.Context, ex database.Executor, query string, args ...any) RowScanner {
	return ex.QueryRowContext(ctx, Rebind(ex.Dialect(), query), args...)
}

// RowScanner is the Scan method of *sql.Row.
type RowScanner interface {
	Scan(dest ...any) error
}

// ForUpdate returns " FOR UPDATE" on Postgres; SQLite serializes writers
// already and has no row locks.
func ForUpdate(d database.Dialect) string {
	if d.Name() == "postgres" {
		return " FOR UPDATE"
	}

	return ""
}

// IsUniqueViolation reports whether err is a unique constraint failure on
// Postgres (SQLSTATE 23505) or SQLite.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	type sqlStater interface {
		error
		SQLState() string
	}

	if s, ok := errors.AsType[sqlStater](err); ok {
		return s.SQLState() == "23505"
	}

	msg := err.Error()

	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "SQLSTATE 23505")
}
