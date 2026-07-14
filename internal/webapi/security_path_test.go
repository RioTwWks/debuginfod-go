package webapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/your-username/debuginfod-go/internal/storage"
)

func TestHandlerSourcePathTraversalRejected(t *testing.T) {
	tmp := t.TempDir()
	store, err := storage.New(filepath.Join(tmp, "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	handler := NewHandler(ServerOpts{
		Store:     store,
		ScanPaths: []string{tmp},
		CacheDir:  filepath.Join(tmp, "cache"),
	})

	req := httptest.NewRequest(http.MethodGet, "/buildid/deadbeef/source/%2e%2e/etc/passwd", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerServeOutsideScanRootForbidden(t *testing.T) {
	tmp := t.TempDir()
	store, err := storage.New(filepath.Join(tmp, "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	allowed := filepath.Join(tmp, "allowed")
	secret := filepath.Join(tmp, "secret", "bin")
	if err := os.MkdirAll(allowed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(secret), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := store.AddArtifact(storage.ArtifactInput{
		BuildID: "abc", Type: "executable", FilePath: secret,
	}, 1); err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(ServerOpts{
		Store:     store,
		ScanPaths: []string{allowed},
		CacheDir:  filepath.Join(tmp, "cache"),
	})

	req := httptest.NewRequest(http.MethodGet, "/buildid/abc/executable", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestHandlerInvalidSectionName(t *testing.T) {
	tmp := t.TempDir()
	store, _ := storage.New(filepath.Join(tmp, "test.sqlite"))
	defer store.Close()

	handler := NewHandler(ServerOpts{Store: store, ScanPaths: []string{tmp}})
	req := httptest.NewRequest(http.MethodGet, "/buildid/abc/section/..", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
