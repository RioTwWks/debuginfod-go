package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/your-username/debuginfod-go/internal/dedup"
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
	if !contains(rec.Body.String(), "Файлы .debug") {
		t.Fatal("expected browse section in UI")
	}
	if !contains(rec.Body.String(), "debuginfo_ico.png") || !contains(rec.Body.String(), "debuginfo_2x.png") {
		t.Fatal("expected favicon and logo in HTML")
	}
}

func TestUIStaticBrandingAssets(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	for _, path := range []string{
		"/ui/static/debuginfo_ico.png",
		"/ui/static/debuginfo_2x.png",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
			t.Fatalf("%s content-type=%q", path, ct)
		}
		if rec.Body.Len() == 0 {
			t.Fatalf("%s empty body", path)
		}
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
	if payload.DedupEnabled {
		t.Fatal("expected dedup_enabled=false by default")
	}
}

func TestUIBrowseDedupDownload(t *testing.T) {
	root := t.TempDir()
	buildDir := filepath.Join(root, "Released", "Qt_Library", "qt", "build_1_2026-01-01")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	debugPath := filepath.Join(buildDir, "libQt5Core.so.5.15.2.100.debug")
	content := []byte("fake-debug-content")
	if err := os.WriteFile(debugPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := storage.New(filepath.Join(t.TempDir(), "browse-dedup-ui.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := dedup.Discover(store, []string{root}, nil); err != nil {
		t.Fatal(err)
	}
	df, err := store.GetDedupFileByPath(debugPath)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	Register(mux, Opts{
		Store:        store,
		Metrics:      metrics.New(),
		ScanPaths:    []string{root},
		AllowedRoots: []string{root},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/api/browse", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("browse status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload BrowseResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Count != 1 {
		t.Fatalf("count=%d want 1", payload.Count)
	}
	if len(payload.Projects) != 1 {
		t.Fatalf("projects=%+v", payload.Projects)
	}
	if payload.Projects[0].Path == "" {
		t.Fatalf("expected commit path in tree node: %+v", payload.Projects[0])
	}

	req = httptest.NewRequest(http.MethodGet, "/ui/api/download/dedup/"+strconv.FormatInt(df.ID, 10), nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("download status=%d body=%s", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "libQt5Core.so.5.15.2.100.debug") {
		t.Fatalf("content-disposition=%q", cd)
	}
	if !bytes.Equal(rec.Body.Bytes(), content) {
		t.Fatalf("body=%q", rec.Body.Bytes())
	}
}

func TestUIBrowse(t *testing.T) {
	store, err := storage.New(filepath.Join(t.TempDir(), "browse.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	scanRoot := t.TempDir()
	_ = store.AddArtifact(storage.ArtifactInput{
		BuildID: "aaa", Type: "debuginfo",
		FilePath: scanRoot + "/Released/ProjA/build_1/libfoo.so.debug",
		GitCommit: "9ae10425c6bbb99c7ee1f71a3941fd7aee058227",
	}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{
		BuildID: "bbb", Type: "executable",
		FilePath: scanRoot + "/Released/ProjA/build_1/bin/quik",
	}, 1)

	mux := http.NewServeMux()
	Register(mux, Opts{Store: store, Metrics: metrics.New(), ScanPaths: []string{scanRoot}})

	req := httptest.NewRequest(http.MethodGet, "/ui/api/browse", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload BrowseResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Count != 1 {
		t.Fatalf("count=%d want 1 debug file", payload.Count)
	}
	if len(payload.Projects) != 1 || payload.Projects[0].Path != "9ae10425c6bbb99c7ee1f71a3941fd7aee058227" {
		t.Fatalf("projects=%+v", payload.Projects)
	}

	req = httptest.NewRequest(http.MethodGet, "/ui/api/browse?q=9ae10425", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Count != 1 {
		t.Fatalf("commit search count=%d", payload.Count)
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
	_, store := testMux(t)
	defer store.Close()

	scanRoot := t.TempDir()
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "aaa", Type: "executable", FilePath: scanRoot + "/usr/bin/ls"}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "bbb", Type: "debuginfo", FilePath: scanRoot + "/usr/lib/debug/libc.so.debug"}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "ccc", Type: "executable", FilePath: scanRoot + "/opt/bin/tool"}, 1)

	mux2 := http.NewServeMux()
	Register(mux2, Opts{
		Store: store, Metrics: metrics.New(), ScanPaths: []string{scanRoot},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/api/search?key=path&value=usr/bin/*", nil)
	rec := httptest.NewRecorder()
	mux2.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Key != "path" {
		t.Fatalf("key=%q", payload.Key)
	}
	if payload.Count != 1 || len(payload.Results) != 1 {
		t.Fatalf("count=%d results=%d, want 1", payload.Count, len(payload.Results))
	}
	if payload.Results[0].BuildID != "aaa" {
		t.Fatalf("buildid=%q", payload.Results[0].BuildID)
	}
	if payload.Results[0].RelativePath != "usr/bin/ls" {
		t.Fatalf("relative=%q", payload.Results[0].RelativePath)
	}
}

func TestUISearchFile(t *testing.T) {
	_, store := testMux(t)
	defer store.Close()

	scanRoot := t.TempDir()
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "deadbeef", Type: "executable", FilePath: scanRoot + "/usr/bin/hello"}, 1)
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "cafebabe", Type: "executable", FilePath: scanRoot + "/usr/bin/other"}, 1)

	mux2 := http.NewServeMux()
	Register(mux2, Opts{
		Store: store, Metrics: metrics.New(), ScanPaths: []string{scanRoot},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/api/search?key=name&value=hello", nil)
	rec := httptest.NewRecorder()
	mux2.ServeHTTP(rec, req)
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

func TestUISearchPathBrowse(t *testing.T) {
	store, err := storage.New(filepath.Join(t.TempDir(), "browse.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	scanRoot := t.TempDir()
	_ = store.AddArtifact(storage.ArtifactInput{BuildID: "x", Type: "debuginfo", FilePath: scanRoot + "/a/b.debug"}, 1)

	mux := http.NewServeMux()
	Register(mux, Opts{Store: store, Metrics: metrics.New(), ScanPaths: []string{scanRoot}})

	req := httptest.NewRequest(http.MethodGet, "/ui/api/search?key=path", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var payload SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("results=%d", len(payload.Results))
	}
}

func TestUISearchGlobRequiresValue(t *testing.T) {
	mux, store := testMux(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/ui/api/search?key=name", nil)
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
