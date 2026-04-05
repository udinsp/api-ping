package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/trioplanet/api-ping/internal/storage"
)

type Server struct {
	addr      string
	store     *storage.Store
	start     time.Time
	endpoints []string
}

func New(addr string, store *storage.Store, endpoints []string) *Server {
	return &Server{
		addr:      addr,
		store:     store,
		start:     time.Now(),
		endpoints: endpoints,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/metrics", s.handleMetrics)

	srv := &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return srv.ListenAndServe()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	uptime := time.Since(s.start)
	fmt.Fprintf(w, "# HELP api_ping_uptime_seconds API ping server uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE api_ping_uptime_seconds gauge\n")
	fmt.Fprintf(w, "api_ping_uptime_seconds %.2f\n\n", uptime.Seconds())

	for _, name := range s.endpoints {
		count, _ := s.store.GetRecentCheckCount(name, 24)
		if count == 0 {
			continue
		}

		for _, hours := range []int{24, 168, 720} {
			uptimePct, err := s.store.GetUptime(name, hours)
			if err == nil && uptimePct > 0 {
				label := fmt.Sprintf("%dh", hours)
				fmt.Fprintf(w, "# HELP api_ping_uptime_percentage_%s %s uptime over last %s\n", label, name, label)
				fmt.Fprintf(w, "# TYPE api_ping_uptime_percentage_%s gauge\n", label)
				fmt.Fprintf(w, "api_ping_uptime_percentage_%s{endpoint=\"%s\"} %.2f\n", label, name, uptimePct)
			}
		}

		avgDur, err := s.store.GetAverageResponseTime(name, 24)
		if err == nil && avgDur > 0 {
			fmt.Fprintf(w, "# HELP api_ping_response_time_ms_%s Average response time over last 24 hours in milliseconds\n", name)
			fmt.Fprintf(w, "# TYPE api_ping_response_time_ms_%s gauge\n", name)
			fmt.Fprintf(w, "api_ping_response_time_ms_%s{endpoint=\"%s\"} %.2f\n", name, name, avgDur)
		}
	}
}