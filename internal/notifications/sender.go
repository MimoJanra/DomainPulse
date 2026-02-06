package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MimoJanra/DomainPulse/internal/models"
)

type NotificationSender struct {
	client *http.Client
}

func NewNotificationSender() *NotificationSender {
	return &NotificationSender{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type NotificationMessage struct {
	CheckID      int
	DomainName   string
	CheckType    string
	Status       string
	ErrorMessage string
	DurationMS   int
	CreatedAt    string
}

func (ns *NotificationSender) SendNotification(settings models.NotificationSettings, msg NotificationMessage) error {
	if !settings.Enabled {
		return nil
	}

	switch settings.Type {
	case "telegram":
		return ns.sendTelegram(settings, msg)
	case "slack":
		return ns.sendSlack(settings, msg)
	default:
		return fmt.Errorf("unsupported notification type: %s", settings.Type)
	}
}

func (ns *NotificationSender) sendTelegram(settings models.NotificationSettings, msg NotificationMessage) error {
	if settings.Token == "" || settings.ChatID == "" {
		return fmt.Errorf("telegram token and chat_id are required")
	}

	text := ns.formatTelegramMessage(msg)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", settings.Token)

	payload := map[string]interface{}{
		"chat_id":    settings.ChatID,
		"text":       text,
		"parse_mode": "HTML",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ns.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

func (ns *NotificationSender) sendSlack(settings models.NotificationSettings, msg NotificationMessage) error {
	if settings.WebhookURL == "" {
		return fmt.Errorf("slack webhook_url is required")
	}

	text := ns.formatSlackMessage(msg)
	payload := map[string]interface{}{
		"text": text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequest("POST", settings.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ns.client.Do(req)
	if err != nil {
		return fmt.Errorf("send slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (ns *NotificationSender) formatTelegramMessage(msg NotificationMessage) string {
	statusEmoji := "✅"
	if msg.Status == "error" || msg.Status == "timeout" {
		statusEmoji = "❌"
	} else if msg.Status == "slow_response" {
		statusEmoji = "⚠️"
	}

	text := fmt.Sprintf("<b>%s Domain Check</b>\n\n", statusEmoji)
	text += fmt.Sprintf("<b>Domain:</b> %s\n", msg.DomainName)
	text += fmt.Sprintf("<b>Type:</b> %s\n", msg.CheckType)
	text += fmt.Sprintf("<b>Status:</b> %s\n", msg.Status)
	text += fmt.Sprintf("<b>Duration:</b> %d ms\n", msg.DurationMS)

	if msg.ErrorMessage != "" {
		text += fmt.Sprintf("<b>Error:</b> %s\n", msg.ErrorMessage)
	}

	text += fmt.Sprintf("<b>Time:</b> %s", msg.CreatedAt)

	return text
}

func (ns *NotificationSender) formatSlackMessage(msg NotificationMessage) string {
	statusEmoji := "✅"
	if msg.Status == "error" || msg.Status == "timeout" {
		statusEmoji = "❌"
	} else if msg.Status == "slow_response" {
		statusEmoji = "⚠️"
	}

	text := fmt.Sprintf("%s *Domain Check Report*\n\n", statusEmoji)
	text += fmt.Sprintf("*Domain:* %s\n", msg.DomainName)
	text += fmt.Sprintf("*Type:* %s\n", msg.CheckType)
	text += fmt.Sprintf("*Status:* %s\n", msg.Status)
	text += fmt.Sprintf("*Duration:* %d ms\n", msg.DurationMS)

	if msg.ErrorMessage != "" {
		text += fmt.Sprintf("*Error:* %s\n", msg.ErrorMessage)
	}

	text += fmt.Sprintf("*Time:* %s", msg.CreatedAt)

	return text
}
