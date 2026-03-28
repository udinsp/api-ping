# api-ping 🏓

[![CI](https://github.com/udinsp/api-ping/actions/workflows/ci.yml/badge.svg)](https://github.com/udinsp/api-ping/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/udinsp/api-ping)](https://github.com/udinsp/api-ping/releases)

Self-hosted API uptime monitor with notifications. Simple, lightweight, built in Go — no Docker required.

## Why api-ping?

- **Zero dependencies** — single binary, no Docker, no Node, no Python
- **Private** — your data stays on your machine
- **Simple** — YAML config, CLI interface, SQLite storage
- **Alerts where you want** — Telegram, Discord, or custom webhooks
- **No false alarms** — retry on failure before reporting down

## Features

- 🔍 Monitor multiple HTTP endpoints (GET, POST, custom headers)
- ⏱️ Configurable check intervals per endpoint
- 📊 Response time & status tracking
- 🐢 Slow response alerts (configurable threshold)
- 🔁 Retry on failure (reduce false alarms)
- 💾 SQLite storage for history
- 🔔 Notifications via Telegram, Discord, or Webhook
- 📈 Uptime percentage (24h, 7d)
- 📄 Static status page generator
- 🚀 Single binary, cross-platform (Linux, macOS, Windows)

## Install

```bash
go install github.com/trioplanet/api-ping@latest
```

Or download from [Releases](https://github.com/udinsp/api-ping/releases).

Or build from source:

```bash
git clone https://github.com/trioplanet/api-ping.git
cd api-ping
go build -o api-ping .
```

## Quick Start

```bash
# Initialize config
api-ping init

# Add an endpoint
api-ping add https://api.example.com/health --name "My API" --interval 60

# Start monitoring
api-ping monitor
```

## Commands

| Command | Description |
|---------|-------------|
| `api-ping init` | Create default config file |
| `api-ping add <url>` | Add endpoint to monitor |
| `api-ping remove <name>` | Remove an endpoint |
| `api-ping monitor` | Start monitoring (runs continuously) |
| `api-ping status` | Show current status of all endpoints |
| `api-ping status-page` | Generate static HTML status page |
| `api-ping logs` | Show check history |

## Configuration

Config file: `~/.api-ping.yaml` (or set `APIPING_CONFIG` env var)

```yaml
endpoints:
  - name: "Production API"
    url: "https://api.example.com/health"
    method: GET
    headers:
      Authorization: "Bearer xxx"
    interval: 60          # seconds between checks
    timeout: 10           # request timeout in seconds
    expected_status: 200
    expected_body: "ok"   # optional: check response body
    max_duration: 3000    # optional: alert if response > 3000ms
    retries: 2            # optional: retry on failure
    retry_delay: 3        # optional: seconds between retries

notifications:
  telegram:
    bot_token: "BOT_TOKEN"
    chat_id: "CHAT_ID"
  discord:
    webhook_url: "https://discord.com/api/webhooks/xxx"
  webhook:
    url: "https://your-server.com/webhook"
    method: POST
  on:
    - down       # alert when endpoint goes down
    - recovered  # alert when endpoint recovers
    - slow       # alert when response is slow

db_path: "api-ping.db"  # SQLite database path
```

## Status Page

Generate a static HTML status page:

```bash
api-ping status-page -o index.html
```

Deploy to GitHub Pages, Netlify, or any static host. The page shows:
- Overall system status
- Per-endpoint status with uptime percentages
- Response times and last check time

## Notifications

### Telegram
1. Create a bot with [@BotFather](https://t.me/BotFather)
2. Get your chat ID (message [@userinfobot](https://t.me/userinfobot))
3. Add to config

### Discord
1. Create a webhook in your server settings → Integrations → Webhooks
2. Add webhook URL to config

### Webhook
Custom webhook receives JSON:
```json
{
  "event": "down",
  "endpoint": "Production API",
  "url": "https://api.example.com/health",
  "status_code": 500,
  "duration_ms": 234,
  "success": false,
  "error": "expected status 200, got 500",
  "timestamp": "2026-03-28T08:00:00Z"
}
```

## Example Output

```
[08:00:01] ✓ JSONPlaceholder | 200 | 234ms | https://jsonplaceholder.typicode.com/posts/1
[08:00:01] ✓ Google DNS | 200 | 45ms | https://dns.google/resolve?name=example.com
[08:01:01] ~ Slow API | 200 | 3521ms | https://api.example.com/health
[08:01:31] ✗ Slow API | 503 | 10020ms | https://api.example.com/health
```

## Contributing

1. Fork the repo
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Commit your changes (`git commit -m "feat: add my feature"`)
4. Push to the branch (`git push origin feature/my-feature`)
5. Open a Pull Request

## License

MIT — see [LICENSE](LICENSE) for details.
