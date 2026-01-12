package storage

import (
	"database/sql"
	"time"

	"github.com/MimoJanra/DomainPulse/internal/models"
)

type ResultRepo struct {
	db *sql.DB
}

func NewResultRepo(db *sql.DB) *ResultRepo { return &ResultRepo{db: db} }

func (r *ResultRepo) Add(res models.Result) error {
	timestamp := res.CreatedAt
	if timestamp == "" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	_, err := r.db.Exec(`
		INSERT INTO results(check_id, status, status_code, duration_ms, outcome, error_message, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)
	`, res.CheckID, res.Status, res.StatusCode, res.DurationMS, res.Outcome, res.ErrorMessage, timestamp)
	return err
}

func (r *ResultRepo) GetByCheckID(checkID int) ([]models.Result, error) {
	rows, err := r.db.Query(`
		SELECT id, check_id, status, status_code, duration_ms, outcome, error_message, created_at
		FROM results
		WHERE check_id = ?
		ORDER BY created_at DESC
	`, checkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Result
	for rows.Next() {
		var res models.Result
		if err := rows.Scan(&res.ID, &res.CheckID, &res.Status, &res.StatusCode, &res.DurationMS, &res.Outcome, &res.ErrorMessage, &res.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, rows.Err()
}

func (r *ResultRepo) GetAll() ([]models.Result, error) {
	rows, err := r.db.Query(`
		SELECT id, check_id, status, status_code, duration_ms, outcome, error_message, created_at
		FROM results
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Result
	for rows.Next() {
		var res models.Result
		if err := rows.Scan(&res.ID, &res.CheckID, &res.Status, &res.StatusCode, &res.DurationMS, &res.Outcome, &res.ErrorMessage, &res.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, rows.Err()
}

func (r *ResultRepo) GetByID(id int) (models.Result, error) {
	row := r.db.QueryRow(`
		SELECT id, check_id, status, status_code, duration_ms, outcome, error_message, created_at
		FROM results
		WHERE id = ?
	`, id)
	var res models.Result
	err := row.Scan(&res.ID, &res.CheckID, &res.Status, &res.StatusCode, &res.DurationMS, &res.Outcome, &res.ErrorMessage, &res.CreatedAt)
	return res, err
}
