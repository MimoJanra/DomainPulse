package checker

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/MimoJanra/DomainPulse/internal/models"
	"github.com/MimoJanra/DomainPulse/internal/notifications"
	"github.com/MimoJanra/DomainPulse/internal/storage"
)

type TLSEvent struct {
	Job    CheckJob
	Result CheckResult
}

type WorkerPool struct {
	workers          int
	jobQueue         chan CheckJob
	tlsEventChan     chan TLSEvent
	wg               sync.WaitGroup
	stopChan         chan struct{}
	domainRepo       *storage.SQLiteDomainRepo
	resultRepo       *storage.ResultRepo
	notificationRepo *storage.NotificationRepo
	checkMetrics     map[int]*CheckMetrics
	metricsMu        sync.RWMutex
}

type CheckMetrics struct {
	mu              sync.Mutex
	errorCount      int
	lastErrorTime   time.Time
	averageDuration time.Duration
	sampleCount     int
	lastCheckTime   time.Time
}

type CheckJob struct {
	Check  models.Check
	Domain models.Domain
}

func NewWorkerPool(workers int, domainRepo *storage.SQLiteDomainRepo, resultRepo *storage.ResultRepo, notificationRepo *storage.NotificationRepo) *WorkerPool {
	return &WorkerPool{
		workers:          workers,
		jobQueue:         make(chan CheckJob, 100),
		tlsEventChan:     make(chan TLSEvent, 50),
		stopChan:         make(chan struct{}),
		domainRepo:       domainRepo,
		resultRepo:       resultRepo,
		notificationRepo: notificationRepo,
		checkMetrics:     make(map[int]*CheckMetrics),
	}
}

func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
	wp.wg.Add(1)
	go wp.tlsEventProcessor()
}

