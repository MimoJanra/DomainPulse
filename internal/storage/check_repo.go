package storage

import (
	"database/sql"

	"github.com/MimoJanra/DomainPulse/internal/models"
)

type CheckRepo struct {
	db *sql.DB
}

func NewCheckRepo(db *sql.DB) *CheckRepo { return &CheckRepo{db: db} }

func (r *CheckRepo) Add(domainID int, checkType, frequency, path string) (models.Check, error) {
	res, err := r.db.Exec(
		"INSERT INTO checks(domain_id, type, frequency, path) VALUES(?, ?, ?, ?)",
		domainID, checkType, frequency, path,
	)
	if err != nil {
		return models.Check{}, err
	}
	id, _ := res.LastInsertId()
	return models.Check{
		ID:        int(id),
		DomainID:  domainID,
		Type:      checkType,
		Frequency: frequency,
		Path:      path,
	}, nil
}

func (r *CheckRepo) GetByDomainID(domainID int) ([]models.Check, error) {
	rows, err := r.db.Query("SELECT id, domain_id, type, frequency, path FROM checks WHERE domain_id = ?", domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.Check
	for rows.Next() {
		var c models.Check
		if err := rows.Scan(&c.ID, &c.DomainID, &c.Type, &c.Frequency, &c.Path); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, nil
}

func (r *CheckRepo) GetAll() ([]models.Check, error) {
	rows, err := r.db.Query("SELECT id, domain_id, type, frequency, path FROM checks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.Check
	for rows.Next() {
		var c models.Check
		if err := rows.Scan(&c.ID, &c.DomainID, &c.Type, &c.Frequency, &c.Path); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, nil
}
