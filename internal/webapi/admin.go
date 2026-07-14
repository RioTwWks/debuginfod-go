package webapi

import (
	"encoding/json"
	"net/http"

	"github.com/your-username/debuginfod-go/internal/metrics"
)

// ScanTrigger запрашивает внеочередной scan.
type ScanTrigger interface {
	Trigger()
}

// ReadyHandler возвращает 200 после первого успешного scan (readiness).
func ReadyHandler(collector *metrics.Collector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if collector == nil || !collector.Ready() {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready\n"))
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	}
}

// AdminRescanHandler запускает внеочередной scan (POST /admin/rescan).
func AdminRescanHandler(trigger ScanTrigger, adminKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if adminKey == "" {
			http.Error(w, "admin API disabled", http.StatusForbidden)
			return
		}
		if !checkAdminToken(r, adminKey) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if trigger == nil {
			http.Error(w, "scan disabled", http.StatusConflict)
			return
		}
		trigger.Trigger()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	}
}

func checkAdminToken(r *http.Request, key string) bool {
	return r.Header.Get("X-Admin-Token") == key || r.URL.Query().Get("key") == key
}
