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

func (r *CheckRepo) Add(domainID int, checkType string, intervalSeconds int, params models.CheckParams, enabled bool) (models.Check, error) {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return models.Check{}, fmt.Errorf("marshal params: %w", err)
	}

	frequency := fmt.Sprintf("%ds", intervalSeconds)
	path := params.Path
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	res, err := r.db.Exec(
		"INSERT INTO checks(domain_id, type, frequency, path, interval_seconds, params, enabled) VALUES(?, ?, ?, ?, ?, ?, ?)",
		domainID, checkType, frequency, path, intervalSeconds, string(paramsJSON), enabledInt,
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
		Enabled:         enabled,
	}, nil
}

func (r *CheckRepo) GetByDomainID(domainID int) ([]models.Check, error) {
	rows, err := r.db.Query("SELECT id, domain_id, type, frequency, path, interval_seconds, params, enabled FROM checks WHERE domain_id = ?", domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.Check
	for rows.Next() {
		var (
			c          models.Check
			paramsJSON string
			enabledInt int
		)
		if err := rows.Scan(&c.ID, &c.DomainID, &c.Type, &c.Frequency, &c.Path, &c.IntervalSeconds, &paramsJSON, &enabledInt); err != nil {
			return nil, err
		}
		c.Params = parseParams(paramsJSON)
		c.Enabled = enabledInt == 1
		if c.Params.Path == "" && c.Path != "" {
			c.Params.Path = c.Path
		}
		checks = append(checks, c)
	}
	return checks, nil
}

func (r *CheckRepo) GetAll(domainID *int) ([]models.Check, error) {
	var rows *sql.Rows
	var err error

	if domainID != nil {
		rows, err = r.db.Query("SELECT id, domain_id, type, frequency, path, interval_seconds, params, enabled FROM checks WHERE domain_id = ?", *domainID)
	} else {
		rows, err = r.db.Query("SELECT id, domain_id, type, frequency, path, interval_seconds, params, enabled FROM checks")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.Check
	for rows.Next() {
		var (
			c          models.Check
			paramsJSON string
			enabledInt int
		)
		if err := rows.Scan(&c.ID, &c.DomainID, &c.Type, &c.Frequency, &c.Path, &c.IntervalSeconds, &paramsJSON, &enabledInt); err != nil {
			return nil, err
		}
		c.Params = parseParams(paramsJSON)
		c.Enabled = enabledInt == 1
		if c.Params.Path == "" && c.Path != "" {
			c.Params.Path = c.Path
		}
		checks = append(checks, c)
	}
	return checks, nil
}

func (r *CheckRepo) GetByID(id int) (models.Check, error) {
	row := r.db.QueryRow("SELECT id, domain_id, type, frequency, path, interval_seconds, params, enabled FROM checks WHERE id = ?", id)
	var (
		c          models.Check
		paramsJSON string
		enabledInt int
	)
	err := row.Scan(&c.ID, &c.DomainID, &c.Type, &c.Frequency, &c.Path, &c.IntervalSeconds, &paramsJSON, &enabledInt)
	if err != nil {
		return models.Check{}, err
	}
	c.Params = parseParams(paramsJSON)
	c.Enabled = enabledInt == 1
	if c.Params.Path == "" && c.Path != "" {
		c.Params.Path = c.Path
	}
	return c, nil
}

func (r *CheckRepo) Update(id int, checkType string, intervalSeconds int, params models.CheckParams) (models.Check, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return models.Check{}, fmt.Errorf("marshal params: %w", err)
	}

	frequency := fmt.Sprintf("%ds", intervalSeconds)
	path := params.Path

	_, err = r.db.Exec(
		"UPDATE checks SET type = ?, frequency = ?, path = ?, interval_seconds = ?, params = ? WHERE id = ?",
		checkType, frequency, path, intervalSeconds, string(paramsJSON), id,
	)
	if err != nil {
		return models.Check{}, err
	}

	return r.GetByID(id)
}

func (r *CheckRepo) SetEnabled(id int, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := r.db.Exec("UPDATE checks SET enabled = ? WHERE id = ?", enabledInt, id)
	return err
}

func (r *CheckRepo) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM checks WHERE id = ?", id)
	return err
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
