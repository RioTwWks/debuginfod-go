package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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
		t.Fatalf("count=%d, want 2 (grouped by build-id)", payload.Count)
	}
}

func TestUISearchGlob(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "aaa", Type: "executable", FilePath: "/usr/bin/ls"}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "bbb", Type: "debuginfo", FilePath: "/usr/lib/debug/libc.so.debug"}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "ccc", Type: "executable", FilePath: "/opt/bin/tool"}, 1)

	req := httptest.NewRequest(http.MethodGet, "/ui/api/search?key=glob&value=/usr/bin/*", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Key != "glob" {
		t.Fatalf("key=%q", payload.Key)
	}
	if payload.Count != 1 || len(payload.Results) != 1 {
		t.Fatalf("count=%d results=%d, want 1", payload.Count, len(payload.Results))
	}
	if payload.Results[0].BuildID != "aaa" {
		t.Fatalf("buildid=%q", payload.Results[0].BuildID)
	}
}

func TestUISearchFile(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "deadbeef", Type: "executable", FilePath: "/usr/bin/hello"}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "cafebabe", Type: "executable", FilePath: "/usr/bin/other"}, 1)

	req := httptest.NewRequest(http.MethodGet, "/ui/api/search?key=file&value=/usr/bin/hello", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	var payload SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Count != 1 || payload.Results[0].BuildID != "deadbeef" {
		t.Fatalf("unexpected results: %+v", payload)
	}
}

func TestUISearchGlobRequiresValue(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/ui/api/search?key=glob", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestUISearchUnsupportedKey(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/ui/api/search?key=unknown&value=x", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestUISearchGrouped(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "deadbeef", Type: "executable", FilePath: "/a"}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "deadbeef", Type: "debuginfo", FilePath: "/a.debug"}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "cafebabe", Type: "debuginfo", FilePath: "/b.debug"}, 1)

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
	if payload.Count != 1 || len(payload.Grouped) != 1 {
		t.Fatalf("count=%d grouped=%d", payload.Count, len(payload.Grouped))
	}
	if len(payload.Grouped[0].Types) != 2 {
		t.Fatalf("types=%v", payload.Grouped[0].Types)
	}
}

func TestUIScansAPI(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	_ = store.InsertScanRun(storage.ScanRunRecord{
		FinishedAt:     time.Now(),
		DurationMs:     100,
		Indexed:        5,
		ArtifactsTotal: 10,
		ScannedFiles:   8,
		BytesOnDisk:    4096,
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/api/scans", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload ScansResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.IndexScans) != 1 {
		t.Fatalf("index_scans=%d", len(payload.IndexScans))
	}
	if payload.IndexScans[0].ArtifactsTotal != 10 {
		t.Fatalf("artifacts_total=%d", payload.IndexScans[0].ArtifactsTotal)
	}
	if payload.IndexScans[0].BytesOnDisk != 4096 {
		t.Fatalf("bytes_on_disk=%d", payload.IndexScans[0].BytesOnDisk)
	}
	if payload.DedupEnabled {
		t.Fatal("expected dedup_enabled=false by default")
	}
}

type fakeScanTrigger struct {
	calls int
}

func (f *fakeScanTrigger) Trigger() {
	f.calls++
}

func TestUIRescanAPI(t *testing.T) {
	store, err := storage.New(filepath.Join(t.TempDir(), "rescan.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	trigger := &fakeScanTrigger{}
	mux := http.NewServeMux()
	Register(mux, Opts{
		Store:       store,
		Metrics:     metrics.New(),
		CacheBytes:  func() int64 { return 0 },
		ScanEnabled: true,
		ScanTrigger: trigger,
	})

	req := httptest.NewRequest(http.MethodPost, "/ui/api/rescan", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if trigger.calls != 1 {
		t.Fatalf("trigger calls=%d", trigger.calls)
	}

	disabledMux := http.NewServeMux()
	Register(disabledMux, Opts{Store: store, ScanEnabled: false})
	rec = httptest.NewRecorder()
	disabledMux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("disabled status=%d", rec.Code)
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
