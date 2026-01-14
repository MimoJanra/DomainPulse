package checker

import (
	"context"
	"fmt"
	"runtime"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

func RunICMPCheck(host string, timeout time.Duration) CheckResult {
	start := time.Now()

	pinger, err := createPinger(host, timeout)
	if err != nil {
		return createICMPErrorResult(fmt.Sprintf("failed to create pinger: %v", err), start)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan CheckResult, 1)

	go runPingerAsync(pinger, start, done)

	select {
	case result := <-done:
		return result
	case <-ctx.Done():
		return createICPTimeoutResult(start, "ping timeout exceeded")
	}
}

func createPinger(host string, timeout time.Duration) (*probing.Pinger, error) {
	pinger, err := probing.NewPinger(host)
	if err != nil {
		return nil, err
	}

	pinger.Count = 1
	pinger.Timeout = timeout
	setPingerPrivilege(pinger)

	return pinger, nil
}

func setPingerPrivilege(pinger *probing.Pinger) {
	if runtime.GOOS == "windows" {
		pinger.SetPrivileged(true)
	} else {
		pinger.SetPrivileged(false)
	}
}

func runPingerAsync(pinger *probing.Pinger, start time.Time, done chan CheckResult) {
	err := pinger.Run()
	if err != nil {
		done <- createICMPErrorResult(fmt.Sprintf("ping failed: %v", err), start)
		return
	}

	result := processPingStatistics(pinger, start)
	done <- result
}

func processPingStatistics(pinger *probing.Pinger, start time.Time) CheckResult {
	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		return createICPTimeoutResult(start, "no response received")
	}

	rtt := calculateRTT(stats)
	return CheckResult{
		Status:       "success",
		DurationMS:   int(rtt),
		Outcome:      "success",
		ErrorMessage: "",
	}
}

func calculateRTT(stats *probing.Statistics) int64 {
	rtt := stats.AvgRtt.Milliseconds()
	if rtt == 0 && stats.MinRtt > 0 {
		rtt = stats.MinRtt.Milliseconds()
	}
	return rtt
}

func createICMPErrorResult(errorMsg string, start time.Time) CheckResult {
	return CheckResult{
		Status:       "error",
		DurationMS:   int(time.Since(start).Milliseconds()),
		Outcome:      "error",
		ErrorMessage: errorMsg,
	}
}

func createICPTimeoutResult(start time.Time, message string) CheckResult {
	return CheckResult{
		Status:       "timeout",
		DurationMS:   int(time.Since(start).Milliseconds()),
		Outcome:      "timeout",
		ErrorMessage: message,
	}
}
