package storage

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "database.db")
	if err != nil {
		return nil, fmt.Errorf("error open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error ping db: %w", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS domains (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE
	);
	`)
	if err != nil {
		return nil, fmt.Errorf("error creating domains table: %w", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS checks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_id INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
		type TEXT NOT NULL,
		path TEXT NOT NULL,
		interval_seconds INTEGER NOT NULL DEFAULT 60,
		params TEXT NOT NULL DEFAULT '{}',
		enabled INTEGER NOT NULL DEFAULT 1,
		realtime_mode INTEGER NOT NULL DEFAULT 0,
		rate_limit_per_minute INTEGER NOT NULL DEFAULT 0
	);
	`)
	if err != nil {
		return nil, fmt.Errorf("error creating checks table: %w", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		check_id INTEGER NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
		status TEXT NOT NULL DEFAULT 'success',
		status_code INTEGER,
		duration_ms INTEGER NOT NULL,
		outcome TEXT,
		error_message TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`)
	if err != nil {
		return nil, fmt.Errorf("error creating results table: %w", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS notification_settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		token TEXT,
		chat_id TEXT,
		webhook_url TEXT,
		notify_on_failure INTEGER NOT NULL DEFAULT 1,
		notify_on_success INTEGER NOT NULL DEFAULT 0,
		notify_on_slow_response INTEGER NOT NULL DEFAULT 0,
		slow_response_threshold_ms INTEGER NOT NULL DEFAULT 0
	);
	`)
	if err != nil {
		return nil, fmt.Errorf("error creating notification_settings table: %w", err)
	}

	_, _ = db.Exec(`ALTER TABLE notification_settings ADD COLUMN notify_on_slow_response INTEGER NOT NULL DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE notification_settings ADD COLUMN slow_response_threshold_ms INTEGER NOT NULL DEFAULT 0`)

	return db, nil
}
