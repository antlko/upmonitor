package api

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"upmonitor/internal/db"
)

// serviceName resolves a service id to its display name, tolerating services
// that were deleted from config.yaml while their incidents remain in SQLite.
func (s *Server) serviceName(id string) string {
	if svc := s.config().Find(id); svc != nil {
		return svc.Name
	}
	return "(deleted service)"
}

// GET /api/incidents?status=&serviceId= → incident list (newest first).
func (s *Server) handleListIncidents(c fiber.Ctx) error {
	status := c.Query("status")
	if status != "" && status != "ongoing" && status != "resolved" {
		return fiber.NewError(fiber.StatusBadRequest, "status must be ongoing or resolved")
	}
	incidents, err := s.conn().ListIncidents(c.Query("serviceId"), status, 500, 0)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not load incidents")
	}
	out := make([]incidentDTO, 0, len(incidents))
	for _, inc := range incidents {
		out = append(out, toIncidentDTO(inc, s.serviceName(inc.ServiceID)))
	}
	return c.JSON(out)
}

// GET /api/incidents/:id → an incident with its comments.
func (s *Server) handleGetIncident(c fiber.Ctx) error {
	inc, err := s.incidentParam(c)
	if err != nil {
		return err
	}
	comments, err := s.conn().ListIncidentComments(inc.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not load comments")
	}
	out := incidentDetailDTO{incidentDTO: toIncidentDTO(*inc, s.serviceName(inc.ServiceID))}
	out.Comments = make([]incidentCommentDTO, 0, len(comments))
	for _, cm := range comments {
		out.Comments = append(out.Comments, toIncidentCommentDTO(cm))
	}
	return c.JSON(out)
}

type incidentInput struct {
	ServiceID  string  `json:"serviceId"`
	Title      string  `json:"title"`
	StartedAt  *string `json:"startedAt"`
	ResolvedAt *string `json:"resolvedAt"`
}

// POST /api/incidents → manually create an incident (admin).
func (s *Server) handleCreateIncident(c fiber.Ctx) error {
	var in incidentInput
	if err := decode(c, &in); err != nil {
		return err
	}
	if s.config().Find(in.ServiceID) == nil {
		return fiber.NewError(fiber.StatusBadRequest, "unknown service")
	}
	started, err := parseTime(in.StartedAt, time.Now())
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid startedAt")
	}
	var title *string
	if t := strings.TrimSpace(in.Title); t != "" {
		title = &t
	}
	var createdBy *int64
	if u := userLocal(c); u != nil {
		createdBy = &u.ID
	}
	inc, err := s.conn().CreateIncident(in.ServiceID, "manual", started.Unix(), title, createdBy)
	if err != nil {
		if errors.Is(err, db.ErrOngoingExists) {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		return fiber.NewError(fiber.StatusInternalServerError, "could not create incident")
	}
	// A manually created incident may already be resolved (logging a past outage).
	if in.ResolvedAt != nil {
		resolved, perr := parseTime(in.ResolvedAt, time.Now())
		if perr != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid resolvedAt")
		}
		inc, err = s.conn().ResolveIncident(inc.ID, resolved.Unix())
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "could not resolve incident")
		}
	}
	return c.Status(fiber.StatusCreated).JSON(toIncidentDTO(*inc, s.serviceName(inc.ServiceID)))
}

// PUT /api/incidents/:id → edit title/started/resolved (admin). Providing
// resolvedAt marks it resolved; omitting it keeps the current state.
func (s *Server) handleUpdateIncident(c fiber.Ctx) error {
	inc, err := s.incidentParam(c)
	if err != nil {
		return err
	}
	var in incidentInput
	if err := decode(c, &in); err != nil {
		return err
	}
	title := inc.Title
	if in.Title != "" {
		title = strings.TrimSpace(in.Title)
	}
	started := inc.StartedAt
	if in.StartedAt != nil {
		t, perr := parseTime(in.StartedAt, time.Unix(inc.StartedAt, 0))
		if perr != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid startedAt")
		}
		started = t.Unix()
	}
	resolved := inc.ResolvedAt
	if in.ResolvedAt != nil {
		t, perr := parseTime(in.ResolvedAt, time.Now())
		if perr != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid resolvedAt")
		}
		u := t.Unix()
		resolved = &u
	}
	updated, err := s.conn().UpdateIncident(inc.ID, title, started, resolved)
	if err != nil {
		if errors.Is(err, db.ErrOngoingExists) {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		return fiber.NewError(fiber.StatusInternalServerError, "could not update incident")
	}
	return c.JSON(toIncidentDTO(*updated, s.serviceName(updated.ServiceID)))
}

// DELETE /api/incidents/:id → remove an incident (admin).
func (s *Server) handleDeleteIncident(c fiber.Ctx) error {
	inc, err := s.incidentParam(c)
	if err != nil {
		return err
	}
	if err := s.conn().DeleteIncident(inc.ID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not delete incident")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /api/incidents/:id/comments → add a comment (any authenticated user).
func (s *Server) handleAddIncidentComment(c fiber.Ctx) error {
	inc, err := s.incidentParam(c)
	if err != nil {
		return err
	}
	var in struct {
		Body string `json:"body"`
	}
	if err := decode(c, &in); err != nil {
		return err
	}
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return fiber.NewError(fiber.StatusBadRequest, "comment body is required")
	}
	var userID *int64
	if u := userLocal(c); u != nil {
		userID = &u.ID
	}
	cm, err := s.conn().AddIncidentComment(inc.ID, userID, body, time.Now().Unix())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not add comment")
	}
	return c.Status(fiber.StatusCreated).JSON(toIncidentCommentDTO(*cm))
}

// incidentParam parses :id and loads the incident (404 if absent).
func (s *Server) incidentParam(c fiber.Ctx) (*db.Incident, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid incident id")
	}
	inc, err := s.conn().GetIncident(id)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "incident not found")
	}
	return inc, nil
}

// parseTime parses an optional RFC3339 timestamp, falling back to def when nil.
func parseTime(s *string, def time.Time) (time.Time, error) {
	if s == nil || *s == "" {
		return def, nil
	}
	return time.Parse(time.RFC3339, *s)
}
