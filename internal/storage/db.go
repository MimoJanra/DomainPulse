package storage

import (
	"database/sql"
	"fmt"
	"strings"

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
		frequency TEXT NOT NULL,
		path TEXT NOT NULL
	);
	`)
	if err != nil {
		return nil, fmt.Errorf("error creating checks table: %w", err)
	}

	if err := addColumnIfMissing(db, "checks", "interval_seconds", "INTEGER NOT NULL DEFAULT 60"); err != nil {
		return nil, fmt.Errorf("error ensuring interval_seconds column: %w", err)
	}
	if err := addColumnIfMissing(db, "checks", "params", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return nil, fmt.Errorf("error ensuring params column: %w", err)
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

	if err := addColumnIfMissing(db, "results", "status", "TEXT NOT NULL DEFAULT 'success'"); err != nil {
		return nil, fmt.Errorf("error ensuring status column: %w", err)
	}
	if err := addColumnIfMissing(db, "results", "error_message", "TEXT"); err != nil {
		return nil, fmt.Errorf("error ensuring error_message column: %w", err)
	}

	return db, nil
}

func addColumnIfMissing(db *sql.DB, table, column, definition string) error {
	_, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	return nil
}
