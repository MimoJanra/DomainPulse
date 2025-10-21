package storage

import (
	"DomainPulse/internal/models"
	"sync"
)

type DomainRepo struct {
	mu      sync.Mutex
	domains []models.Domain
	nextID  int
}

func NewDomainRepo() *DomainRepo {
	return &DomainRepo{
		domains: make([]models.Domain, 0),
		nextID:  1,
	}
}

func (r *DomainRepo) GetAll() []models.Domain {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.domains
}

func (r *DomainRepo) Add(name string) models.Domain {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := models.Domain{ID: r.nextID, Name: name}
	r.nextID++
	r.domains = append(r.domains, d)
	return d
}

func (r *DomainRepo) DeleteByID(id int) (models.Domain, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, d := range r.domains {
		if d.ID == id {
			deleted := d
			r.domains = append(r.domains[:i], r.domains[i+1:]...)
			return deleted, true
		}
	}

	return models.Domain{}, false
}
