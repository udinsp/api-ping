package http

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

type Server struct {
	addr   string
	server *http.Server
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
}

type MetricsResponse struct {
	Timestamp     time.Time `json:"timestamp"`
	TotalChecks   uint64    `json:"total_checks"`
	SuccessCount  uint64    `json:"success_count"`
	FailureCount  uint64    `json:"failure_count"`
	AvgLatencyMs  float64   `json:"avg_latency_ms"`
	UptimePercent float64   `json:"uptime_percent"`
}

var (
	totalChecks   uint64
	successCount  uint64
	failureCount  uint64
	avgLatencySum uint64
	startTime     = time.Now()
)

func RecordCheck(success bool, latencyMs float64) {
	atomic.AddUint64(&totalChecks, 1)
	atomic.AddUint64(&avgLatencySum, uint64(latencyMs))
	if success {
		atomic.AddUint64(&successCount, 1)
	} else {
		atomic.AddUint64(&failureCount, 1)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Uptime:    time.Since(startTime).Round(time.Second).String(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	total := atomic.LoadUint64(&totalChecks)
	success := atomic.LoadUint64(&successCount)
	latencySum := atomic.LoadUint64(&avgLatencySum)

	var avgLatency float64
	if total > 0 {
		avgLatency = float64(latencySum) / float64(total)
	}

	var uptimePercent float64
	if total > 0 {
		uptimePercent = float64(success) / float64(total) * 100
	}

	resp := MetricsResponse{
		Timestamp:     time.Now().UTC(),
		TotalChecks:   total,
		SuccessCount:  success,
		FailureCount:  total - success,
		AvgLatencyMs:  avgLatency,
		UptimePercent: uptimePercent,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func New(addr string) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/metrics", metricsHandler)

	return &Server{
		addr: addr,
		server: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
}

func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

func (s *Server) Stop() error {
	return s.server.Close()
}