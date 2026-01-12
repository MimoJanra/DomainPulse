package checker

import (
	"log"
	"sync"
	"time"

	"github.com/MimoJanra/DomainPulse/internal/models"
	"github.com/MimoJanra/DomainPulse/internal/storage"
)

type WorkerPool struct {
	workers    int
	jobQueue   chan CheckJob
	wg         sync.WaitGroup
	stopChan   chan struct{}
	domainRepo *storage.SQLiteDomainRepo
	resultRepo *storage.ResultRepo
}

type CheckJob struct {
	Check  models.Check
	Domain models.Domain
}

func NewWorkerPool(workers int, domainRepo *storage.SQLiteDomainRepo, resultRepo *storage.ResultRepo) *WorkerPool {
	return &WorkerPool{
		workers:    workers,
		jobQueue:   make(chan CheckJob, 100),
		stopChan:   make(chan struct{}),
		domainRepo: domainRepo,
		resultRepo: resultRepo,
	}
}

func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

func (wp *WorkerPool) Stop() {
	close(wp.stopChan)
	close(wp.jobQueue)
	wp.wg.Wait()
}

func (wp *WorkerPool) Submit(job CheckJob) {
	select {
	case wp.jobQueue <- job:
	case <-wp.stopChan:
		return
	default:
		log.Printf("worker pool queue full, dropping job for check %d", job.Check.ID)
	}
}

func (wp *WorkerPool) SetWorkers(count int) {
	if count < 1 {
		count = 1
	}

	if count > wp.workers {
		for i := wp.workers; i < count; i++ {
			wp.wg.Add(1)
			go wp.worker(i)
		}
		wp.workers = count
	} else if count < wp.workers {
		wp.workers = count
	}
}

func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	for {
		select {
		case <-wp.stopChan:
			return
		case job, ok := <-wp.jobQueue:
			if !ok {
				return
			}
			wp.executeCheck(job)
		}
	}
}

func (wp *WorkerPool) executeCheck(job CheckJob) {
	var result CheckResult
	timeout := 10 * time.Second

	if job.Check.Params.TimeoutMS > 0 {
		timeout = time.Duration(job.Check.Params.TimeoutMS) * time.Millisecond
	}

	switch job.Check.Type {
	case "http":
		path := job.Check.Params.Path
		if path == "" {
			path = "/"
		}
		fullURL := "https://" + job.Domain.Name
		if len(path) > 0 && path[0] != '/' {
			fullURL += "/"
		}
		fullURL += path
		result = RunHTTPCheck(fullURL, timeout)

	case "icmp":
		result = RunICMPCheck(job.Domain.Name, timeout)

	case "tcp":
		port := job.Check.Params.Port
		if port <= 0 {
			log.Printf("invalid port for TCP check %d", job.Check.ID)
			return
		}
		result = RunTCPCheck(job.Domain.Name, port, timeout)

	case "udp":
		port := job.Check.Params.Port
		if port <= 0 {
			log.Printf("invalid port for UDP check %d", job.Check.ID)
			return
		}
		payload := job.Check.Params.Payload
		result = RunUDPCheck(job.Domain.Name, port, payload, timeout)

	default:
		log.Printf("unsupported check type: %s for check %d", job.Check.Type, job.Check.ID)
		return
	}

	res := models.Result{
		CheckID:      job.Check.ID,
		Status:       result.Status,
		StatusCode:   result.StatusCode,
		DurationMS:   result.DurationMS,
		Outcome:      result.Outcome,
		ErrorMessage: result.ErrorMessage,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}

	if err := wp.resultRepo.Add(res); err != nil {
		log.Printf("failed to save result for check %d: %v", job.Check.ID, err)
	}
}
