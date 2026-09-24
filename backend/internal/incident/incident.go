// Package incident derives outage incidents from service status transitions.
// It is deliberately independent of internal/monitor (which imports it) to
// avoid an import cycle: the scheduler calls OnTransition after each check.
package incident

import (
	"context"
	"log/slog"
	"time"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
	"upmonitor/internal/notify"
)

// InitialStatus seeds a worker's "previous status" when it starts. If the
// service already has an ongoing incident (e.g. the process restarted mid-
// outage) it starts offline, so the next successful check resolves it; otherwise
// it starts online, so a first failing check opens an incident but a first
// successful check does not spuriously resolve anything.
func InitialStatus(database *db.DB, serviceID string) string {
	inc, err := database.GetOngoingIncident(serviceID)
	if err != nil {
		slog.Error("incident: seed status", "service", serviceID, "error", err)
		return db.StatusOnline
	}
	if inc != nil {
		return db.StatusOffline
	}
	return db.StatusOnline
}

// Outcome is what a completed check cycle decided. It carries the attempt count
// and first-failure reason so a warning notification can explain itself without
// this package importing monitor (which would be an import cycle).
type Outcome struct {
	Status   string
	Attempts int
	Error    string
}

// OnTransition reacts to a status change. The rule it encodes is a single line:
// an incident is open exactly while the service is offline, and a warning
// counts as up. So →offline opens one, →online and →warning both resolve the
// ongoing one, and a →warning that had nothing to resolve fires a warning
// notification instead. It is a no-op when the status is unchanged, which is
// what stops a service that keeps blipping from notifying every cycle.
// dispatcher may be nil (notifications are then skipped).
func OnTransition(ctx context.Context, database *db.DB, dispatcher *notify.Dispatcher, svc config.Service, previous string, cur Outcome, ts int64) {
	if previous == cur.Status {
		return
	}
	switch cur.Status {
	case db.StatusOffline:
		inc, err := database.CreateIncident(svc.ID, "auto", ts, nil, nil)
		if err != nil {
			// A concurrent check may have opened it first (unique index) — fine.
			slog.Debug("incident: open", "service", svc.ID, "error", err)
			return
		}
		slog.Info("incident: opened", "service", svc.ID, "incident", inc.ID)
		fire(dispatcher, notify.EventIncidentStart, svc, inc)
	case db.StatusOnline, db.StatusWarning:
		inc, err := database.ResolveOngoingIncident(svc.ID, ts)
		if err != nil {
			slog.Error("incident: resolve", "service", svc.ID, "error", err)
			return
		}
		if inc != nil {
			// Coming back from an outage — even via a warning — is a recovery.
			// One message per event: the resolve already says we are back.
			slog.Info("incident: resolved", "service", svc.ID, "incident", inc.ID)
			fire(dispatcher, notify.EventIncidentResolve, svc, inc)
			return
		}
		if cur.Status == db.StatusWarning {
			slog.Info("monitor: check recovered on retry", "service", svc.ID,
				"attempts", cur.Attempts, "first_error", cur.Error)
			fireWarning(dispatcher, svc, cur, ts)
			// Recorded already-resolved so it shows up in the incident feed
			// (e.g. a service's "Recent incidents") without ever touching the
			// one-ongoing-incident-per-service invariant above.
			if _, err := database.CreateWarningEvent(svc.ID, ts); err != nil {
				slog.Error("incident: record warning event", "service", svc.ID, "error", err)
			}
		}
	}
}

// fireWarning dispatches a warning notification. It cannot reuse fire: there is
// no incident to reference.
func fireWarning(dispatcher *notify.Dispatcher, svc config.Service, cur Outcome, ts int64) {
	if dispatcher == nil {
		return
	}
	msg := notify.Message{
		Event:       notify.EventWarning,
		ServiceID:   svc.ID,
		ServiceName: svc.Name,
		ServiceURL:  svc.URL,
		StartedAt:   time.Unix(ts, 0),
		Attempts:    cur.Attempts,
		Error:       cur.Error,
	}
	go dispatcher.Notify(context.Background(), msg)
}

// fire builds a notification message and dispatches it without blocking the
// caller (the scheduler's check goroutine).
func fire(dispatcher *notify.Dispatcher, event notify.Event, svc config.Service, inc *db.Incident) {
	if dispatcher == nil || inc == nil {
		return
	}
	msg := notify.Message{
		Event:       event,
		IncidentID:  inc.ID,
		ServiceID:   svc.ID,
		ServiceName: svc.Name,
		ServiceURL:  svc.URL,
		StartedAt:   time.Unix(inc.StartedAt, 0),
	}
	if inc.ResolvedAt != nil {
		t := time.Unix(*inc.ResolvedAt, 0)
		msg.ResolvedAt = &t
	}
	go dispatcher.Notify(context.Background(), msg)
}
