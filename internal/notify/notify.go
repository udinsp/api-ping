package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/trioplanet/api-ping/internal/checker"
	"github.com/trioplanet/api-ping/internal/config"
)

type Notifier interface {
	Send(event string, result checker.Result) error
}

func NotifyAll(cfg config.Notifications, event string, result checker.Result) {
	if !cfg.ShouldNotify(event) {
		return
	}

	if cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
		if err := sendTelegram(cfg.Telegram, event, result); err != nil {
			fmt.Printf("[notify] telegram error: %v\n", err)
		}
	}

	if cfg.Discord.WebhookURL != "" {
		if err := sendDiscord(cfg.Discord, event, result); err != nil {
			fmt.Printf("[notify] discord error: %v\n", err)
		}
	}

	if cfg.Webhook.URL != "" {
		if err := sendWebhook(cfg.Webhook, event, result); err != nil {
			fmt.Printf("[notify] webhook error: %v\n", err)
		}
	}
}

func sendTelegram(tg config.TelegramConfig, event string, result checker.Result) error {
	icon := "🟢"
	if event == "down" {
		icon = "🔴"
	} else if event == "recovered" {
		icon = "🟡"
	}

	msg := fmt.Sprintf("%s *api-ping* | %s\n\n"+
		"**Endpoint:** %s\n"+
		"**URL:** `%s`\n"+
		"**Status:** %d\n"+
		"**Duration:** %v\n",
		icon, event,
		result.Endpoint.Name,
		result.Endpoint.URL,
		result.StatusCode,
		result.Duration.Round(time.Millisecond),
	)

	if result.Error != "" {
		msg += fmt.Sprintf("**Error:** `%s`\n", result.Error)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tg.BotToken)
	body, _ := json.Marshal(map[string]string{
		"chat_id":    tg.ChatID,
		"text":       msg,
		"parse_mode": "Markdown",
	})

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram returned %d", resp.StatusCode)
	}
	return nil
}

func sendDiscord(dc config.DiscordConfig, event string, result checker.Result) error {
	color := 0x00ff00 // green
	if event == "down" {
		color = 0xff0000 // red
	} else if event == "recovered" {
		color = 0xffff00 // yellow
	}

	embed := map[string]interface{}{
		"title":  fmt.Sprintf("api-ping | %s", event),
		"color":  color,
		"fields": []map[string]interface{}{
			{"name": "Endpoint", "value": result.Endpoint.Name, "inline": true},
			{"name": "Status", "value": fmt.Sprintf("%d", result.StatusCode), "inline": true},
			{"name": "Duration", "value": result.Duration.Round(time.Millisecond).String(), "inline": true},
			{"name": "URL", "value": result.Endpoint.URL, "inline": false},
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if result.Error != "" {
		embed["fields"] = append(embed["fields"].([]map[string]interface{}),
			map[string]interface{}{"name": "Error", "value": result.Error, "inline": false},
		)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"embeds": []interface{}{embed},
	})

	resp, err := http.Post(dc.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf("discord returned %d", resp.StatusCode)
	}
	return nil
}

func sendWebhook(wh config.WebhookConfig, event string, result checker.Result) error {
	method := wh.Method
	if method == "" {
		method = "POST"
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"event":       event,
		"endpoint":    result.Endpoint.Name,
		"url":         result.Endpoint.URL,
		"status_code": result.StatusCode,
		"duration_ms": result.Duration.Milliseconds(),
		"success":     result.Success,
		"error":       result.Error,
		"timestamp":   time.Now().Format(time.RFC3339),
	})

	req, err := http.NewRequest(method, wh.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
