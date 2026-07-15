// Package notify delivers incident notifications to configured integrations
// (Telegram, Slack, email, custom webhook). A Dispatcher loads the enabled
// integrations for an event and fans the message out to each channel's Sender,
// recording every attempt in the notification log.
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrNoSender is returned when no sender is registered for an integration type.
var ErrNoSender = errors.New("no sender registered for integration type")

// Event is the kind of incident transition being notified.
type Event string

const (
	EventIncidentStart   Event = "incident_start"
	EventIncidentResolve Event = "incident_resolve"
)

// Message is the payload handed to each Sender.
type Message struct {
	Event       Event
	IncidentID  int64
	ServiceID   string
	ServiceName string
	ServiceURL  string
	StartedAt   time.Time
	ResolvedAt  *time.Time // set for resolve events
}

// Down reports whether the message is a service-down (start) event.
func (m Message) Down() bool { return m.Event == EventIncidentStart }

// Subject is a short one-line summary suitable for an email subject / message title.
func (m Message) Subject() string {
	if m.Down() {
		return "🔴 " + m.ServiceName + " is DOWN"
	}
	return "🟢 " + m.ServiceName + " has RECOVERED"
}

// Body is a plain-text description of the event.
func (m Message) Body() string {
	if m.Down() {
		return m.ServiceName + " (" + m.ServiceURL + ") went down at " +
			m.StartedAt.UTC().Format(time.RFC1123) + "."
	}
	when := m.StartedAt
	if m.ResolvedAt != nil {
		when = *m.ResolvedAt
	}
	return m.ServiceName + " (" + m.ServiceURL + ") recovered at " +
		when.UTC().Format(time.RFC1123) + "."
}

// Sender delivers a message to one channel. config is the integration's stored
// JSON blob (shape is sender-specific).
type Sender interface {
	Send(ctx context.Context, config json.RawMessage, msg Message) error
}

// senders is the registry of channel implementations, populated by each
// sender file's init(). Empty until the concrete senders are registered.
var senders = map[string]Sender{}

func register(kind string, s Sender) { senders[kind] = s }

// SenderFor returns the registered sender for a channel type, if any.
func SenderFor(kind string) (Sender, bool) {
	s, ok := senders[kind]
	return s, ok
}
