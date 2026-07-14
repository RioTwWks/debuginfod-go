package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/storage"
)

func testMux(t *testing.T) (*http.ServeMux, *storage.Storage) {
	t.Helper()
	store, err := storage.New(filepath.Join(t.TempDir(), "ui.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	Register(mux, Opts{
		Store:      store,
		Metrics:    metrics.New(),
		CacheBytes: func() int64 { return 0 },
	})
	return mux, store
}

func TestUIIndex(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/ui/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("content-type=%q", ct)
	}
	if !contains(rec.Body.String(), "debuginfod-go") {
		t.Fatal("expected HTML body")
	}
}

func TestUIStats(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	_ = store.AddArtifact(storage.ArtifactInput{
		BuildID: "abc", Type: "executable", FilePath: "/bin/a",
	}, 1)

	req := httptest.NewRequest(http.MethodGet, "/ui/api/stats", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	var payload StatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.ArtifactsTotal != 1 {
		t.Fatalf("artifacts_total=%d", payload.ArtifactsTotal)
	}
}

func TestUISearch(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "deadbeef", Type: "executable", FilePath: "/a"}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "deadcafe", Type: "debuginfo", FilePath: "/b"}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "cafebabe", Type: "executable", FilePath: "/c"}, 1)

	req := httptest.NewRequest(http.MethodGet, "/ui/api/search?q=dead", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	var payload SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Count != 2 {
		t.Fatalf("count=%d, want 2", payload.Count)
	}
}

func TestUIRedirect(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status=%d", rec.Code)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
