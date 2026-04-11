package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/trioplanet/api-ping/internal/config"
	"github.com/trioplanet/api-ping/internal/storage"
)

type Server struct {
	cfg        config.HealthServerConfig
	store      *storage.Store
	startTime  time.Time
	httpServer *http.Server

	endpointStatsMu sync.RWMutex
	endpointStats  map[string]*EndpointStats
}

type EndpointStats struct {
	mu                sync.RWMutex
	TotalRequests     int64
	SuccessfulRequests int64
	TotalResponseTime int64 // in milliseconds
	LastResponseTime  int64 // in milliseconds
	LastStatusCode    int
	LastSuccess       bool
	LastCheckedAt     time.Time
}

func New(cfg config.HealthServerConfig, store *storage.Store) *Server {
	return &Server{
		cfg:           cfg,
		store:         store,
		startTime:     time.Now(),
		endpointStats: make(map[string]*EndpointStats),
	}
}

func (s *Server) AddEndpoint(name string) {
	s.endpointStatsMu.Lock()
	defer s.endpointStatsMu.Unlock()
	s.endpointStats[name] = &EndpointStats{}
}

func (s *Server) RecordCheck(endpoint string, statusCode int, duration time.Duration, success bool) {
	s.endpointStatsMu.RLock()
	stats, ok := s.endpointStats[endpoint]
	s.endpointStatsMu.RUnlock()

	if !ok {
		return
	}

	stats.mu.Lock()
	atomic.AddInt64(&stats.TotalRequests, 1)
	if success {
		atomic.AddInt64(&stats.SuccessfulRequests, 1)
	}
	atomic.AddInt64(&stats.TotalResponseTime, duration.Milliseconds())
	atomic.StoreInt64(&stats.LastResponseTime, duration.Milliseconds())
	stats.LastStatusCode = statusCode
	stats.LastSuccess = success
	stats.LastCheckedAt = time.Now()
	stats.mu.Unlock()
}

func (s *Server) Start() error {
	if !s.cfg.Enabled {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/metrics", s.handleMetrics)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.cfg.GetBind(), s.cfg.GetPort()),
		Handler: mux,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Health server error: %v\n", err)
		}
	}()

	fmt.Printf("Health server listening on %s:%d\n", s.cfg.GetBind(), s.cfg.GetPort())
	return nil
}

func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(s.startTime)
	uptimePercent := 100.0

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	fmt.Fprintf(w, "# HELP api_ping_uptime_seconds Total uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE api_ping_uptime_seconds gauge\n")
	fmt.Fprintf(w, "api_ping_uptime_seconds %.0f\n\n", uptime.Seconds())

	fmt.Fprintf(w, "# HELP api_ping_uptime_percentage Uptime percentage\n")
	fmt.Fprintf(w, "# TYPE api_ping_uptime_percentage gauge\n")
	fmt.Fprintf(w, "api_ping_uptime_percentage %.2f\n\n", uptimePercent)

	s.endpointStatsMu.RLock()
	defer s.endpointStatsMu.RUnlock()

	for name, stats := range s.endpointStats {
		stats.mu.RLock()
		total := atomic.LoadInt64(&stats.TotalRequests)
		success := atomic.LoadInt64(&stats.SuccessfulRequests)
		totalRespTime := atomic.LoadInt64(&stats.TotalResponseTime)
		lastRespTime := atomic.LoadInt64(&stats.LastResponseTime)
		lastStatus := stats.LastStatusCode
		lastSuccess := stats.LastSuccess
		stats.mu.RUnlock()

		var avgRespTime float64
		if total > 0 {
			avgRespTime = float64(totalRespTime) / float64(total)
		}

		var uptimePct float64
		if total > 0 {
			uptimePct = float64(success) / float64(total) * 100
		}

		safeName := sanitizeMetricName(name)

		fmt.Fprintf(w, "# HELP api_ping_endpoint_uptime_percentage Uptime percentage for endpoint\n")
		fmt.Fprintf(w, "# TYPE api_ping_endpoint_uptime_percentage gauge\n")
		fmt.Fprintf(w, "api_ping_endpoint_uptime_percentage{name=\"%s\"} %.2f\n\n", safeName, uptimePct)

		fmt.Fprintf(w, "# HELP api_ping_endpoint_response_time_avg_ms Average response time in milliseconds\n")
		fmt.Fprintf(w, "# TYPE api_ping_endpoint_response_time_avg_ms gauge\n")
		fmt.Fprintf(w, "api_ping_endpoint_response_time_avg_ms{name=\"%s\"} %.2f\n\n", safeName, avgRespTime)

		fmt.Fprintf(w, "# HELP api_ping_endpoint_response_time_last_ms Last response time in milliseconds\n")
		fmt.Fprintf(w, "# TYPE api_ping_endpoint_response_time_last_ms gauge\n")
		fmt.Fprintf(w, "api_ping_endpoint_response_time_last_ms{name=\"%s\"} %d\n\n", safeName, lastRespTime)

		fmt.Fprintf(w, "# HELP api_ping_endpoint_status_code_last Last HTTP status code\n")
		fmt.Fprintf(w, "# TYPE api_ping_endpoint_status_code_last gauge\n")
		fmt.Fprintf(w, "api_ping_endpoint_status_code_last{name=\"%s\"} %d\n\n", safeName, lastStatus)

		fmt.Fprintf(w, "# HELP api_ping_endpoint_success_last Last check success status\n")
		fmt.Fprintf(w, "# TYPE api_ping_endpoint_success_last gauge\n")
		if lastSuccess {
			fmt.Fprintf(w, "api_ping_endpoint_success_last{name=\"%s\"} 1\n\n", safeName)
		} else {
			fmt.Fprintf(w, "api_ping_endpoint_success_last{name=\"%s\"} 0\n\n", safeName)
		}

		fmt.Fprintf(w, "# HELP api_ping_endpoint_total_requests Total number of requests\n")
		fmt.Fprintf(w, "# TYPE api_ping_endpoint_total_requests counter\n")
		fmt.Fprintf(w, "api_ping_endpoint_total_requests{name=\"%s\"} %d\n\n", safeName, total)
	}
}

func sanitizeMetricName(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}
