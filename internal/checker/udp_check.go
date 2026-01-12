package checker

import (
	"fmt"
	"net"
	"time"
)

func RunUDPCheck(host string, port int, payload string, timeout time.Duration) CheckResult {
	start := time.Now()

	address := fmt.Sprintf("%s:%d", host, port)

	conn, err := net.DialTimeout("udp", address, timeout)
	if err != nil {
		duration := time.Since(start).Milliseconds()
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
			ErrorMessage: fmt.Sprintf("UDP connection failed: %v", err),
		}
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		duration := time.Since(start).Milliseconds()
		return CheckResult{
			Status:       "error",
			DurationMS:   int(duration),
			Outcome:      "error",
			ErrorMessage: fmt.Sprintf("failed to set read deadline: %v", err),
		}
	}

	sendData := []byte(payload)
	if payload == "" {
		sendData = []byte("ping")
	}

	_, err = conn.Write(sendData)
	if err != nil {
		duration := time.Since(start).Milliseconds()
		return CheckResult{
			Status:       "error",
			DurationMS:   int(duration),
			Outcome:      "error",
			ErrorMessage: fmt.Sprintf("failed to send UDP packet: %v", err),
		}
	}

	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, err = conn.Read(buffer)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return CheckResult{
				Status:       "success",
				DurationMS:   int(duration),
				Outcome:      "no_response",
				ErrorMessage: "UDP packet sent but no response received (expected for UDP)",
			}
		}
		return CheckResult{
			Status:       "error",
			DurationMS:   int(duration),
			Outcome:      "error",
			ErrorMessage: fmt.Sprintf("UDP read error: %v", err),
		}
	}

	return CheckResult{
		Status:       "success",
		DurationMS:   int(duration),
		Outcome:      "success",
		ErrorMessage: "",
	}
}
