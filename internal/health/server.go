package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	mu          sync.RWMutex
	startTime   time.Time
	checkCounts map[string]*endpointMetrics
}

type endpointMetrics struct {
	name          string
	totalChecks   int64
	successCount  int64
	totalDuration int64 // milliseconds
	minDuration   int64
	maxDuration    int64
}

func NewMetrics() *Metrics {
	return &Metrics{
		startTime:   time.Now(),
		checkCounts: make(map[string]*endpointMetrics),
	}
}

func (m *Metrics) RecordCheck(endpointName string, success bool, durationMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	em, ok := m.checkCounts[endpointName]
	if !ok {
		em = &endpointMetrics{name: endpointName}
		m.checkCounts[endpointName] = em
	}

	atomic.AddInt64(&em.totalChecks, 1)
	if success {
		atomic.AddInt64(&em.successCount, 1)
	}
	atomic.AddInt64(&em.totalDuration, durationMs)

	var oldMin, oldMax int64
	for {
		oldMin = atomic.LoadInt64(&em.minDuration)
		if oldMin == 0 || durationMs < oldMin {
			if atomic.CompareAndSwapInt64(&em.minDuration, oldMin, durationMs) {
				break
			}
		} else {
			break
		}
	}

	for {
		oldMax := atomic.LoadInt64(&em.maxDuration)
		if durationMs > oldMax {
			if atomic.CompareAndSwapInt64(&em.maxDuration, oldMax, durationMs) {
				break
			}
		} else {
			break
		}
	}
}

type HealthResponse struct {
	Status string `json:"status"`
}

func (m *Metrics) ServeHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}

func (m *Metrics) ServeMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.1")

	m.mu.RLock()
	uptime := time.Since(m.startTime).Seconds()
	m.mu.RUnlock()

	fmt.Fprintf(w, "# HELP api_ping_uptime_seconds Total uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE api_ping_uptime_seconds gauge\n")
	fmt.Fprintf(w, "api_ping_uptime_seconds %.2f\n", uptime)

	fmt.Fprintf(w, "# HELP api_ping_endpoint_up Whether endpoint is up (1=yes, 0=no)\n")
	fmt.Fprintf(w, "# TYPE api_ping_endpoint_up gauge\n")

	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, em := range m.checkCounts {
		total := atomic.LoadInt64(&em.totalChecks)
		success := atomic.LoadInt64(&em.successCount)

		up := 0.0
		if total > 0 && success > 0 {
			up = 1.0
		}
		fmt.Fprintf(w, "api_ping_endpoint_up{endpoint=\"%s\"} %.0f\n", name, up)

		uptimePct := 0.0
		if total > 0 {
			uptimePct = float64(success) / float64(total) * 100
		}

		fmt.Fprintf(w, "# HELP api_ping_endpoint_uptime_percent Percentage of successful checks\n")
		fmt.Fprintf(w, "# TYPE api_ping_endpoint_uptime_percent gauge\n")
		fmt.Fprintf(w, "api_ping_endpoint_uptime_percent{endpoint=\"%s\"} %.2f\n", name, uptimePct)

		totalDur := atomic.LoadInt64(&em.totalDuration)
		avgDur := 0.0
		if total > 0 {
			avgDur = float64(totalDur) / float64(total)
		}

		minDur := atomic.LoadInt64(&em.minDuration)
		maxDur := atomic.LoadInt64(&em.maxDuration)

		fmt.Fprintf(w, "# HELP api_ping_endpoint_checks_total Total number of checks\n")
		fmt.Fprintf(w, "# TYPE api_ping_endpoint_checks_total counter\n")
		fmt.Fprintf(w, "api_ping_endpoint_checks_total{endpoint=\"%s\"} %d\n", name, total)

		fmt.Fprintf(w, "# HELP api_ping_endpoint_response_time_avg_ms Average response time in milliseconds\n")
		fmt.Fprintf(w, "# TYPE api_ping_endpoint_response_time_avg_ms gauge\n")
		fmt.Fprintf(w, "api_ping_endpoint_response_time_avg_ms{endpoint=\"%s\"} %.2f\n", name, avgDur)

		fmt.Fprintf(w, "# HELP api_ping_endpoint_response_time_min_ms Minimum response time in milliseconds\n")
		fmt.Fprintf(w, "# TYPE api_ping_endpoint_response_time_min_ms gauge\n")
		fmt.Fprintf(w, "api_ping_endpoint_response_time_min_ms{endpoint=\"%s\"} %.2f\n", name, float64(minDur))

		fmt.Fprintf(w, "# HELP api_ping_endpoint_response_time_max_ms Maximum response time in milliseconds\n")
		fmt.Fprintf(w, "# TYPE api_ping_endpoint_response_time_max_ms gauge\n")
		fmt.Fprintf(w, "api_ping_endpoint_response_time_max_ms{endpoint=\"%s\"} %.2f\n", name, float64(maxDur))
	}
}

type Server struct {
	httpServer *http.Server
	metrics    *Metrics
}

func NewServer(port int, bind string) *Server {
	m := NewMetrics()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", m.ServeHealth)
	mux.HandleFunc("/metrics", m.ServeMetrics)

	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", bind, port),
			Handler: mux,
		},
		metrics: m,
	}
}

func (s *Server) Metrics() *Metrics {
	return s.metrics
}

func (s *Server) Start() error {
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Health server error: %v\n", err)
		}
	}()
	return nil
}

func (s *Server) Stop() error {
	return s.httpServer.Close()
}