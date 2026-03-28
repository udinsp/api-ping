# api-ping 🏓

Self-hosted API uptime monitor with Telegram & Discord notifications.

Simple, lightweight, and built in Go — no Docker required.

## Features

- 🔍 Monitor multiple HTTP endpoints
- ⏱️ Configurable check intervals per endpoint
- 📊 Response time & status tracking
- 💾 SQLite storage for history
- 🔔 Notifications via Telegram, Discord, or Webhook
- 📈 Uptime percentage calculation
- 🚀 Single binary, cross-platform

## Install

```bash
go install github.com/trioplanet/api-ping@latest
```

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
api-ping add https://jsonplaceholder.typicode.com/posts/1 --name "Test API" --interval 60

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
| `api-ping logs` | Show check history |
| `api-ping version` | Show version |

## Configuration

Config file: `~/.api-ping.yaml` (or set `APIPING_CONFIG` env var)

```yaml
endpoints:
  - name: "Production API"
    url: "https://api.example.com/health"
    method: GET
    headers:
      Authorization: "Bearer xxx"
    interval: 60        # seconds between checks
    timeout: 10         # request timeout in seconds
    expected_status: 200
    expected_body: "ok" # optional: check response body
    expected_body: "ok" # optional: check response body

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

db_path: "api-ping.db"  # SQLite database path
```

## Notifications

### Telegram
1. Create a bot with [@BotFather](https://t.me/BotFather)
2. Get your chat ID
3. Add to config

### Discord
1. Create a webhook in your server settings
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
[08:01:01] ✓ JSONPlaceholder | 200 | 189ms | https://jsonplaceholder.typicode.com/posts/1
[08:01:01] ✗ GitHub API | 503 | 1002ms | https://api.github.com
```

## License

MIT
