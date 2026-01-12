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
	return r.AddWithRealtime(domainID, checkType, intervalSeconds, params, enabled, false, 0)
}

func (r *CheckRepo) AddWithRealtime(domainID int, checkType string, intervalSeconds int, params models.CheckParams, enabled bool, realtimeMode bool, rateLimitPerMinute int) (models.Check, error) {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return models.Check{}, fmt.Errorf("marshal params: %w", err)
	}

	path := params.Path
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	realtimeInt := 0
	if realtimeMode {
		realtimeInt = 1
	}

	res, err := r.db.Exec(
		"INSERT INTO checks(domain_id, type, path, interval_seconds, params, enabled, realtime_mode, rate_limit_per_minute) VALUES(?, ?, ?, ?, ?, ?, ?, ?)",
		domainID, checkType, path, intervalSeconds, string(paramsJSON), enabledInt, realtimeInt, rateLimitPerMinute,
	)
	if err != nil {
		return models.Check{}, err
	}
	id, _ := res.LastInsertId()
	return models.Check{
		ID:                int(id),
		DomainID:          domainID,
		Type:              checkType,
		Path:              path,
		IntervalSeconds:   intervalSeconds,
		Params:            params,
		Enabled:           enabled,
		RealtimeMode:      realtimeMode,
		RateLimitPerMinute: rateLimitPerMinute,
	}, nil
}

func (r *CheckRepo) GetByDomainID(domainID int) ([]models.Check, error) {
	rows, err := r.db.Query("SELECT id, domain_id, type, path, interval_seconds, params, enabled, realtime_mode, rate_limit_per_minute FROM checks WHERE domain_id = ?", domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.Check
	for rows.Next() {
		var (
			c                models.Check
			paramsJSON       string
			enabledInt       int
			realtimeInt      int
			rateLimitPerMin  int
		)
		if err := rows.Scan(&c.ID, &c.DomainID, &c.Type, &c.Path, &c.IntervalSeconds, &paramsJSON, &enabledInt, &realtimeInt, &rateLimitPerMin); err != nil {
			return nil, err
		}
		c.Params = parseParams(paramsJSON)
		c.Enabled = enabledInt == 1
		c.RealtimeMode = realtimeInt == 1
		c.RateLimitPerMinute = rateLimitPerMin
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
		rows, err = r.db.Query("SELECT id, domain_id, type, path, interval_seconds, params, enabled, realtime_mode, rate_limit_per_minute FROM checks WHERE domain_id = ?", *domainID)
	} else {
		rows, err = r.db.Query("SELECT id, domain_id, type, path, interval_seconds, params, enabled, realtime_mode, rate_limit_per_minute FROM checks")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.Check
	for rows.Next() {
		var (
			c                models.Check
			paramsJSON       string
			enabledInt       int
			realtimeInt      int
			rateLimitPerMin  int
		)
		if err := rows.Scan(&c.ID, &c.DomainID, &c.Type, &c.Path, &c.IntervalSeconds, &paramsJSON, &enabledInt, &realtimeInt, &rateLimitPerMin); err != nil {
			return nil, err
		}
		c.Params = parseParams(paramsJSON)
		c.Enabled = enabledInt == 1
		c.RealtimeMode = realtimeInt == 1
		c.RateLimitPerMinute = rateLimitPerMin
		if c.Params.Path == "" && c.Path != "" {
			c.Params.Path = c.Path
		}
		checks = append(checks, c)
	}
	return checks, nil
}

func (r *CheckRepo) GetByID(id int) (models.Check, error) {
	row := r.db.QueryRow("SELECT id, domain_id, type, path, interval_seconds, params, enabled, realtime_mode, rate_limit_per_minute FROM checks WHERE id = ?", id)
	var (
		c                models.Check
		paramsJSON       string
		enabledInt       int
		realtimeInt      int
		rateLimitPerMin  int
	)
	err := row.Scan(&c.ID, &c.DomainID, &c.Type, &c.Path, &c.IntervalSeconds, &paramsJSON, &enabledInt, &realtimeInt, &rateLimitPerMin)
	if err != nil {
		return models.Check{}, err
	}
	c.Params = parseParams(paramsJSON)
	c.Enabled = enabledInt == 1
	c.RealtimeMode = realtimeInt == 1
	c.RateLimitPerMinute = rateLimitPerMin
	if c.Params.Path == "" && c.Path != "" {
		c.Params.Path = c.Path
	}
	return c, nil
}

func (r *CheckRepo) Update(id int, checkType string, intervalSeconds int, params models.CheckParams) (models.Check, error) {
	return r.UpdateWithRealtime(id, checkType, intervalSeconds, params, false, 0)
}

func (r *CheckRepo) UpdateWithRealtime(id int, checkType string, intervalSeconds int, params models.CheckParams, realtimeMode bool, rateLimitPerMinute int) (models.Check, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return models.Check{}, fmt.Errorf("marshal params: %w", err)
	}

	path := params.Path
	realtimeInt := 0
	if realtimeMode {
		realtimeInt = 1
	}

	_, err = r.db.Exec(
		"UPDATE checks SET type = ?, path = ?, interval_seconds = ?, params = ?, realtime_mode = ?, rate_limit_per_minute = ? WHERE id = ?",
		checkType, path, intervalSeconds, string(paramsJSON), realtimeInt, rateLimitPerMinute, id,
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
