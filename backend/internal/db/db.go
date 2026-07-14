// Package db wraps the SQLite database (pure-Go modernc driver) that stores
// users, sessions and monitoring history. Service definitions live in
// config.yaml, not here. Schema changes are managed by goose migrations in
// migrations/*.sql.
package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// DB is a thin wrapper around *sql.DB with domain queries.
type DB struct {
	*sql.DB
}

//go:embed migrations/*.sql
var embedMigrations embed.FS

// Open opens (creating if needed) the SQLite database at path and runs migrations.
func Open(path string) (*DB, error) {
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// SQLite has a single writer; serialize connections to avoid lock contention.
	sqlDB.SetMaxOpenConns(1)

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return &DB{sqlDB}, nil
}

// migrate runs the embedded goose migrations against the SQLite database.
func migrate(sqlDB *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(goose.NopLogger()) // keep goose off stdout; we log via slog
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set migrations dialect: %w", err)
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
