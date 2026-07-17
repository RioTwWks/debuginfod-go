package webapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/your-username/debuginfod-go/internal/dedup"
	"github.com/your-username/debuginfod-go/internal/storage"
)

type fakeDedupRunner struct{}

func (fakeDedupRunner) RunBackfill(project string, batch int, dryRun bool) (dedup.BackfillResult, error) {
	return dedup.BackfillResult{DryRun: dryRun, BuildDirsProcessed: 1}, nil
}

func TestAdminDedupBackfill(t *testing.T) {
	store, _ := storage.New(t.TempDir() + "/t.sqlite")
	defer store.Close()

	handler := AdminDedupBackfillHandler(fakeDedupRunner{}, "secret")
	req := httptest.NewRequest(http.MethodPost, "/admin/dedup-backfill?dry_run=true", nil)
	req.Header.Set("X-Admin-Token", "secret")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var result dedup.BackfillResult
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.DryRun || result.BuildDirsProcessed != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAdminDedupBackfillDisabled(t *testing.T) {
	handler := AdminDedupBackfillHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/admin/dedup-backfill", nil)
	req.Header.Set("X-Admin-Token", "secret")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}
