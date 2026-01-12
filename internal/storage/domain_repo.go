package storage

import (
	"database/sql"
	"log"

	"github.com/MimoJanra/DomainPulse/internal/models"
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
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("%v", err)
		}
	}(rows)

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

func (r *SQLiteDomainRepo) GetByID(id int) (models.Domain, error) {
	row := r.db.QueryRow("SELECT id, name FROM domains WHERE id = ?", id)
	var d models.Domain
	err := row.Scan(&d.ID, &d.Name)
	return d, err
}

func (r *SQLiteDomainRepo) Add(name string) (models.Domain, error) {
	res, err := r.db.Exec("INSERT INTO domains (name) VALUES (?)", name)
	if err != nil {
		return models.Domain{}, err
	}
	id, _ := res.LastInsertId()
	return models.Domain{ID: int(id), Name: name}, nil
}

func (r *SQLiteDomainRepo) DeleteByID(id int) (bool, error) {
	res, err := r.db.Exec("DELETE FROM domains WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}
