package main

import (
	"fmt"
	"os"
	"time"

	"github.com/trioplanet/api-ping/internal/config"
	"github.com/trioplanet/api-ping/internal/storage"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current status of all endpoints",
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

			statuses, err := store.GetAllStatus()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting status: %v\n", err)
				os.Exit(1)
			}

			if len(cfg.Endpoints) == 0 {
				fmt.Println("No endpoints configured.")
				return
			}

			fmt.Println("api-ping status")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Printf("%-20s %-8s %-10s %-8s %s\n", "NAME", "STATUS", "DURATION", "HTTP", "LAST CHECK")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

			for _, ep := range cfg.Endpoints {
				s, ok := statuses[ep.Name]
				if !ok {
					fmt.Printf("%-20s %-8s %-10s %-8s %s\n", ep.Name, "—", "—", "—", "never")
					continue
				}

				status := "✓ UP"
				if !s.Success {
					status = "✗ DOWN"
				} else if ep.GetMaxDuration() > 0 && s.Duration > ep.GetMaxDuration().Milliseconds() {
					status = "~ SLOW"
				}

				ago := time.Since(s.CheckedAt).Round(time.Second)
				fmt.Printf("%-20s %-8s %-10dms %-8d %s ago\n",
					ep.Name,
					status,
					s.Duration,
					s.StatusCode,
					ago,
				)
			}
		},
	}
}

func newLogsCmd() *cobra.Command {
	var hours int
	var endpoint string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show check history",
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

			targets := cfg.Endpoints
			if endpoint != "" {
				targets = nil
				for _, ep := range cfg.Endpoints {
					if ep.Name == endpoint {
						targets = append(targets, ep)
						break
					}
				}
				if targets == nil {
					fmt.Printf("Endpoint '%s' not found\n", endpoint)
					return
				}
			}

			fmt.Printf("api-ping logs (last %dh)\n", hours)
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

			for _, ep := range targets {
				checks, err := store.GetRecentChecks(ep.Name, hours)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error getting logs for %s: %v\n", ep.Name, err)
					continue
				}

				if len(checks) == 0 {
					fmt.Printf("\n%s: no data\n", ep.Name)
					continue
				}

			uptime, err := store.GetUptime(ep.Name, hours)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting uptime for %s: %v\n", ep.Name, err)
				continue
			}
			fmt.Printf("\n%s (uptime: %.1f%%)\n", ep.Name, uptime)

				for _, c := range checks {
					icon := "✓"
					if !c.Success {
						icon = "✗"
					}
					fmt.Printf("  %s %s | %d | %dms | %s\n",
						icon,
						c.CheckedAt.Format("15:04:05"),
						c.StatusCode,
						c.Duration,
						c.Error,
					)
				}
			}
		},
	}

	cmd.Flags().IntVarP(&hours, "hours", "H", 24, "Hours of history")
	cmd.Flags().StringVarP(&endpoint, "endpoint", "e", "", "Filter by endpoint name")

	return cmd
}
