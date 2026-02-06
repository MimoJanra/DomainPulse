package checker

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"
)

func RunTLSCheck(host string, port int, timeout time.Duration) CheckResult {
	return runTLSCheckWithSNI(host, host, port, timeout)
}

func tlsConfigForHost(serverName string) *tls.Config {
	cfg := &tls.Config{InsecureSkipVerify: true}
	if serverName != "" && net.ParseIP(serverName) == nil {
		cfg.ServerName = serverName
	}
	return cfg
}

func runTLSCheckWithSNI(host, serverName string, port int, timeout time.Duration) CheckResult {
	start := time.Now()
	address := net.JoinHostPort(host, strconv.Itoa(port))

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfigForHost(serverName))
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
			ErrorMessage: fmt.Sprintf("TLS connection failed: %v", err),
		}
	}

	if err := conn.Close(); err != nil {
		return CheckResult{
			Status:       "success",
			DurationMS:   int(duration),
			Outcome:      "success",
			ErrorMessage: "",
		}
	}
	return CheckResult{
		Status:       "success",
		DurationMS:   int(duration),
		Outcome:      "success",
		ErrorMessage: "",
	}
}

func RunTLSPersistentLoop(host string, port int, timeout time.Duration, onEvent func(CheckResult), stopChan chan struct{}) {
	readTimeout := 5 * time.Minute
	if timeout > 0 && timeout < readTimeout {
		readTimeout = timeout
	}

	for {
		select {
		case <-stopChan:
			return
		default:
		}

		address := net.JoinHostPort(host, strconv.Itoa(port))
		dialer := &net.Dialer{Timeout: timeout}
		conn, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfigForHost(host))

		if err != nil {
			status := "timeout"
			outcome := "timeout"
			if !isNetworkTimeout(err) {
				status = "error"
				outcome = "error"
			}
			onEvent(CheckResult{
				Status:       status,
				DurationMS:   0,
				Outcome:      outcome,
				ErrorMessage: fmt.Sprintf("TLS connection failed: %v", err),
			})
			select {
			case <-stopChan:
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		connectedAt := time.Now()
		onEvent(CheckResult{
			Status:       "success",
			DurationMS:   int(time.Since(connectedAt).Milliseconds()),
			Outcome:      "connected",
			ErrorMessage: "",
		})

		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		buf := make([]byte, 1)
		for {
			_, err := conn.Read(buf)
			if err != nil {
				onEvent(CheckResult{
					Status:       "error",
					DurationMS:   int(time.Since(connectedAt).Milliseconds()),
					Outcome:      "disconnected",
					ErrorMessage: fmt.Sprintf("connection closed: %v", err),
				})
				_ = conn.Close()
				break
			}
			_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		}

		select {
		case <-stopChan:
			return
		case <-time.After(5 * time.Second):
		}
	}
}
