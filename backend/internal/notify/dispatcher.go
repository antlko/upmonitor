package notify

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"upmonitor/internal/db"
)

// Dispatcher fans incident notifications out to the enabled integrations.
type Dispatcher struct {
	db *db.DB
}

// NewDispatcher builds a dispatcher backed by database.
func NewDispatcher(database *db.DB) *Dispatcher {
	return &Dispatcher{db: database}
}

// Notify delivers msg to every enabled integration whose type has a registered
// sender, logging each attempt. It is nil-safe and blocks until all sends
// complete (callers that must not block should invoke it in a goroutine).
func (d *Dispatcher) Notify(ctx context.Context, msg Message) {
	if d == nil || d.db == nil {
		return
	}
	integrations, err := d.db.ListEnabledIntegrations()
	if err != nil {
		slog.Error("notify: list integrations", "error", err)
		return
	}
	var wg sync.WaitGroup
	for _, in := range integrations {
		// Warnings are opt-in per channel: a flapping service would otherwise
		// page everyone every cycle for something that is still up.
		if msg.Event == EventWarning && !in.NotifyWarnings {
			continue
		}
		sender, ok := SenderFor(in.Type)
		if !ok {
			continue
		}
		wg.Add(1)
		go func(in db.Integration, sender Sender) {
			defer wg.Done()
			d.deliver(ctx, sender, in, msg)
		}(in, sender)
	}
	wg.Wait()
}

func (d *Dispatcher) deliver(ctx context.Context, sender Sender, in db.Integration, msg Message) {
	status, errMsg := "sent", ""
	if err := sender.Send(ctx, in.Config, msg); err != nil {
		status, errMsg = "failed", err.Error()
		slog.Warn("notify: delivery failed", "integration", in.ID, "type", in.Type, "error", err)
	}
	var incID *int64
	if msg.IncidentID != 0 {
		incID = &msg.IncidentID
	}
	if err := d.db.LogNotification(in.ID, incID, string(msg.Event), status, errMsg, time.Now().Unix()); err != nil {
		slog.Error("notify: log attempt", "error", err)
	}
}

// Test delivers msg through one integration synchronously, returning the send
// error (used by the "send test notification" endpoint). It does not log.
func (d *Dispatcher) Test(ctx context.Context, in db.Integration, msg Message) error {
	sender, ok := SenderFor(in.Type)
	if !ok {
		return ErrNoSender
	}
	return sender.Send(ctx, in.Config, msg)
}
