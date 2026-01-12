package storage

import (
	"database/sql"

	"github.com/MimoJanra/DomainPulse/internal/models"
)

type ResultRepo struct {
	db *sql.DB
}

func NewResultRepo(db *sql.DB) *ResultRepo { return &ResultRepo{db: db} }

func (r *ResultRepo) Add(res models.Result) error {
	_, err := r.db.Exec(`
		INSERT INTO results(check_id, status_code, duration_ms, outcome, created_at)
		VALUES(?, ?, ?, ?, datetime('now'))
	`, res.CheckID, res.StatusCode, res.DurationMS, res.Outcome)
	return err
}
