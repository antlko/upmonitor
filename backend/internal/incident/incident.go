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

// OnTransition reacts to a status change: online→offline opens an incident and
// fires a start notification; offline→online resolves the ongoing incident and
// fires a resolve notification. It is a no-op when the status is unchanged.
// dispatcher may be nil (notifications are then skipped).
func OnTransition(ctx context.Context, database *db.DB, dispatcher *notify.Dispatcher, svc config.Service, previous, current string, ts int64) {
	if previous == current {
		return
	}
	switch current {
	case db.StatusOffline:
		inc, err := database.CreateIncident(svc.ID, "auto", ts, nil, nil)
		if err != nil {
			// A concurrent check may have opened it first (unique index) — fine.
			slog.Debug("incident: open", "service", svc.ID, "error", err)
			return
		}
		slog.Info("incident: opened", "service", svc.ID, "incident", inc.ID)
		fire(dispatcher, notify.EventIncidentStart, svc, inc)
	case db.StatusOnline:
		inc, err := database.ResolveOngoingIncident(svc.ID, ts)
		if err != nil {
			slog.Error("incident: resolve", "service", svc.ID, "error", err)
			return
		}
		if inc == nil {
			return
		}
		slog.Info("incident: resolved", "service", svc.ID, "incident", inc.ID)
		fire(dispatcher, notify.EventIncidentResolve, svc, inc)
	}
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
