package api

import (
	"encoding/json"
	"time"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
)

// Wire formats (camelCase) mirroring the frontend's TypeScript types.

type layoutDTO struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type checkDTO struct {
	Interval       int    `json:"interval"`
	Method         string `json:"method"`
	Timeout        int    `json:"timeout"`
	ExpectedStatus []int  `json:"expectedStatus"`
}

type widgetDTO struct {
	Mode string `json:"mode"`
}

type serviceDTO struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	Icon           *string   `json:"icon"`
	Check          checkDTO  `json:"check"`
	Widget         widgetDTO `json:"widget"`
	Layout         layoutDTO `json:"layout"`
	Status         string    `json:"status"`
	LatencyMs      *int      `json:"latencyMs"`
	Uptime         float64   `json:"uptime"`
	ErrorCount     int       `json:"errorCount"`
	LastCheck      *string   `json:"lastCheck"`
	LastSuccess    *string   `json:"lastSuccess"`
	LatencyHistory []int     `json:"latencyHistory"`
}

func isoPtr(ts *int64) *string {
	if ts == nil {
		return nil
	}
	s := time.Unix(*ts, 0).UTC().Format(time.RFC3339)
	return &s
}

func iso(ts int64) string {
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

// toServiceDTO combines a service definition with its runtime metrics.
func toServiceDTO(svc config.Service, m *db.ServiceMetrics) serviceDTO {
	var icon *string
	if svc.Icon != "" {
		u := "/images/" + svc.Icon
		icon = &u
	}
	expected := svc.Check.ExpectedStatus
	if expected == nil {
		expected = []int{}
	}
	dto := serviceDTO{
		ID:   svc.ID,
		Name: svc.Name,
		URL:  svc.URL,
		Icon: icon,
		Check: checkDTO{
			Interval:       svc.Check.Interval,
			Method:         svc.Check.Method,
			Timeout:        svc.Check.Timeout,
			ExpectedStatus: expected,
		},
		Widget:         widgetDTO{Mode: svc.Widget.Mode},
		Layout:         layoutDTO{X: svc.Layout.X, Y: svc.Layout.Y, W: svc.Layout.W, H: svc.Layout.H},
		Status:         db.StatusUnknown,
		Uptime:         0,
		LatencyHistory: []int{},
	}
	if m != nil {
		dto.Status = m.Status
		dto.LatencyMs = m.LatencyMs
		dto.Uptime = m.Uptime
		dto.ErrorCount = m.ErrorCount
		dto.LastCheck = isoPtr(m.LastCheck)
		dto.LastSuccess = isoPtr(m.LastSuccess)
		if m.History != nil {
			dto.LatencyHistory = m.History
		}
	}
	return dto
}

type tlsDTO struct {
	CheckedAt  string  `json:"checkedAt"`
	ValidFrom  *string `json:"validFrom"`
	ValidUntil *string `json:"validUntil"`
	Issuer     string  `json:"issuer"`
	Subject    string  `json:"subject"`
	DaysLeft   *int    `json:"daysLeft"`
	Error      string  `json:"error"`
}

func toTLSDTO(t *db.ServiceTLS) *tlsDTO {
	if t == nil {
		return nil
	}
	dto := tlsDTO{
		CheckedAt: iso(t.CheckedAt),
		Issuer:    t.Issuer,
		Subject:   t.Subject,
		Error:     t.Error,
	}
	if t.ValidFrom > 0 {
		dto.ValidFrom = isoPtr(&t.ValidFrom)
	}
	if t.ValidUntil > 0 {
		dto.ValidUntil = isoPtr(&t.ValidUntil)
		days := int(time.Until(time.Unix(t.ValidUntil, 0)).Hours() / 24)
		dto.DaysLeft = &days
	}
	return &dto
}

type uptimeWindowsDTO struct {
	Days7   float64 `json:"days7"`
	Days30  float64 `json:"days30"`
	Days365 float64 `json:"days365"`
}

type settingsDTO struct {
	PublicDashboard   bool             `json:"publicDashboard"`
	DefaultWidgetMode string           `json:"defaultWidgetMode"`
	Theme             string           `json:"theme"`
	Check             checkSettingsDTO `json:"check"`
	ConfigDir         string           `json:"configDir"`
}

type checkSettingsDTO struct {
	DefaultInterval int `json:"defaultInterval"`
	Timeout         int `json:"timeout"`
	RetentionDays   int `json:"retentionDays"`
}

func toSettingsDTO(c *config.Config, dir string) settingsDTO {
	return settingsDTO{
		PublicDashboard:   c.Settings.PublicDashboard,
		DefaultWidgetMode: c.Settings.DefaultWidgetMode,
		Theme:             c.Settings.Theme,
		Check: checkSettingsDTO{
			DefaultInterval: c.Settings.Check.DefaultInterval,
			Timeout:         c.Settings.Check.Timeout,
			RetentionDays:   c.Settings.Check.RetentionDays,
		},
		ConfigDir: dir,
	}
}

type userDTO struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
}

