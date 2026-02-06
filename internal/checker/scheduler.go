package checker

import (
	"log"
	"sync"
	"time"

	"github.com/MimoJanra/DomainPulse/internal/models"
	"github.com/MimoJanra/DomainPulse/internal/storage"
)

type Scheduler struct {
	checkRepo        *storage.CheckRepo
	domainRepo       *storage.SQLiteDomainRepo
	resultRepo       *storage.ResultRepo
	notificationRepo *storage.NotificationRepo
	workerPool       *WorkerPool
	tickers          map[int]*time.Ticker
	realtimeLoops    map[int]chan struct{}
	tlsLoops         map[int]chan struct{}
	rateLimiters     map[int]*RateLimiter
	stopChan         chan struct{}
	mu               sync.RWMutex
	running          bool
}

func NewScheduler(
	checkRepo *storage.CheckRepo,
	domainRepo *storage.SQLiteDomainRepo,
	resultRepo *storage.ResultRepo,
	notificationRepo *storage.NotificationRepo,
	workerCount int,
) *Scheduler {
	workerPool := NewWorkerPool(workerCount, domainRepo, resultRepo, notificationRepo)
	workerPool.Start()

	return &Scheduler{
		checkRepo:        checkRepo,
		domainRepo:       domainRepo,
		resultRepo:       resultRepo,
		notificationRepo: notificationRepo,
		workerPool:       workerPool,
		tickers:          make(map[int]*time.Ticker),
		realtimeLoops:    make(map[int]chan struct{}),
		tlsLoops:         make(map[int]chan struct{}),
		rateLimiters:     make(map[int]*RateLimiter),
		stopChan:         make(chan struct{}),
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
	for id, ch := range s.tlsLoops {
		close(ch)
		delete(s.tlsLoops, id)
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
	if ch, exists := s.tlsLoops[check.ID]; exists {
		close(ch)
		delete(s.tlsLoops, check.ID)
	}

	if check.Type == "tls" {
		domain, err := s.domainRepo.GetByID(check.DomainID)
		if err != nil {
			log.Printf("domain not found for TLS check %d: %v", check.ID, err)
			return
		}
		if check.Params.Port <= 0 {
			log.Printf("invalid port for TLS check %d", check.ID)
			return
		}
		stopChan := make(chan struct{})
		s.tlsLoops[check.ID] = stopChan
		timeout := 10 * time.Second
		if check.Params.TimeoutMS > 0 {
			timeout = time.Duration(check.Params.TimeoutMS) * time.Millisecond
		}
		job := CheckJob{Check: check, Domain: domain}
		go s.runTLSPersistentLoop(job, timeout, stopChan)
		return
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
			s.runRealtimeCheck(check)
		}
	}
}

func (s *Scheduler) runRealtimeCheck(check models.Check) {
	s.waitForGlobalRateLimit()
	s.waitForCheckRateLimit(check)
	s.runCheck(check)
}

func (s *Scheduler) runTLSPersistentLoop(job CheckJob, timeout time.Duration, stopChan chan struct{}) {
	onEvent := func(result CheckResult) {
		s.workerPool.SubmitTLSEvent(job, result)
	}
	RunTLSPersistentLoop(job.Domain.Name, job.Check.Params.Port, timeout, onEvent, stopChan)
}

func (s *Scheduler) waitForGlobalRateLimit() {
	if GlobalRateLimiter != nil {
		GlobalRateLimiter.Wait()
	}
}

func (s *Scheduler) waitForCheckRateLimit(check models.Check) {
	limiter := s.getOrCreateRateLimiter(check)
	if limiter != nil {
		limiter.Wait()
	}
}

func (s *Scheduler) getOrCreateRateLimiter(check models.Check) *RateLimiter {
	s.mu.RLock()
	limiter, hasLimiter := s.rateLimiters[check.ID]
	s.mu.RUnlock()

	if hasLimiter {
		return limiter
	}

	if check.RateLimitPerMinute <= 0 {
		return nil
	}

	return s.createRateLimiterForCheck(check)
}

func (s *Scheduler) createRateLimiterForCheck(check models.Check) *RateLimiter {
	minIntervalMS := (60 * 1000) / check.RateLimitPerMinute
	limiter := NewRateLimiter(check.RateLimitPerMinute, minIntervalMS)

	s.mu.Lock()
	s.rateLimiters[check.ID] = limiter
	s.mu.Unlock()

	return limiter
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

	currentCheckIDs := s.processChecks(checks)
	s.cleanupRemovedChecks(currentCheckIDs)
}

func (s *Scheduler) processChecks(checks []models.Check) map[int]bool {
	currentCheckIDs := make(map[int]bool)

	for _, check := range checks {
		currentCheckIDs[check.ID] = true

		if check.Enabled {
			if s.checkNeedsUpdate(check) {
				s.scheduleCheck(check)
			}
		} else {
			s.unscheduleCheck(check.ID)
		}
	}

	return currentCheckIDs
}

func (s *Scheduler) checkNeedsUpdate(check models.Check) bool {
	hasTicker := s.tickers[check.ID] != nil
	hasRealtimeLoop := s.realtimeLoops[check.ID] != nil
	hasTLSLoop := s.tlsLoops[check.ID] != nil

	if check.Type == "tls" {
		return !hasTLSLoop
	}
	if hasTLSLoop {
		return true
	}
	if hasTicker && check.RealtimeMode {
		return true
	}
	if hasRealtimeLoop && !check.RealtimeMode {
		return true
	}
	return !hasTicker && !hasRealtimeLoop
}

func (s *Scheduler) unscheduleCheck(checkID int) {
	if ticker, exists := s.tickers[checkID]; exists {
		ticker.Stop()
		delete(s.tickers, checkID)
	}
	if stopChan, exists := s.realtimeLoops[checkID]; exists {
		close(stopChan)
		delete(s.realtimeLoops, checkID)
	}
	if ch, exists := s.tlsLoops[checkID]; exists {
		close(ch)
		delete(s.tlsLoops, checkID)
	}
}

func (s *Scheduler) cleanupRemovedChecks(currentCheckIDs map[int]bool) {
	for id, ticker := range s.tickers {
		if !currentCheckIDs[id] {
			ticker.Stop()
			delete(s.tickers, id)
		}
	}
	for id, stopChan := range s.realtimeLoops {
		if !currentCheckIDs[id] {
			close(stopChan)
			delete(s.realtimeLoops, id)
		}
	}
	for id, ch := range s.tlsLoops {
		if !currentCheckIDs[id] {
			close(ch)
			delete(s.tlsLoops, id)
		}
	}
}
