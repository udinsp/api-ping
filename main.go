package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.1.0"

func main() {
	root := &cobra.Command{
		Use:     "api-ping",
		Short:   "Self-hosted API uptime monitor",
		Long:    "api-ping monitors your API endpoints and notifies you when they go down.\nSupports Telegram, Discord, and webhook notifications.",
		Version: version,
	}

	root.AddCommand(
		newAddCmd(),
		newRemoveCmd(),
		newMonitorCmd(),
		newStatusCmd(),
		newLogsCmd(),
		newInitCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
