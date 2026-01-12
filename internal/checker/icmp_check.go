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

	pinger, err := probing.NewPinger(host)
	if err != nil {
		return CheckResult{
			Status:       "error",
			DurationMS:   int(time.Since(start).Milliseconds()),
			Outcome:      "error",
			ErrorMessage: fmt.Sprintf("failed to create pinger: %v", err),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pinger.Count = 1
	pinger.Timeout = timeout

	if runtime.GOOS == "windows" {
		pinger.SetPrivileged(true)
	} else {
		pinger.SetPrivileged(false)
	}

	done := make(chan bool, 1)
	var result CheckResult

	go func() {
		err := pinger.Run()
		if err != nil {
			result = CheckResult{
				Status:       "error",
				DurationMS:   int(time.Since(start).Milliseconds()),
				Outcome:      "error",
				ErrorMessage: fmt.Sprintf("ping failed: %v", err),
			}
		} else {
			stats := pinger.Statistics()
			if stats.PacketsRecv > 0 {
				rtt := stats.AvgRtt.Milliseconds()
				if rtt == 0 && stats.MinRtt > 0 {
					rtt = stats.MinRtt.Milliseconds()
				}
				result = CheckResult{
					Status:       "success",
					DurationMS:   int(rtt),
					Outcome:      "success",
					ErrorMessage: "",
				}
			} else {
				result = CheckResult{
					Status:       "timeout",
					DurationMS:   int(time.Since(start).Milliseconds()),
					Outcome:      "timeout",
					ErrorMessage: "no response received",
				}
			}
		}
		done <- true
	}()

	select {
	case <-done:
		return result
	case <-ctx.Done():
		return CheckResult{
			Status:       "timeout",
			DurationMS:   int(time.Since(start).Milliseconds()),
			Outcome:      "timeout",
			ErrorMessage: "ping timeout exceeded",
		}
	}
}