func toUserDTO(u *db.User) userDTO {
	return userDTO{ID: u.ID, Username: u.Username, Role: u.Role, CreatedAt: iso(u.CreatedAt)}
}

type incidentDTO struct {
	ID          int64   `json:"id"`
	ServiceID   string  `json:"serviceId"`
	ServiceName string  `json:"serviceName"`
	Status      string  `json:"status"`
	Source      string  `json:"source"`
	Title       string  `json:"title"`
	StartedAt   string  `json:"startedAt"`
	ResolvedAt  *string `json:"resolvedAt"`
	CreatedBy   *int64  `json:"createdBy"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

type incidentCommentDTO struct {
	ID         int64  `json:"id"`
	IncidentID int64  `json:"incidentId"`
	Username   string `json:"username"`
	Body       string `json:"body"`
	CreatedAt  string `json:"createdAt"`
}

type incidentDetailDTO struct {
	incidentDTO
	Comments []incidentCommentDTO `json:"comments"`
}

func toIncidentDTO(inc db.Incident, serviceName string) incidentDTO {
	return incidentDTO{
		ID:          inc.ID,
		ServiceID:   inc.ServiceID,
		ServiceName: serviceName,
		Status:      inc.Status,
		Source:      inc.Source,
		Title:       inc.Title,
		StartedAt:   iso(inc.StartedAt),
		ResolvedAt:  isoPtr(inc.ResolvedAt),
		CreatedBy:   inc.CreatedBy,
		CreatedAt:   iso(inc.CreatedAt),
		UpdatedAt:   iso(inc.UpdatedAt),
	}
}

func toIncidentCommentDTO(cm db.IncidentComment) incidentCommentDTO {
	return incidentCommentDTO{
		ID:         cm.ID,
		IncidentID: cm.IncidentID,
		Username:   cm.Username,
		Body:       cm.Body,
		CreatedAt:  iso(cm.CreatedAt),
	}
}

type integrationDTO struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Enabled   bool            `json:"enabled"`
	Config    json.RawMessage `json:"config"`  // secret fields removed
	Secrets   map[string]bool `json:"secrets"` // secret field → whether it's set
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
}

// toIntegrationDTO redacts secret fields from the stored config, exposing only
// whether each secret is set so the UI can show a "configured" placeholder.
func toIntegrationDTO(in db.Integration) integrationDTO {
	cfg := map[string]any{}
	_ = json.Unmarshal(in.Config, &cfg)
	secrets := map[string]bool{}
	for _, field := range secretFields[in.Type] {
		v, ok := cfg[field].(string)
		secrets[field] = ok && v != ""
		delete(cfg, field)
	}
	redacted, _ := json.Marshal(cfg)
	return integrationDTO{
		ID:        in.ID,
		Type:      in.Type,
		Name:      in.Name,
		Enabled:   in.Enabled,
		Config:    redacted,
		Secrets:   secrets,
		CreatedAt: iso(in.CreatedAt),
		UpdatedAt: iso(in.UpdatedAt),
	}
}
