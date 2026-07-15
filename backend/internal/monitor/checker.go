// Package monitor performs HTTP health checks and schedules them per service.
package monitor

import (
	"context"
	"io"
	"net/http"
	"time"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
)

// Result is the outcome of a single health check.
type Result struct {
	Status     string
	LatencyMs  *int
	StatusCode *int
	Error      string
	TLS        *TLSInfo // populated for HTTPS responses with a presented certificate
}

// TLSInfo is the leaf-certificate metadata read from a successful TLS handshake.
type TLSInfo struct {
	NotBefore time.Time
	NotAfter  time.Time
	Issuer    string
	Subject   string
}

// Check performs one HTTP request for svc and classifies the result.
// A service is "online" when the response status is in ExpectedStatus (or any
// 2xx when ExpectedStatus is empty); otherwise "offline".
func Check(ctx context.Context, client *http.Client, svc config.Service) Result {
	method := svc.Check.Method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, svc.URL, nil)
	if err != nil {
		return Result{Status: db.StatusOffline, Error: err.Error()}
	}
	req.Header.Set("User-Agent", "upmonitor/1.0 (+health-check)")

	start := time.Now()
	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return Result{Status: db.StatusOffline, LatencyMs: &latency, Error: err.Error()}
	}
	defer resp.Body.Close()
	// Drain a little of the body so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	code := resp.StatusCode
	res := Result{LatencyMs: &latency, StatusCode: &code}
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		res.TLS = &TLSInfo{
			NotBefore: cert.NotBefore,
			NotAfter:  cert.NotAfter,
			Issuer:    certName(cert.Issuer.CommonName, cert.Issuer.String()),
			Subject:   certName(cert.Subject.CommonName, cert.Subject.String()),
		}
	}
	if statusMatches(code, svc.Check.ExpectedStatus) {
		res.Status = db.StatusOnline
	} else {
		res.Status = db.StatusOffline
		res.Error = http.StatusText(code)
		if res.Error == "" {
			res.Error = "unexpected status"
		}
	}
	return res
}

// certName prefers the certificate CommonName, falling back to the full
// distinguished name when the CN is empty.
func certName(cn, full string) string {
	if cn != "" {
		return cn
	}
	return full
}

// statusMatches reports whether code is acceptable. An empty expected list
// accepts any 2xx.
func statusMatches(code int, expected []int) bool {
	if len(expected) == 0 {
		return code >= 200 && code < 300
	}
	for _, e := range expected {
		if code == e {
			return true
		}
	}
	return false
}
