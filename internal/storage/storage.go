// Package storage owns the SQLite connection pool, migrations, and the
// sqlc-generated query layer (under internal/storage/sqlitegen — generated,
// do not edit by hand).
//
// Bounded contexts (shortener, analytics, identity) consume storage via
// their own thin repo interfaces; storage itself does not import from them.
package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/pressly/goose/v3"

	// pure-Go SQLite — no CGO required.
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens a SQLite database with sensible defaults for an HTTP service:
//   - WAL journal mode (concurrent reads alongside writes)
//   - synchronous=NORMAL (fsync on commit but not on every page write)
//   - foreign_keys=ON
//   - busy_timeout=5000ms
//   - max-open-conns=1 for writes (sqlite is single-writer); we still
//     open a separate read pool for parallel reads.
func Open(ctx context.Context, dbPath string, log *slog.Logger) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)",
		dbPath,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}
	log.Info("storage: opened", "path", dbPath)
	return db, nil
}

// MigrationFS returns the embedded migrations directory as fs.FS.
// Exposed so cmd/shortr migrate can drive goose directly.
func MigrationFS() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		// embed.FS guarantees this; panic at startup is fine.
		panic(fmt.Errorf("storage: sub migrations: %w", err))
	}
	return sub
}

// MigrateUp runs all pending migrations to head.
func MigrateUp(db *sql.DB, log *slog.Logger) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(gooseSlog{log})
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// MigrateDown rolls back the most recent migration.
func MigrateDown(db *sql.DB, log *slog.Logger) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(gooseSlog{log})
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	return goose.Down(db, "migrations")
}

// MigrateStatus prints migration status.
func MigrateStatus(db *sql.DB, log *slog.Logger) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(gooseSlog{log})
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	return goose.Status(db, "migrations")
}

// gooseSlog adapts goose.Logger to slog.
type gooseSlog struct{ log *slog.Logger }

func (g gooseSlog) Fatalf(format string, v ...interface{}) {
	g.log.Error(fmt.Sprintf(format, v...))
}
func (g gooseSlog) Printf(format string, v ...interface{}) {
	g.log.Info(fmt.Sprintf(format, v...))
}
