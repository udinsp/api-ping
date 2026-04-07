package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Endpoint struct {
	Name           string            `yaml:"name"`
	URL            string            `yaml:"url"`
	Method         string            `yaml:"method,omitempty"`
	Headers        map[string]string `yaml:"headers,omitempty"`
	Body           string            `yaml:"body,omitempty"`
	Interval       int               `yaml:"interval,omitempty"`
	Timeout        int               `yaml:"timeout,omitempty"`
	ExpectedStatus int               `yaml:"expected_status,omitempty"`
	ExpectedBody   string            `yaml:"expected_body,omitempty"`
	MaxDuration    int               `yaml:"max_duration,omitempty"` // max response time in ms
	Retries        int               `yaml:"retries,omitempty"`      // retries on failure
	RetryDelay     int               `yaml:"retry_delay,omitempty"`  // delay between retries in seconds
}

func (e Endpoint) GetMethod() string {
	if e.Method == "" {
		return "GET"
	}
	return e.Method
}

func (e Endpoint) GetTimeout() time.Duration {
	if e.Timeout <= 0 {
		return 10 * time.Second
	}
	return time.Duration(e.Timeout) * time.Second
}

func (e Endpoint) GetExpectedStatus() int {
	if e.ExpectedStatus <= 0 {
		return 200
	}
	return e.ExpectedStatus
}

func (e Endpoint) GetInterval() time.Duration {
	if e.Interval <= 0 {
		return 60 * time.Second
	}
	return time.Duration(e.Interval) * time.Second
}

func (e Endpoint) GetMaxDuration() time.Duration {
	if e.MaxDuration <= 0 {
		return 0 // no limit
	}
	return time.Duration(e.MaxDuration) * time.Millisecond
}

type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

type DiscordConfig struct {
	WebhookURL string `yaml:"webhook_url"`
}

type WebhookConfig struct {
	URL    string `yaml:"url"`
	Method string `yaml:"method,omitempty"`
}

type Notifications struct {
	Telegram TelegramConfig `yaml:"telegram,omitempty"`
	Discord  DiscordConfig  `yaml:"discord,omitempty"`
	Webhook  WebhookConfig  `yaml:"webhook,omitempty"`
	Events   []string       `yaml:"on,omitempty"`
}

func (n Notifications) ShouldNotify(event string) bool {
	if len(n.Events) == 0 {
		return true
	}
	for _, e := range n.Events {
		if e == event || e == "all" {
			return true
		}
	}
	return false
}

type Config struct {
	Endpoints      []Endpoint      `yaml:"endpoints"`
	Notifications  Notifications   `yaml:"notifications,omitempty"`
	DBPath         string          `yaml:"db_path,omitempty"`
	RetentionDays  int             `yaml:"retention_days,omitempty"`
	HealthServer   HealthServerCfg `yaml:"health_server,omitempty"`
}

type HealthServerCfg struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	Port    int    `yaml:"port,omitempty"`
	Bind    string `yaml:"bind,omitempty"`
}

func (h HealthServerCfg) GetPort() int {
	if h.Port <= 0 {
		return 8080
	}
	return h.Port
}

func (h HealthServerCfg) GetBind() string {
	if h.Bind == "" {
		return "0.0.0.0"
	}
	return h.Bind
}

func (c Config) GetDBPath() string {
	if c.DBPath == "" {
		return "api-ping.db"
	}
	return c.DBPath
}

func (c Config) GetRetentionDays() int {
	if c.RetentionDays <= 0 {
		return 90
	}
	return c.RetentionDays
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func DefaultConfig() *Config {
	return &Config{
		Endpoints: []Endpoint{},
		Notifications: Notifications{
			Events: []string{"down", "recovered"},
		},
		DBPath:        "api-ping.db",
		RetentionDays: 90,
	}
}

func (e Endpoint) GetRetries() int {
	if e.Retries < 0 {
		return 0
	}
	return e.Retries
}

func (e Endpoint) GetRetryDelay() time.Duration {
	if e.RetryDelay <= 0 {
		return 1 * time.Second
	}
	return time.Duration(e.RetryDelay) * time.Second
}
