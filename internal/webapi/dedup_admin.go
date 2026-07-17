package webapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/your-username/debuginfod-go/internal/dedup"
)

// DedupRunner запускает backfill/ingest dedup.
type DedupRunner interface {
	RunBackfill(project string, batch int, dryRun bool) (dedup.BackfillResult, error)
}

// AdminDedupBackfillHandler — POST /admin/dedup-backfill.
func AdminDedupBackfillHandler(runner DedupRunner, adminKey string) http.HandlerFunc {
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
		if runner == nil {
			http.Error(w, "dedup disabled", http.StatusConflict)
			return
		}

		q := r.URL.Query()
		project := q.Get("project")
		dryRun := q.Get("dry_run") == "true" || q.Get("dry_run") == "1"
		batch := 50
		if v := q.Get("batch"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				batch = n
			}
		}

		result, err := runner.RunBackfill(project, batch, dryRun)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
}
