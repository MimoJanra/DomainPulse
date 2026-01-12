package checker

import (
	"io"
	"net/http"
	"time"
)

type HTTPResult struct {
	StatusCode int
	DurationMS int
	Outcome    string
}

func RunHTTPCheck(url string, timeout time.Duration) HTTPResult {
	client := http.Client{Timeout: timeout}

	start := time.Now()
	resp, err := client.Get(url)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return HTTPResult{Outcome: "timeout", DurationMS: int(duration)}
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	outcome := "2xx"
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		outcome = "4xx"
	} else if resp.StatusCode >= 500 {
		outcome = "5xx"
	}

	return HTTPResult{
		StatusCode: resp.StatusCode,
		DurationMS: int(duration),
		Outcome:    outcome,
	}
}
