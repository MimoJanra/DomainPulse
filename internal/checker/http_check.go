package checker

import (
	"bytes"
	"fmt"
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
	Headers      map[string]string
}

type HTTPResult = CheckResult

func RunHTTPCheckWithMethod(url string, method string, body string, timeout time.Duration) CheckResult {
	return RunHTTPCheckWithMethodAndHeaders(url, method, body, nil, timeout)
}

func RunHTTPCheckWithMethodAndHeaders(url string, method string, body string, expectedHeaders map[string]string, timeout time.Duration) CheckResult {
	client := http.Client{Timeout: timeout}
	start := time.Now()

	method = normalizeMethod(method)
	req, err := createHTTPRequest(method, url, body)
	if err != nil {
		return createErrorResult(err.Error())
	}

	resp, err := client.Do(req)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return handleRequestError(err, int(duration))
	}

	defer closeResponseBody(resp.Body)
	return createSuccessResultWithHeaders(resp, int(duration), expectedHeaders)
}

func normalizeMethod(method string) string {
	if method == "" {
		return "GET"
	}
	return method
}

func createHTTPRequest(method, url, body string) (*http.Request, error) {
	if hasRequestBody(method, body) {
		req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	return http.NewRequest(method, url, nil)
}

func hasRequestBody(method, body string) bool {
	return body != "" && (method == "POST" || method == "PUT" || method == "PATCH")
}

func createErrorResult(errorMsg string) CheckResult {
	return CheckResult{
		Status:       "error",
		DurationMS:   0,
		Outcome:      "error",
		ErrorMessage: errorMsg,
	}
}

func handleRequestError(err error, duration int) CheckResult {
	status, outcome := determineErrorStatus(err)
	return CheckResult{
		Status:       status,
		DurationMS:   duration,
		Outcome:      outcome,
		ErrorMessage: err.Error(),
	}
}

func determineErrorStatus(err error) (status, outcome string) {
	if isTimeoutError(err) {
		return "timeout", "timeout"
	}
	return "error", "error"
}

func closeResponseBody(body io.ReadCloser) {
	if body != nil {
		_ = body.Close()
	}
}

func createSuccessResultWithHeaders(resp *http.Response, duration int, expectedHeaders map[string]string) CheckResult {
	status, outcome := determineResponseStatus(resp.StatusCode)
	errorMsg := ""

	if len(expectedHeaders) > 0 {
		missingHeaders := []string{}
		for key, expectedValue := range expectedHeaders {
			actualValue := resp.Header.Get(key)
			if actualValue == "" {
				missingHeaders = append(missingHeaders, key+" (missing)")
			} else if expectedValue != "" && actualValue != expectedValue {
				errorMsg = fmt.Sprintf("Header %s mismatch: expected '%s', got '%s'", key, expectedValue, actualValue)
				status = "error"
				outcome = "header_mismatch"
				break
			}
		}
		if len(missingHeaders) > 0 && errorMsg == "" {
			errorMsg = "Missing headers: " + strings.Join(missingHeaders, ", ")
			status = "error"
			outcome = "missing_headers"
		}
	}

	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return CheckResult{
		Status:       status,
		StatusCode:   resp.StatusCode,
		DurationMS:   duration,
		Outcome:      outcome,
		ErrorMessage: errorMsg,
		Headers:      headers,
	}
}

func determineResponseStatus(statusCode int) (status, outcome string) {
	switch {
	case statusCode >= 500:
		return "failure", "5xx"
	case statusCode >= 400:
		return "failure", "4xx"
	default:
		return "success", "2xx"
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
