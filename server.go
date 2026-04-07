package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/trioplanet/api-ping/internal/config"
	"github.com/trioplanet/api-ping/internal/storage"

	"github.com/spf13/cobra"
)

type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Endpoints map[string]Status `json:"endpoints,omitempty"`
}

type Status struct {
	Success   bool   `json:"success"`
	StatusCode int   `json:"status_code,omitempty"`
	Duration  int64  `json:"duration_ms,omitempty"`
	LastCheck time.Time `json:"last_check,omitempty"`
	Error     string `json:"error,omitempty"`
}

type MetricsResponse struct {
	Timestamp time.Time         `json:"timestamp"`
	Endpoints map[string]EndpointMetrics `json:"endpoints"`
}

type EndpointMetrics struct {
	Name       string  `json:"name"`
	URL        string  `json:"url"`
	Status     string  `json:"status"`
	Uptime     float64 `json:"uptime_percent,omitempty"`
	LastCheck  time.Time `json:"last_check,omitempty"`
	StatusCode int     `json:"status_code,omitempty"`
	Duration   int64   `json:"duration_ms,omitempty"`
}

func newServerCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start HTTP health/metrics server",
		Long: `Start an HTTP server that provides:
  - GET /health - Basic health check
  - GET /metrics - Current endpoint statuses and metrics`,
		Run: func(cmd *cobra.Command, args []string) {
			path := configPath()
			cfg, err := config.Load(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}

			store, err := storage.New(cfg.GetDBPath())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
				os.Exit(1)
			}
			defer store.Close()

			mux := http.NewServeMux()
			mux.HandleFunc("/health", handleHealth(store, cfg))
			mux.HandleFunc("/metrics", handleMetrics(store, cfg))

			addr := fmt.Sprintf(":%d", port)
			fmt.Printf("Starting server on %s\n", addr)
			fmt.Printf("Health endpoint: http://%s/health\n", addr)
			fmt.Printf("Metrics endpoint: http://%s/metrics\n", addr)

srv := &http.Server{Addr: addr, Handler: mux}
go func() {
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}()

sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
<-sigCh
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := srv.Shutdown(ctx); err != nil {
	fmt.Fprintf(os.Stderr, "Server shutdown error: %v\n", err)
}
				fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to listen on")

	return cmd
}

func handleHealth(store storage.Storage, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		statuses, err := store.GetAllStatus()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(HealthResponse{
				Status:    "error",
				Timestamp: time.Now(),
			})
			return
		}

		response := HealthResponse{
			Status:    "ok",
			Timestamp: time.Now(),
			Endpoints: make(map[string]Status),
		}

		for name, s := range statuses {
			response.Endpoints[name] = Status{
				Success:    s.Success,
				StatusCode: s.StatusCode,
				Duration:  s.Duration,
				LastCheck: s.CheckedAt,
				Error:     s.Error,
			}
		}

		json.NewEncoder(w).Encode(response)
	}
}

func handleMetrics(store storage.Storage, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		statuses, err := store.GetAllStatus()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(MetricsResponse{
				Timestamp: time.Now(),
				Endpoints: map[string]EndpointMetrics{},
			})
			return
		}

		response := MetricsResponse{
			Timestamp: time.Now(),
			Endpoints: make(map[string]EndpointMetrics),
		}

		for _, ep := range cfg.Endpoints {
			s, ok := statuses[ep.Name]

			metrics := EndpointMetrics{
				Name: ep.Name,
				URL: ep.URL,
			}

			if ok {
				metrics.StatusCode = s.StatusCode
				metrics.Duration = s.Duration
				metrics.LastCheck = s.CheckedAt

				if s.Success {
					metrics.Status = "up"
				} else {
					metrics.Status = "down"
				}
			} else {
				metrics.Status = "unknown"
			}

			uptime, err := store.GetUptime(ep.Name, 24)
			if err == nil {
				metrics.Uptime = uptime
			}

			response.Endpoints[ep.Name] = metrics
		}

		json.NewEncoder(w).Encode(response)
	}
}
