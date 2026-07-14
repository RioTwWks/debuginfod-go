package webapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/your-username/debuginfod-go/internal/metrics"
)

type mockScanTrigger struct {
	calls int
}

func (m *mockScanTrigger) Trigger() {
	m.calls++
}

func TestReadyHandlerNotReady(t *testing.T) {
	collector := metrics.New()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	ReadyHandler(collector)(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestReadyHandlerReady(t *testing.T) {
	collector := metrics.New()
	collector.MarkReady()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	ReadyHandler(collector)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAdminRescanRequiresKey(t *testing.T) {
	trigger := &mockScanTrigger{}
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", nil)
	rec := httptest.NewRecorder()
	AdminRescanHandler(trigger, "secret")(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	if trigger.calls != 0 {
		t.Fatalf("trigger called without auth")
	}
}

func TestAdminRescanAccepted(t *testing.T) {
	trigger := &mockScanTrigger{}
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan?key=secret", nil)
	rec := httptest.NewRecorder()
	AdminRescanHandler(trigger, "secret")(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if trigger.calls != 1 {
		t.Fatalf("trigger calls=%d", trigger.calls)
	}
	var payload map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "accepted" {
		t.Fatalf("payload=%v", payload)
	}
}

func TestAdminRescanDisabledWithoutKey(t *testing.T) {
	trigger := &mockScanTrigger{}
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", nil)
	rec := httptest.NewRecorder()
	AdminRescanHandler(trigger, "")(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAdminRescanScanDisabled(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan?key=secret", nil)
	rec := httptest.NewRecorder()
	AdminRescanHandler(nil, "secret")(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestReadyzSkipsBasicAuth(t *testing.T) {
	collector := metrics.New()
	collector.MarkReady()
	mux := http.NewServeMux()
	mux.HandleFunc("/readyz", ReadyHandler(collector))
	h := BasicAuthMiddleware("user", "pass", mux)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz should skip auth, status=%d", rec.Code)
	}
}
