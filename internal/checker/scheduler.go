package checker

import (
	"log"
	"sync"
	"time"

	"github.com/MimoJanra/DomainPulse/internal/models"
	"github.com/MimoJanra/DomainPulse/internal/storage"
)

type Scheduler struct {
	checkRepo      *storage.CheckRepo
	domainRepo     *storage.SQLiteDomainRepo
	resultRepo     *storage.ResultRepo
	workerPool     *WorkerPool
	tickers        map[int]*time.Ticker
	realtimeLoops  map[int]chan struct{}
	rateLimiters   map[int]*RateLimiter  
	stopChan       chan struct{}
	mu             sync.RWMutex
	running        bool
}

func NewScheduler(
	checkRepo *storage.CheckRepo,
	domainRepo *storage.SQLiteDomainRepo,
	resultRepo *storage.ResultRepo,
	workerCount int,
) *Scheduler {
	workerPool := NewWorkerPool(workerCount, domainRepo, resultRepo)
	workerPool.Start()

	return &Scheduler{
		checkRepo:     checkRepo,
		domainRepo:    domainRepo,
		resultRepo:    resultRepo,
		workerPool:    workerPool,
		tickers:       make(map[int]*time.Ticker),
		realtimeLoops: make(map[int]chan struct{}),
		rateLimiters:  make(map[int]*RateLimiter),
		stopChan:      make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	log.Println("Scheduler started")

	s.loadAndScheduleChecks()

	go s.watchForChanges()
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChan)

	for id, ticker := range s.tickers {
		ticker.Stop()
		delete(s.tickers, id)
	}

	for id, stopChan := range s.realtimeLoops {
		close(stopChan)
		delete(s.realtimeLoops, id)
	}

	s.workerPool.Stop()

	log.Println("Scheduler stopped")
}

func (s *Scheduler) SetWorkerCount(count int) {
	s.workerPool.SetWorkers(count)
	log.Printf("Worker count set to %d", count)
}

func (s *Scheduler) loadAndScheduleChecks() {
	checks, err := s.checkRepo.GetAll(nil)
	if err != nil {
		log.Printf("failed to load checks: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, check := range checks {
		if check.Enabled {
			s.scheduleCheck(check)
		}
	}
}

func (s *Scheduler) scheduleCheck(check models.Check) {
	if ticker, exists := s.tickers[check.ID]; exists {
		ticker.Stop()
		delete(s.tickers, check.ID)
	}
	if stopChan, exists := s.realtimeLoops[check.ID]; exists {
		close(stopChan)
		delete(s.realtimeLoops, check.ID)
	}

	if check.RealtimeMode && check.RateLimitPerMinute > 0 {
		minIntervalMS := 0
		if check.RateLimitPerMinute > 0 {
			minIntervalMS = (60 * 1000) / check.RateLimitPerMinute
		}
		s.rateLimiters[check.ID] = NewRateLimiter(check.RateLimitPerMinute, minIntervalMS)
	}

	if check.RealtimeMode {
		stopChan := make(chan struct{})
		s.realtimeLoops[check.ID] = stopChan
		
		go s.runRealtimeLoop(check, stopChan)
	} else {
		interval := time.Duration(check.IntervalSeconds) * time.Second
		ticker := time.NewTicker(interval)
		s.tickers[check.ID] = ticker

		go s.runCheck(check)

		go func(c models.Check, t *time.Ticker) {
			for {
				select {
				case <-t.C:
					s.runCheck(c)
				case <-s.stopChan:
					return
				}
			}
		}(check, ticker)
	}
}

func (s *Scheduler) runRealtimeLoop(check models.Check, stopChan chan struct{}) {
	for {
		select {
		case <-stopChan:
			return
		case <-s.stopChan:
			return
		default:
			s.runRealtimeCheck(check, stopChan)
		}
	}
}

func (s *Scheduler) runRealtimeCheck(check models.Check, stopChan chan struct{}) {
	if GlobalRateLimiter != nil {
		GlobalRateLimiter.Wait()
	}

	s.mu.RLock()
	limiter, hasLimiter := s.rateLimiters[check.ID]
	s.mu.RUnlock()

	if hasLimiter {
		limiter.Wait()
	} else if check.RateLimitPerMinute > 0 {
		minIntervalMS := 0
		if check.RateLimitPerMinute > 0 {
			minIntervalMS = (60 * 1000) / check.RateLimitPerMinute
		}
		s.mu.Lock()
		s.rateLimiters[check.ID] = NewRateLimiter(check.RateLimitPerMinute, minIntervalMS)
		limiter = s.rateLimiters[check.ID]
		s.mu.Unlock()
		limiter.Wait()
	}

	s.runCheck(check)
}

func (s *Scheduler) runCheck(check models.Check) {
	domain, err := s.domainRepo.GetByID(check.DomainID)
	if err != nil {
		log.Printf("domain not found for check %d: %v", check.ID, err)
		return
	}

	job := CheckJob{
		Check:  check,
		Domain: domain,
	}
	s.workerPool.Submit(job)
}

func (s *Scheduler) watchForChanges() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.updateSchedule()
		case <-s.stopChan:
			return
		}
	}
}

func (s *Scheduler) updateSchedule() {
	checks, err := s.checkRepo.GetAll(nil)
	if err != nil {
		log.Printf("failed to reload checks: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	currentCheckIDs := make(map[int]bool)
	for _, check := range checks {
		currentCheckIDs[check.ID] = true

		if check.Enabled {
			needsUpdate := false
			if _, exists := s.tickers[check.ID]; exists {
				if check.RealtimeMode {
					needsUpdate = true
				}
			} else if _, realtimeExists := s.realtimeLoops[check.ID]; realtimeExists {
				if !check.RealtimeMode {
					needsUpdate = true
				}
			} else {
				needsUpdate = true
			}

			if needsUpdate {
				s.scheduleCheck(check)
			}
		} else {
			if ticker, exists := s.tickers[check.ID]; exists {
				ticker.Stop()
				delete(s.tickers, check.ID)
			}
			if stopChan, exists := s.realtimeLoops[check.ID]; exists {
				close(stopChan)
				delete(s.realtimeLoops, check.ID)
			}
		}
	}

	for id, ticker := range s.tickers {
		if !currentCheckIDs[id] {
			ticker.Stop()
			delete(s.tickers, id)
		}
	}
}
