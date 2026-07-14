package api

import (
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
