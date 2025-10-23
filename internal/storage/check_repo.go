package storage

import (
	"DomainPulse/internal/models"
	"database/sql"
)

type CheckRepo struct {
	db *sql.DB
}

func NewCheckRepo(db *sql.DB) *CheckRepo { return &CheckRepo{db: db} }

func (r *CheckRepo) Add(domainID int, checkType, frequency, url string) (models.Check, error) {
	res, err := r.db.Exec("INSERT INTO checks(domain_id, type, frequency, url) VALUES(?, ?, ?, ?)",
		domainID, checkType, frequency, url)
	if err != nil {
		return models.Check{}, err
	}
	id, _ := res.LastInsertId()
	return models.Check{ID: int(id), DomainID: domainID, Type: checkType, Frequency: frequency, URL: url}, nil
}

func (r *CheckRepo) GetByDomainID(domainID int) ([]models.Check, error) {
	rows, err := r.db.Query("SELECT id, domain_id, type, frequency, url FROM checks WHERE domain_id = ?", domainID)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	var checks []models.Check
	for rows.Next() {
		var c models.Check
		if err := rows.Scan(&c.ID, &c.DomainID, &c.Type, &c.Frequency, &c.URL); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, nil
}

func (r *CheckRepo) GetAll() ([]models.Check, error) {
	rows, err := r.db.Query("SELECT id, domain_id, type, frequency, url FROM checks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.Check
	for rows.Next() {
		var c models.Check
		if err := rows.Scan(&c.ID, &c.DomainID, &c.Type, &c.Frequency, &c.URL); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, nil
}
