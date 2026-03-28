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

			// Track previous state for notifications
			prevState := make(map[string]bool)
			for _, ep := range cfg.Endpoints {
				prevState[ep.Name] = true
			}

			fmt.Printf("api-ping monitoring %d endpoints...\n", len(cfg.Endpoints))
			fmt.Println("Press Ctrl+C to stop")
			fmt.Println()

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			var wg sync.WaitGroup

			for _, ep := range cfg.Endpoints {
				wg.Add(1)
				go func(endpoint config.Endpoint) {
					defer wg.Done()
					ticker := time.NewTicker(endpoint.GetInterval())
					defer ticker.Stop()

					// First check immediately
					doCheck(endpoint, cfg.Notifications, store, prevState)

					for {
						select {
						case <-ticker.C:
							doCheck(endpoint, cfg.Notifications, store, prevState)
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

func doCheck(ep config.Endpoint, notifs config.Notifications, store *storage.Store, prevState map[string]bool) {
	result := checker.Check(ep)

	// Save to database
	if err := store.SaveCheck(checker.ToStorageResult(result)); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving check: %v\n", err)
	}

	// Print status
	icon := "✓"
	if !result.Success {
		icon = "✗"
	}
	fmt.Printf("[%s] %s | %s | %d | %v | %s\n",
		time.Now().Format("15:04:05"),
		icon,
		ep.Name,
		result.StatusCode,
		result.Duration.Round(time.Millisecond),
		ep.URL,
	)

	// Notify on state change
	wasUp := prevState[ep.Name]
	isUp := result.Success

	if wasUp && !isUp {
		notify.NotifyAll(notifs, "down", result)
	} else if !wasUp && isUp {
		notify.NotifyAll(notifs, "recovered", result)
	}

	prevState[ep.Name] = isUp
}
