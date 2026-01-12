package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/MimoJanra/DomainPulse/internal/models"
)

type CheckRepo struct {
	db *sql.DB
}

func NewCheckRepo(db *sql.DB) *CheckRepo { return &CheckRepo{db: db} }

func (r *CheckRepo) Add(domainID int, checkType string, intervalSeconds int, params models.CheckParams) (models.Check, error) {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return models.Check{}, fmt.Errorf("marshal params: %w", err)
	}

	frequency := fmt.Sprintf("%ds", intervalSeconds)
	path := params.Path

	res, err := r.db.Exec(
		"INSERT INTO checks(domain_id, type, frequency, path, interval_seconds, params) VALUES(?, ?, ?, ?, ?, ?)",
		domainID, checkType, frequency, path, intervalSeconds, string(paramsJSON),
	)
	if err != nil {
		return models.Check{}, err
	}
	id, _ := res.LastInsertId()
	return models.Check{
		ID:              int(id),
		DomainID:        domainID,
		Type:            checkType,
		Frequency:       frequency,
		Path:            path,
		IntervalSeconds: intervalSeconds,
		Params:          params,
	}, nil
}

func (r *CheckRepo) GetByDomainID(domainID int) ([]models.Check, error) {
	rows, err := r.db.Query("SELECT id, domain_id, type, frequency, path, interval_seconds, params FROM checks WHERE domain_id = ?", domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.Check
	for rows.Next() {
		var (
			c          models.Check
			paramsJSON string
		)
		if err := rows.Scan(&c.ID, &c.DomainID, &c.Type, &c.Frequency, &c.Path, &c.IntervalSeconds, &paramsJSON); err != nil {
			return nil, err
		}
		c.Params = parseParams(paramsJSON)
		if c.Params.Path == "" && c.Path != "" {
			c.Params.Path = c.Path
		}
		checks = append(checks, c)
	}
	return checks, nil
}

func (r *CheckRepo) GetAll() ([]models.Check, error) {
	rows, err := r.db.Query("SELECT id, domain_id, type, frequency, path, interval_seconds, params FROM checks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.Check
	for rows.Next() {
		var (
			c          models.Check
			paramsJSON string
		)
		if err := rows.Scan(&c.ID, &c.DomainID, &c.Type, &c.Frequency, &c.Path, &c.IntervalSeconds, &paramsJSON); err != nil {
			return nil, err
		}
		c.Params = parseParams(paramsJSON)
		if c.Params.Path == "" && c.Path != "" {
			c.Params.Path = c.Path
		}
		checks = append(checks, c)
	}
	return checks, nil
}

func parseParams(raw string) models.CheckParams {
	if raw == "" {
		return models.CheckParams{}
	}
	var params models.CheckParams
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return models.CheckParams{}
	}
	return params
}
