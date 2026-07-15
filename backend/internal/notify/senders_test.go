package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func sampleMessage() Message {
	return Message{
		Event:       EventIncidentStart,
		IncidentID:  1,
		ServiceName: "API",
		ServiceURL:  "https://api.example.com",
		StartedAt:   time.Unix(1_700_000_000, 0),
	}
}

func TestSlackSender(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
	}))
	defer srv.Close()

	cfg, _ := json.Marshal(slackConfig{WebhookURL: srv.URL})
	if err := (slackSender{}).Send(context.Background(), cfg, sampleMessage()); err != nil {
		t.Fatalf("send: %v", err)
	}
	text, _ := gotBody["text"].(string)
	if !strings.Contains(text, "API") {
		t.Errorf("slack text missing service name: %q", text)
	}
}

func TestTelegramSender(t *testing.T) {
	var path string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
	}))
	defer srv.Close()

	old := telegramAPIBase
	telegramAPIBase = srv.URL
	defer func() { telegramAPIBase = old }()

	cfg, _ := json.Marshal(telegramConfig{BotToken: "tok123", ChatID: "42"})
	if err := (telegramSender{}).Send(context.Background(), cfg, sampleMessage()); err != nil {
		t.Fatalf("send: %v", err)
	}
	if path != "/bottok123/sendMessage" {
		t.Errorf("telegram path = %q", path)
	}
	if id, _ := gotBody["chat_id"].(string); id != "42" {
		t.Errorf("chat_id = %q, want 42", id)
	}
}

func TestWebhookSenderDefaultBody(t *testing.T) {
	var method, ctype string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		ctype = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
	}))
	defer srv.Close()

	cfg, _ := json.Marshal(webhookConfig{URL: srv.URL})
	if err := (webhookSender{}).Send(context.Background(), cfg, sampleMessage()); err != nil {
		t.Fatalf("send: %v", err)
	}
	if method != http.MethodPost {
		t.Errorf("method = %s, want POST", method)
	}
	if ctype != "application/json" {
		t.Errorf("content-type = %q", ctype)
	}
	if gotBody["service"] != "API" || gotBody["status"] != "down" {
		t.Errorf("unexpected default body: %+v", gotBody)
	}
}

func TestWebhookSenderTemplateAndMethod(t *testing.T) {
	var method, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer srv.Close()

	cfg, _ := json.Marshal(webhookConfig{
		URL:          srv.URL,
		Method:       "put",
		BodyTemplate: "{{.ServiceName}} is {{.Event}}",
		Headers:      map[string]string{"X-Token": "abc"},
	})
	if err := (webhookSender{}).Send(context.Background(), cfg, sampleMessage()); err != nil {
		t.Fatalf("send: %v", err)
	}
	if method != http.MethodPut {
		t.Errorf("method = %s, want PUT", method)
	}
	if body != "API is incident_start" {
		t.Errorf("rendered body = %q", body)
	}
}

func TestWebhookSenderNon2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg, _ := json.Marshal(webhookConfig{URL: srv.URL})
	if err := (webhookSender{}).Send(context.Background(), cfg, sampleMessage()); err == nil {
		t.Error("expected an error for a 500 response, got nil")
	}
}

func TestBuildEmailMessage(t *testing.T) {
	cfg := emailConfig{From: "bot@x.com", To: "ops@x.com"}
	out := string(buildMessage(cfg, sampleMessage()))
	for _, want := range []string{
		"From: bot@x.com\r\n",
		"To: ops@x.com\r\n",
		"Subject: ",
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n",
		"API",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("email message missing %q\n---\n%s", want, out)
		}
	}
}
