package checker

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

func RunTCPCheckWithPayload(host string, port int, payload string, timeout time.Duration) CheckResult {
	start := time.Now()

	address := net.JoinHostPort(host, strconv.Itoa(port))

	conn, err := net.DialTimeout("tcp", address, timeout)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		status := "timeout"
		outcome := "timeout"
		if !isNetworkTimeout(err) {
			status = "error"
			outcome = "error"
		}
		return CheckResult{
			Status:       status,
			DurationMS:   int(duration),
			Outcome:      outcome,
			ErrorMessage: fmt.Sprintf("TCP connection failed: %v", err),
		}
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("failed to close TCP connection: %v", err)
		}
	}()

	if payload != "" {
		if _, writeErr := conn.Write([]byte(payload)); writeErr != nil {
			return CheckResult{
				Status:       "error",
				DurationMS:   int(duration),
				Outcome:      "error",
				ErrorMessage: fmt.Sprintf("TCP write failed: %v", writeErr),
			}
		}
	}

	return CheckResult{
		Status:       "success",
		DurationMS:   int(duration),
		Outcome:      "success",
		ErrorMessage: "",
	}
}

func isNetworkTimeout(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}
