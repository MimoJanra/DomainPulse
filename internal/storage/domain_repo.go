package storage

import (
	"DomainPulse/internal/models"
	"database/sql"
)

type SQLiteDomainRepo struct {
	db *sql.DB
}

func NewSQLiteDomainRepo(db *sql.DB) *SQLiteDomainRepo {
	return &SQLiteDomainRepo{db: db}
}

func (r *SQLiteDomainRepo) GetAll() ([]models.Domain, error) {
	rows, err := r.db.Query("SELECT id, name FROM domains")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []models.Domain
	for rows.Next() {
		var d models.Domain
		if err := rows.Scan(&d.ID, &d.Name); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}
	return domains, nil
}

func (r *SQLiteDomainRepo) Add(name string) (models.Domain, error) {
	res, err := r.db.Exec("INSERT INTO domains (name) VALUES (?)", name)
	if err != nil {
		return models.Domain{}, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return models.Domain{}, err
	}

	return models.Domain{ID: int(id), Name: name}, nil
}

func (r *SQLiteDomainRepo) DeleteByID(id int) (bool, error) {
	res, err := r.db.Exec("DELETE FROM domains WHERE id = ?", id)
	if err != nil {
		return false, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows > 0, nil
}
