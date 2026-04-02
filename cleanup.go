package main

import (
	"fmt"
	"os"

	"github.com/trioplanet/api-ping/internal/config"
	"github.com/trioplanet/api-ping/internal/storage"

	"github.com/spf13/cobra"
)

func newCleanupCmd() *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Purge old check records from the database",
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

			retention := cfg.GetRetentionDays()
			if days > 0 {
				retention = days
			}

			n, err := store.PurgeOldChecks(retention)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error purging old checks: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Deleted %d check record(s) older than %d days\n", n, retention)
		},
	}

	cmd.Flags().IntVarP(&days, "days", "d", 0, "Override retention days (default: from config)")

	return cmd
}
