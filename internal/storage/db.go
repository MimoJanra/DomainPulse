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

	fmt.Println("✅ db connected")
	return db, nil
}
