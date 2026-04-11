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

			healthServer := health.New(cfg.HealthServer, store)
			for _, ep := range cfg.Endpoints {
				healthServer.AddEndpoint(ep.Name)
			}
			if err := healthServer.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Error starting health server: %v\n", err)
				os.Exit(1)
			}
			defer healthServer.Stop()

			fmt.Printf("api-ping monitoring %d endpoints...\n", len(cfg.Endpoints))
			fmt.Println("Press Ctrl+C to stop")

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

					doCheck(endpoint, cfg.Notifications, store, healthServer, prevState, prevSlow, &mu)

					for {
						select {
						case <-ticker.C:
							doCheck(endpoint, cfg.Notifications, store, healthServer, prevState, prevSlow, &mu)
						case <-quit:
							return
						}
					}
				}(ep)
			}

			<-quit
			fmt.Println("\nShutting down...")
			wg.Wait()
		},
	}
}

func doCheck(ep config.Endpoint, notifs config.Notifications, store *storage.Store, healthServer *health.Server, prevState map[string]bool, prevSlow map[string]bool, mu *sync.Mutex) {
	result := checker.Check(ep)

	if err := store.SaveCheck(checker.ToStorageResult(result)); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving check: %v\n", err)
	}

	healthServer.RecordCheck(ep.Name, result.StatusCode, result.Duration, result.Success)

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
