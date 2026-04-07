package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/trioplanet/api-ping/internal/checker"
	"github.com/trioplanet/api-ping/internal/config"
	"github.com/trioplanet/api-ping/internal/health"
	"github.com/trioplanet/api-ping/internal/notify"
	"github.com/trioplanet/api-ping/internal/storage"

	"github.com/spf13/cobra"
)

func newMonitorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "monitor",
		Short: "Start monitoring endpoints",
		Run: func(cmd *cobra.Command, args []string) {
			path := configPath()
			cfg, err := config.Load(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}

			if len(cfg.Endpoints) == 0 {
				fmt.Println("No endpoints configured. Use 'api-ping add <url>' to add one.")
				return
			}

			store, err := storage.New(cfg.GetDBPath())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
				os.Exit(1)
			}
			defer store.Close()

			// Purge old checks on startup
			if n, err := store.PurgeOldChecks(cfg.GetRetentionDays()); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: cleanup failed: %v\n", err)
			} else if n > 0 {
				fmt.Printf("Cleaned up %d old check record(s)\n", n)
			}

			prevState := make(map[string]bool)
			prevSlow := make(map[string]bool)
			for _, ep := range cfg.Endpoints {
				prevState[ep.Name] = true
				prevSlow[ep.Name] = false
			}

			fmt.Printf("api-ping monitoring %d endpoints...\n", len(cfg.Endpoints))
			fmt.Println("Press Ctrl+C to stop")

			var healthServer *health.Server
			var metrics *health.Metrics
			if cfg.HealthServer.Enabled {
				healthServer = health.NewServer(cfg.HealthServer.GetPort(), cfg.HealthServer.GetBind())
				metrics = healthServer.Metrics()
				if err := healthServer.Start(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to start health server: %v\n", err)
				} else {
					fmt.Printf("Health server listening on http://%s:%d\n", cfg.HealthServer.GetBind(), cfg.HealthServer.GetPort())
				}
			}

			fmt.Println()

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			var wg sync.WaitGroup
			var mu sync.Mutex

			// Periodic cleanup goroutine (every 24 hours)
			wg.Add(1)
			go func() {
				defer wg.Done()
				ticker := time.NewTicker(24 * time.Hour)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if n, err := store.PurgeOldChecks(cfg.GetRetentionDays()); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: cleanup failed: %v\n", err)
						} else if n > 0 {
							fmt.Printf("Cleaned up %d old check record(s)\n", n)
						}
					case <-quit:
						return
					}
				}
			}()

			for _, ep := range cfg.Endpoints {
				wg.Add(1)
				go func(endpoint config.Endpoint) {
					defer wg.Done()
					ticker := time.NewTicker(endpoint.GetInterval())
					defer ticker.Stop()

					doCheck(endpoint, cfg.Notifications, store, prevState, prevSlow, &mu, metrics)

					for {
						select {
						case <-ticker.C:
							doCheck(endpoint, cfg.Notifications, store, prevState, prevSlow, &mu, metrics)
						case <-quit:
							return
						}
					}
				}(ep)
			}

		<-quit
		fmt.Println("\nShutting down...")
		if healthServer != nil {
			healthServer.Stop()
		}
		wg.Wait()
	},
}
}

func doCheck(ep config.Endpoint, notifs config.Notifications, store *storage.Store, prevState map[string]bool, prevSlow map[string]bool, mu *sync.Mutex, metrics *health.Metrics) {
	result := checker.Check(ep)

	if err := store.SaveCheck(checker.ToStorageResult(result)); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving check: %v\n", err)
	}

	if metrics != nil {
		metrics.RecordCheck(ep.Name, result.Success, result.Duration.Milliseconds())
	}

	icon := "✓"
	if !result.Success {
		icon = "✗"
	} else if result.Slow {
		icon = "~"
	}
	fmt.Printf("[%s] %s | %s | %d | %v | %s\n",
		time.Now().Format("15:04:05"),
		icon,
		ep.Name,
		result.StatusCode,
		result.Duration.Round(time.Millisecond),
		ep.URL,
	)

	// Determine notification events before acquiring lock
	var events []string
	mu.Lock()
	wasUp := prevState[ep.Name]
	isUp := result.Success
	wasSlow := prevSlow[ep.Name]

	if wasUp && !isUp {
		events = append(events, "down")
	} else if !wasUp && isUp {
		events = append(events, "recovered")
	}

	if result.Slow && !wasSlow {
		events = append(events, "slow")
	}

	prevState[ep.Name] = isUp
	prevSlow[ep.Name] = result.Slow
	mu.Unlock()

	// Send notifications outside of lock to avoid blocking other goroutines
	for _, event := range events {
		notify.NotifyAll(notifs, event, result)
	}
}
