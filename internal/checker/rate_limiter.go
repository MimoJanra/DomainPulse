package checker

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu          sync.Mutex
	tokens      int
	maxTokens   int
	refillRate  int 
	lastRefill  time.Time
	minInterval time.Duration
	lastRequest time.Time
}

func NewRateLimiter(maxTokensPerMinute int, minIntervalMS int) *RateLimiter {
	rl := &RateLimiter{
		maxTokens:  maxTokensPerMinute,
		refillRate: maxTokensPerMinute,
		lastRefill: time.Now(),
		minInterval: time.Duration(minIntervalMS) * time.Millisecond,
	}
	if maxTokensPerMinute > 0 {
		rl.tokens = maxTokensPerMinute
	}
	return rl
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	if rl.minInterval > 0 {
		if !rl.lastRequest.IsZero() && now.Sub(rl.lastRequest) < rl.minInterval {
			return false
		}
	}

	if rl.maxTokens <= 0 {
		rl.lastRequest = now
		return true
	}

	elapsed := now.Sub(rl.lastRefill)
	if elapsed >= time.Minute {
		rl.tokens = rl.maxTokens
		rl.lastRefill = now
	} else {
		tokensToAdd := int(float64(rl.refillRate) * elapsed.Seconds() / 60.0)
		if tokensToAdd > 0 {
			rl.tokens += tokensToAdd
			if rl.tokens > rl.maxTokens {
				rl.tokens = rl.maxTokens
			}
			rl.lastRefill = now
		}
	}

	if rl.tokens <= 0 {
		return false
	}

	rl.tokens--
	rl.lastRequest = now
	return true
}

func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	now := time.Now()

	rl.waitForMinInterval(&now)

	if rl.maxTokens <= 0 {
		rl.lastRequest = now
		rl.mu.Unlock()
		return
	}

	rl.refillTokens(now)
	rl.waitForAvailableTokens(&now)
	rl.consumeToken(now)

	rl.mu.Unlock()
}

func (rl *RateLimiter) waitForMinInterval(now *time.Time) {
	if rl.minInterval <= 0 || rl.lastRequest.IsZero() {
		return
	}

	sleepTime := rl.minInterval - (*now).Sub(rl.lastRequest)
	if sleepTime > 0 {
		rl.mu.Unlock()
		time.Sleep(sleepTime)
		rl.mu.Lock()
		*now = time.Now()
	}
}

func (rl *RateLimiter) refillTokens(now time.Time) {
	elapsed := now.Sub(rl.lastRefill)
	if elapsed >= time.Minute {
		rl.tokens = rl.maxTokens
		rl.lastRefill = now
		return
	}

	tokensToAdd := rl.calculateTokensToAdd(elapsed)
	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		rl.lastRefill = now
	}
}

func (rl *RateLimiter) calculateTokensToAdd(elapsed time.Duration) int {
	return int(float64(rl.refillRate) * elapsed.Seconds() / 60.0)
}

func (rl *RateLimiter) waitForAvailableTokens(now *time.Time) {
	for rl.tokens <= 0 {
		rl.waitUntilNextRefill(now)
		rl.refillTokens(*now)
	}
}

func (rl *RateLimiter) waitUntilNextRefill(now *time.Time) {
	nextRefill := rl.lastRefill.Add(time.Minute)
	sleepTime := nextRefill.Sub(*now)

	if sleepTime > 0 {
		rl.mu.Unlock()
		time.Sleep(sleepTime)
		rl.mu.Lock()
		*now = time.Now()
	} else {
		rl.tokens = rl.maxTokens
		rl.lastRefill = *now
	}
}

func (rl *RateLimiter) consumeToken(now time.Time) {
	rl.tokens--
	rl.lastRequest = now
}

var GlobalRateLimiter *RateLimiter

func InitGlobalRateLimiter(maxRequestsPerMinute int) {
	GlobalRateLimiter = NewRateLimiter(maxRequestsPerMinute, 0)
}
