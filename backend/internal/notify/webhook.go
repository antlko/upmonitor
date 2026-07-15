package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"text/template"
)

type webhookConfig struct {
	URL          string            `json:"url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers"`
	BodyTemplate string            `json:"bodyTemplate"`
}

type webhookSender struct{}

func init() { register("webhook", webhookSender{}) }

func (webhookSender) Send(ctx context.Context, raw json.RawMessage, msg Message) error {
	var cfg webhookConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	if cfg.URL == "" {
		return errors.New("webhook: url is required")
	}
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodPost
	}
	body, err := renderWebhookBody(cfg.BodyTemplate, msg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, bytes.NewReader([]byte(body)))
	if err != nil {
		return err
	}
	if cfg.BodyTemplate == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	return doRequest(req)
}

// renderWebhookBody renders the user template against the message, or emits a
// default JSON payload when no template is configured.
func renderWebhookBody(tmpl string, msg Message) (string, error) {
	if strings.TrimSpace(tmpl) == "" {
		status := "down"
		if !msg.Down() {
			status = "recovered"
		}
		b, _ := json.Marshal(map[string]any{
			"event":   string(msg.Event),
			"service": msg.ServiceName,
			"url":     msg.ServiceURL,
			"status":  status,
			"message": msg.Body(),
		})
		return string(b), nil
	}
	t, err := template.New("webhook").Option("missingkey=zero").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, msg); err != nil {
		return "", err
	}
	return buf.String(), nil
}
