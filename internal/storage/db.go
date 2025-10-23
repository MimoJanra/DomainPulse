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
		closeErr := db.Close()
		if closeErr != nil {
			fmt.Printf("warning: failed to close db: %v\n", closeErr)
		}
		return nil, fmt.Errorf("error ping db: %w", err)
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS domains (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL
        );
    `)
	if err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS checks (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            domain_id INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
            type TEXT NOT NULL,
            frequency TEXT NOT NULL,
            url TEXT NOT NULL
        );
    `)
	if err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS results (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        check_id INTEGER NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
        status_code INTEGER,
        duration_ms INTEGER,
        outcome TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
`)
	if err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	fmt.Println("db connected")
	return db, nil
}