func (wp *WorkerPool) Stop() {
	close(wp.stopChan)
	close(wp.tlsEventChan)
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

func (wp *WorkerPool) SubmitTLSEvent(job CheckJob, result CheckResult) {
	select {
	case wp.tlsEventChan <- TLSEvent{Job: job, Result: result}:
	case <-wp.stopChan:
	default:
		log.Printf("tls event queue full, dropping event for check %d", job.Check.ID)
	}
}

func (wp *WorkerPool) tlsEventProcessor() {
	defer wp.wg.Done()
	for ev := range wp.tlsEventChan {
		wp.saveResult(ev.Job, ev.Result, 0)
		wp.sendNotifications(ev.Job, ev.Result, time.Now().Format(time.RFC3339))
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
	startTime := time.Now()
	timeout := wp.getTimeout(job.Check)

	result := wp.runCheckByType(job, timeout)
	if result == nil {
		return
	}

	wp.saveResult(job, *result, time.Since(startTime))
}

func (wp *WorkerPool) getTimeout(check models.Check) time.Duration {
	timeout := 10 * time.Second
	if check.Params.TimeoutMS > 0 {
		timeout = time.Duration(check.Params.TimeoutMS) * time.Millisecond
	}
	return timeout
}

func (wp *WorkerPool) runCheckByType(job CheckJob, timeout time.Duration) *CheckResult {
	switch job.Check.Type {
	case "http":
		result := wp.runHTTPCheck(job, timeout)
		return &result
	case "icmp":
		result := RunICMPCheck(job.Domain.Name, timeout)
		return &result
	case "tcp":
		return wp.runTCPCheck(job, timeout)
	case "udp":
		return wp.runUDPCheck(job, timeout)
	case "tls":
		return wp.runTLSCheck(job, timeout)
	default:
		log.Printf("unsupported check type: %s for check %d", job.Check.Type, job.Check.ID)
		return nil
	}
}

func (wp *WorkerPool) runHTTPCheck(job CheckJob, timeout time.Duration) CheckResult {
	fullURL := buildHTTPURL(job.Domain.Name, job.Check.Params)
	method := normalizeHTTPMethod(job.Check.Params.Method)
	return RunHTTPCheckWithMethodAndHeaders(fullURL, method, job.Check.Params.Body, job.Check.Params.Headers, timeout)
}

func buildHTTPURL(domainName string, params models.CheckParams) string {
	path := params.Path
	if path == "" {
		path = "/"
	}
	scheme := params.Scheme
	if scheme == "" {
		scheme = "https"
	}
	host := domainName
	if ip := net.ParseIP(domainName); ip != nil && ip.To4() == nil {
		host = "[" + domainName + "]"
	}
	fullURL := scheme + "://" + host
	if len(path) > 0 && path[0] != '/' {
		fullURL += "/"
	}
	fullURL += path
	return fullURL
}

func normalizeHTTPMethod(method string) string {
	if method == "" {
		return "GET"
	}
	return method
}

func (wp *WorkerPool) runTCPCheck(job CheckJob, timeout time.Duration) *CheckResult {
	port := job.Check.Params.Port
	if port <= 0 {
		log.Printf("invalid port for TCP check %d", job.Check.ID)
		return nil
	}
	result := RunTCPCheckWithPayload(job.Domain.Name, port, job.Check.Params.Payload, timeout)
	return &result
}

func (wp *WorkerPool) runUDPCheck(job CheckJob, timeout time.Duration) *CheckResult {
	port := job.Check.Params.Port
	if port <= 0 {
		log.Printf("invalid port for UDP check %d", job.Check.ID)
		return nil
	}
	result := RunUDPCheck(job.Domain.Name, port, job.Check.Params.Payload, timeout)
	return &result
}

func (wp *WorkerPool) runTLSCheck(job CheckJob, timeout time.Duration) *CheckResult {
	port := job.Check.Params.Port
	if port <= 0 {
		log.Printf("invalid port for TLS check %d", job.Check.ID)
		return nil
	}
	result := RunTLSCheck(job.Domain.Name, port, timeout)
	return &result
}

func (wp *WorkerPool) saveResult(job CheckJob, result CheckResult, duration time.Duration) {
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

	isError := result.Status == "error" || result.Status == "timeout"
	wp.updateMetrics(job.Check.ID, duration, isError)

	wp.sendNotifications(job, result, res.CreatedAt)
}

func (wp *WorkerPool) sendNotifications(job CheckJob, result CheckResult, createdAt string) {
	settingsList, err := wp.notificationRepo.GetEnabled()
	if err != nil {
		log.Printf("failed to get notification settings: %v", err)
		return
	}

	if len(settingsList) == 0 {
		return
	}

	isFailure := result.Status == "error" || result.Status == "timeout"

	msg := notifications.NotificationMessage{
		CheckID:      job.Check.ID,
		DomainName:   job.Domain.Name,
		CheckType:    job.Check.Type,
		Status:       result.Status,
		ErrorMessage: result.ErrorMessage,
		DurationMS:   result.DurationMS,
		CreatedAt:    createdAt,
	}

	sender := notifications.NewNotificationSender()

	for _, settings := range settingsList {
		shouldNotify := false
		notifySlow := false

		if isFailure && settings.NotifyOnFailure {
			shouldNotify = true
		}
		if !isFailure && settings.NotifyOnSuccess {
			shouldNotify = true
		}

		if settings.NotifyOnSlowResponse && settings.SlowResponseThreshold > 0 {
			if result.DurationMS >= settings.SlowResponseThreshold {
				notifySlow = true
			}
		}

		if shouldNotify {
			sender.SendNotification(settings, msg)
		}

		if notifySlow {
			slowMsg := msg
			slowMsg.Status = "slow_response"
			slowMsg.ErrorMessage = fmt.Sprintf("Response time %d ms exceeds threshold of %d ms", result.DurationMS, settings.SlowResponseThreshold)
			sender.SendNotification(settings, slowMsg)
		}
	}
}

func (wp *WorkerPool) updateMetrics(checkID int, duration time.Duration, isError bool) {
	metrics := wp.getOrCreateMetrics(checkID)

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	wp.updateErrorMetrics(metrics, isError)
	wp.updateDurationMetrics(metrics, duration)
	wp.checkOverload(checkID, metrics)
}

func (wp *WorkerPool) getOrCreateMetrics(checkID int) *CheckMetrics {
	wp.metricsMu.Lock()
	defer wp.metricsMu.Unlock()

	metrics, exists := wp.checkMetrics[checkID]
	if !exists {
		metrics = &CheckMetrics{}
		wp.checkMetrics[checkID] = metrics
	}
	return metrics
}

func (wp *WorkerPool) updateErrorMetrics(metrics *CheckMetrics, isError bool) {
	now := time.Now()
	if isError {
		metrics.errorCount++
		metrics.lastErrorTime = now
	} else {
		metrics.errorCount = 0
	}
	metrics.lastCheckTime = now
}

func (wp *WorkerPool) updateDurationMetrics(metrics *CheckMetrics, duration time.Duration) {
	if metrics.sampleCount < 10 {
		metrics.sampleCount++
		metrics.averageDuration = (metrics.averageDuration*time.Duration(metrics.sampleCount-1) + duration) / time.Duration(metrics.sampleCount)
	} else {
		metrics.averageDuration = wp.calculateExponentialMovingAverage(metrics.averageDuration, duration)
	}
}

func (wp *WorkerPool) calculateExponentialMovingAverage(current, newValue time.Duration) time.Duration {
	alpha := 0.2
	return time.Duration(float64(current)*(1-alpha) + float64(newValue)*alpha)
}

func (wp *WorkerPool) checkOverload(checkID int, metrics *CheckMetrics) {
	if metrics.errorCount >= 5 || metrics.averageDuration > 5*time.Second {
		log.Printf("Check %d: overload detected (errors: %d, avg duration: %v). Consider reducing interval.",
			checkID, metrics.errorCount, metrics.averageDuration)
	}
}
