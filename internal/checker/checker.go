package checker

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/trioplanet/api-ping/internal/config"
	"github.com/trioplanet/api-ping/internal/storage"
)

type Result struct {
	Endpoint   config.Endpoint
	StatusCode int
	Duration   time.Duration
	Success    bool
	Slow       bool
	Retries    int
	Error      string
}

func Check(ep config.Endpoint) Result {
	maxRetries := ep.GetRetries()
	var lastResult Result

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(ep.GetRetryDelay())
		}

		lastResult = checkOnce(ep)
		lastResult.Retries = attempt

		if lastResult.Success {
			return lastResult
		}
	}

	return lastResult
}

func checkOnce(ep config.Endpoint) Result {
	client := &http.Client{
		Timeout: ep.GetTimeout(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	var bodyReader io.Reader
	if ep.Body != "" {
		bodyReader = strings.NewReader(ep.Body)
	}

	req, err := http.NewRequest(ep.GetMethod(), ep.URL, bodyReader)
	if err != nil {
		return Result{
			Endpoint: ep,
			Success:  false,
			Error:    fmt.Sprintf("request creation failed: %v", err),
		}
	}

	for k, v := range ep.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return Result{
			Endpoint: ep,
			Duration: duration,
			Success:  false,
			Error:    fmt.Sprintf("request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	success := resp.StatusCode == ep.GetExpectedStatus()

	if success && ep.ExpectedBody != "" {
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		if err != nil {
			return Result{
				Endpoint:   ep,
				StatusCode: resp.StatusCode,
				Duration:   duration,
				Success:    false,
				Error:      fmt.Sprintf("read body failed: %v", err),
			}
		}
		if !strings.Contains(string(respBody), ep.ExpectedBody) {
			success = false
		}
	}

	result := Result{
		Endpoint:   ep,
		StatusCode: resp.StatusCode,
		Duration:   duration,
		Success:    success,
	}

	if !success {
		result.Error = fmt.Sprintf("expected status %d, got %d", ep.GetExpectedStatus(), resp.StatusCode)
	}

	if ep.GetMaxDuration() > 0 && duration > ep.GetMaxDuration() {
		result.Slow = true
	}

	return result
}

func ToStorageResult(r Result) storage.CheckResult {
	return storage.CheckResult{
		Endpoint:   r.Endpoint.Name,
		URL:        r.Endpoint.URL,
		StatusCode: r.StatusCode,
		Duration:   r.Duration.Milliseconds(),
		Success:    r.Success,
		Error:      r.Error,
		CheckedAt:  time.Now(),
	}
}
