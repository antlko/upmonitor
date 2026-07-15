package notify

import (
	"context"
	"encoding/json"
	"errors"
)

type slackConfig struct {
	WebhookURL string `json:"webhookUrl"`
}

type slackSender struct{}

func init() { register("slack", slackSender{}) }

func (slackSender) Send(ctx context.Context, raw json.RawMessage, msg Message) error {
	var cfg slackConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	if cfg.WebhookURL == "" {
		return errors.New("slack: webhookUrl is required")
	}
	body, _ := json.Marshal(map[string]string{"text": msg.Subject() + "\n" + msg.Body()})
	return postJSON(ctx, cfg.WebhookURL, nil, body)
}
