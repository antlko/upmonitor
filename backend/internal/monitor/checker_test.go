package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
)

func TestCheckExtractsTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := Check(context.Background(), srv.Client(), config.Service{URL: srv.URL})
	if res.Status != db.StatusOnline {
		t.Fatalf("status = %q, want online", res.Status)
	}
	if res.TLS == nil {
		t.Fatal("expected TLS info from an HTTPS server, got nil")
	}
	if res.TLS.NotAfter.IsZero() {
		t.Error("expected a non-zero NotAfter expiry")
	}
	if res.TLS.NotAfter.Before(res.TLS.NotBefore) {
		t.Error("NotAfter should be after NotBefore")
	}
}

func TestCheckNoTLSForHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := Check(context.Background(), srv.Client(), config.Service{URL: srv.URL})
	if res.Status != db.StatusOnline {
		t.Fatalf("status = %q, want online", res.Status)
	}
	if res.TLS != nil {
		t.Errorf("expected no TLS info for a plain-HTTP server, got %+v", res.TLS)
	}
}
