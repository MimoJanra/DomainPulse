package checker

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"
)

type CheckResult struct {
	Status       string
	StatusCode   int
	DurationMS   int
	Outcome      string
	ErrorMessage string
}

type HTTPResult = CheckResult

func RunHTTPCheckWithMethod(url string, method string, body string, timeout time.Duration) CheckResult {
	client := http.Client{Timeout: timeout}

	var req *http.Request
	var err error

	start := time.Now()

	if method == "" {
		method = "GET"
	}

	if body != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		req, err = http.NewRequest(method, url, bytes.NewBufferString(body))
		if err != nil {
			return CheckResult{
				Status:       "error",
				DurationMS:   0,
				Outcome:      "error",
				ErrorMessage: err.Error(),
			}
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return CheckResult{
				Status:       "error",
				DurationMS:   0,
				Outcome:      "error",
				ErrorMessage: err.Error(),
			}
		}
	}

	resp, err := client.Do(req)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		errorMsg := err.Error()
		status := "timeout"
		outcome := "timeout"
		if !isTimeoutError(err) {
			status = "error"
			outcome = "error"
		}
		return CheckResult{
			Status:       status,
			DurationMS:   int(duration),
			Outcome:      outcome,
			ErrorMessage: errorMsg,
		}
	}
	defer func(Body io.ReadCloser) {
		if Body != nil {
			err := Body.Close()
			if err != nil {
			}
		}
	}(resp.Body)

	status := "success"
	outcome := "2xx"
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		status = "failure"
		outcome = "4xx"
	} else if resp.StatusCode >= 500 {
		status = "failure"
		outcome = "5xx"
	}

	return CheckResult{
		Status:       status,
		StatusCode:   resp.StatusCode,
		DurationMS:   int(duration),
		Outcome:      outcome,
		ErrorMessage: "",
	}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "context deadline exceeded")
}
