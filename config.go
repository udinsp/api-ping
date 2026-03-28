package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/trioplanet/api-ping/internal/config"

	"github.com/spf13/cobra"
)

func configPath() string {
	if p := os.Getenv("APIPING_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".api-ping.yaml"
	}
	return filepath.Join(home, ".api-ping.yaml")
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create default config file",
		Run: func(cmd *cobra.Command, args []string) {
			path := configPath()
			if _, err := os.Stat(path); err == nil {
				fmt.Printf("Config already exists at %s\n", path)
				return
			}

			cfg := config.DefaultConfig()
			if err := config.Save(path, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Created config at %s\n", path)
		},
	}
}

func newAddCmd() *cobra.Command {
	var name string
	var interval int
	var timeout int
	var method string

	cmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Add an endpoint to monitor",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := configPath()
			cfg, err := config.Load(path)
			if err != nil {
				cfg = config.DefaultConfig()
			}

			if name == "" {
				name = args[0]
			}

			ep := config.Endpoint{
				Name:     name,
				URL:      args[0],
				Method:   method,
				Interval: interval,
				Timeout:  timeout,
			}

			cfg.Endpoints = append(cfg.Endpoints, ep)

			if err := config.Save(path, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Added endpoint: %s (%s)\n", name, args[0])
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Endpoint name")
	cmd.Flags().IntVarP(&interval, "interval", "i", 60, "Check interval in seconds")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 10, "Request timeout in seconds")
	cmd.Flags().StringVarP(&method, "method", "m", "GET", "HTTP method")

	return cmd
}

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an endpoint",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := configPath()
			cfg, err := config.Load(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}

			var filtered []config.Endpoint
			for _, ep := range cfg.Endpoints {
				if ep.Name != args[0] && ep.URL != args[0] {
					filtered = append(filtered, ep)
				}
			}

			if len(filtered) == len(cfg.Endpoints) {
				fmt.Printf("Endpoint '%s' not found\n", args[0])
				return
			}

			cfg.Endpoints = filtered
			if err := config.Save(path, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Removed endpoint: %s\n", args[0])
		},
	}
}
