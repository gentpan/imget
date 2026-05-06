package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type DB struct {
	*sql.DB
	path string
}

func Open(path string) (*DB, error) {
	if path == "" {
		return nil, errors.New("db: empty path")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("db: mkdir %s: %w", dir, err)
		}
	}

	// Per-connection PRAGMAs in the DSN so EVERY connection in the pool runs
	// them at open time. SQLite WAL mode (set once persistently below) lets
	// multiple readers run concurrently with one writer, so a multi-conn
	// pool actually parallelizes hot-path read queries.
	dsn := "file:" + path +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(on)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=cache_size(-64000)" +
		"&_pragma=mmap_size(268435456)" +
		"&_pragma=temp_store(MEMORY)"
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	// WAL mode supports concurrent readers + 1 writer. modernc/sqlite
	// serializes writes internally, so 8 connections is safe and lets reads
	// scale across CPU cores.
	sqldb.SetMaxOpenConns(8)
	sqldb.SetMaxIdleConns(8)
	sqldb.SetConnMaxIdleTime(5 * time.Minute)

	db := &DB{DB: sqldb, path: path}
	if err := db.Migrate(); err != nil {
		_ = sqldb.Close()
		return nil, err
	}
	return db, nil
}

// Path returns the on-disk SQLite path.
func (d *DB) Path() string { return d.path }

func (d *DB) Migrate() error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("db: read migrations: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".sql" {
			continue
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("db: read %s: %w", name, err)
		}
		if _, err := d.Exec(string(body)); err != nil {
			return fmt.Errorf("db: apply %s: %w", name, err)
		}
	}
	return nil
}

// Checkpoint forces a WAL truncate. Useful to keep -wal file small.
func (d *DB) Checkpoint(ctx context.Context) error {
	_, err := d.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}
