package main

import (
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/trioplanet/api-ping/internal/config"
	"github.com/trioplanet/api-ping/internal/storage"

	"github.com/spf13/cobra"
)

func newStatusPageCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "status-page",
		Short: "Generate a static HTML status page",
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

			type EndpointStatus struct {
				Name       string
				URL        string
				Status     string
				StatusCode int
				Duration   int64
				Uptime24h  float64
				Uptime7d   float64
				LastCheck  string
			}

			var endpoints []EndpointStatus
			allUp := true

			for _, ep := range cfg.Endpoints {
				s, ok := statuses[ep.Name]
				status := "unknown"
				statusCode := 0
				duration := int64(0)
				lastCheck := "never"

				if ok {
					statusCode = s.StatusCode
					duration = s.Duration
					lastCheck = time.Since(s.CheckedAt).Round(time.Second).String() + " ago"
					if s.Success {
						status = "operational"
					} else {
						status = "outage"
						allUp = false
					}
				}

			uptime24, err := store.GetUptime(ep.Name, 24)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting 24h uptime for %s: %v\n", ep.Name, err)
				os.Exit(1)
			}
			uptime7d, err := store.GetUptime(ep.Name, 168)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting 7d uptime for %s: %v\n", ep.Name, err)
				os.Exit(1)
			}
				uptime7d, _ := store.GetUptime(ep.Name, 168)

				endpoints = append(endpoints, EndpointStatus{
					Name:       ep.Name,
					URL:        ep.URL,
					Status:     status,
					StatusCode: statusCode,
					Duration:   duration,
					Uptime24h:  uptime24,
					Uptime7d:   uptime7d,
					LastCheck:  lastCheck,
				})
			}

			data := struct {
				Title     string
				AllUp     bool
				Endpoints []EndpointStatus
				Generated string
			}{
				Title:     "API Status",
				AllUp:     allUp,
				Endpoints: endpoints,
				Generated: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
			}

			out := os.Stdout
			if outputFile != "" {
				f, err := os.Create(outputFile)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
					os.Exit(1)
				}
				defer f.Close()
				out = f
			}

			tmpl := template.Must(template.New("status").Parse(statusPageHTML))
			if err := tmpl.Execute(out, data); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating page: %v\n", err)
				os.Exit(1)
			}

			if outputFile != "" {
				fmt.Printf("Status page generated: %s\n", outputFile)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")

	return cmd
}

const statusPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{ .Title }}</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:system-ui,-apple-system,sans-serif;background:#0f172a;color:#e2e8f0;min-height:100vh;padding:40px 20px}
.container{max-width:720px;margin:0 auto}
.header{text-align:center;margin-bottom:40px}
.header h1{font-size:28px;font-weight:700;margin-bottom:8px}
.status-badge{display:inline-flex;align-items:center;gap:8px;padding:8px 20px;border-radius:99px;font-size:14px;font-weight:600}
.status-badge.up{background:rgba(34,197,94,.15);color:#22c55e;border:1px solid rgba(34,197,94,.3)}
.status-badge.down{background:rgba(239,68,68,.15);color:#ef4444;border:1px solid rgba(239,68,68,.3)}
.dot{width:8px;height:8px;border-radius:50%}
.dot.up{background:#22c55e}
.dot.down{background:#ef4444}
.endpoint{background:#1e293b;border:1px solid #334155;border-radius:12px;padding:20px;margin-bottom:12px;display:flex;justify-content:space-between;align-items:center}
.endpoint-info h3{font-size:16px;font-weight:600;margin-bottom:4px}
.endpoint-info p{font-size:13px;color:#94a3b8}
.endpoint-stats{text-align:right}
.endpoint-stats .status{font-size:14px;font-weight:600;margin-bottom:4px}
.endpoint-stats .status.operational{color:#22c55e}
.endpoint-stats .status.outage{color:#ef4444}
.endpoint-stats .status.unknown{color:#94a3b8}
.endpoint-stats .meta{font-size:12px;color:#64748b}
.uptime{margin-top:8px;font-size:12px;color:#94a3b8}
.footer{text-align:center;margin-top:40px;font-size:12px;color:#475569}
</style>
</head>
<body>
<div class="container">
  <div class="header">
    <h1>{{ .Title }}</h1>
    {{ if .AllUp }}
    <div class="status-badge up"><span class="dot up"></span> All Systems Operational</div>
    {{ else }}
    <div class="status-badge down"><span class="dot down"></span> Some Systems Experiencing Issues</div>
    {{ end }}
  </div>
  {{ range .Endpoints }}
  <div class="endpoint">
    <div class="endpoint-info">
      <h3>{{ .Name | html }}</h3>
      <p>{{ .URL | html }}</p>
      <div class="uptime">Uptime 24h: {{ printf "%.1f" .Uptime24h }}% | 7d: {{ printf "%.1f" .Uptime7d }}%</div>
    </div>
    <div class="endpoint-stats">
      <div class="status {{ .Status }}">{{ if eq .Status "operational" }}Operational{{ else if eq .Status "outage" }}Outage{{ else }}Unknown{{ end }}</div>
      <div class="meta">{{ .StatusCode }} · {{ .Duration }}ms · {{ .LastCheck }}</div>
    </div>
  </div>
  {{ end }}
  <div class="footer">Generated by api-ping · {{ .Generated }}</div>
</div>
</body>
</html>`
