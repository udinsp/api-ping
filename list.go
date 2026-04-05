package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/trioplanet/api-ping/internal/config"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured endpoints",
		Run: func(cmd *cobra.Command, args []string) {
			path := configPath()
			cfg, err := config.Load(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}

			if len(cfg.Endpoints) == 0 {
				fmt.Println("No endpoints configured.")
				return
			}

			events := "all"
			if len(cfg.Notifications.Events) > 0 {
				events = strings.Join(cfg.Notifications.Events, ", ")
			}

			fmt.Println("Configured endpoints")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Printf("%-20s %-40s %-8s %-10s %-10s %-8s %s\n",
				"NAME", "URL", "METHOD", "INTERVAL", "TIMEOUT", "STATUS", "NOTIFICATIONS")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

			for _, ep := range cfg.Endpoints {
				interval := ep.Interval
				if interval <= 0 {
					interval = 60
				}
				timeout := ep.Timeout
				if timeout <= 0 {
					timeout = 10
				}
				fmt.Printf("%-20s %-40s %-8s %-10d %-10d %-8d %s\n",
					ep.Name,
					ep.URL,
					ep.GetMethod(),
					interval,
					timeout,
					ep.GetExpectedStatus(),
					events,
				)
			}
		},
	}
}
