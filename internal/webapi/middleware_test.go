package webapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/your-username/debuginfod-go/internal/metrics"
)

func TestShouldRecordHTTPMetric(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/buildid/abc/debuginfo", true},
		{"/metadata", true},
		{"/healthz", true},
		{"/zabbix", true},
		{"/ui", false},
		{"/ui/", false},
		{"/ui/api/stats", false},
		{"/ui/static/app.js", false},
	}
	for _, tc := range tests {
		if got := shouldRecordHTTPMetric(tc.path); got != tc.want {
			t.Errorf("shouldRecordHTTPMetric(%q)=%v want %v", tc.path, got, tc.want)
		}
	}
}

func TestMetricsMiddlewareSkipsUI(t *testing.T) {
	collector := metrics.New()
	handler := MetricsMiddleware(collector, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	for _, path := range []string{"/ui/api/stats", "/buildid/deadbeef/debuginfo"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	if collector.HTTPRequests() != 1 {
		t.Fatalf("http requests=%d want 1 (UI excluded)", collector.HTTPRequests())
	}
}
