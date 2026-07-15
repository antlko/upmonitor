package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type telegramConfig struct {
	BotToken string `json:"botToken"`
	ChatID   string `json:"chatId"`
}

type telegramSender struct{}

// telegramAPIBase is overridable in tests.
var telegramAPIBase = "https://api.telegram.org"

func init() { register("telegram", telegramSender{}) }

func (telegramSender) Send(ctx context.Context, raw json.RawMessage, msg Message) error {
	var cfg telegramConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return errors.New("telegram: botToken and chatId are required")
	}
	body, _ := json.Marshal(map[string]string{
		"chat_id": cfg.ChatID,
		"text":    msg.Subject() + "\n\n" + msg.Body(),
	})
	url := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBase, cfg.BotToken)
	return postJSON(ctx, url, nil, body)
}
