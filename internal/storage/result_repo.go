package storage

import (
	"DomainPulse/internal/models"
	"database/sql"
)

type ResultRepo struct {
	db *sql.DB
}

func NewResultRepo(db *sql.DB) *ResultRepo { return &ResultRepo{db: db} }

func (r *ResultRepo) Add(res models.Result) error {
	_, err := r.db.Exec(`
		INSERT INTO results(check_id, status_code, duration_ms, outcome)
		VALUES(?, ?, ?, ?)`,
		res.CheckID, res.StatusCode, res.DurationMS, res.Outcome,
	)
	return err
}
