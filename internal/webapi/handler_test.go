package webapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/your-username/debuginfod-go/internal/storage"
)

func TestHandlerDebugInfoAndExecutable(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.sqlite")
	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	execPath := filepath.Join(tmp, "bin")
	debugPath := filepath.Join(tmp, "bin.debug")
	if err := os.WriteFile(execPath, []byte("exec"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(debugPath, []byte("debug"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := store.AddArtifact(storage.ArtifactInput{
		BuildID: "deadbeef", Type: "executable", FilePath: execPath,
	}, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.AddArtifact(storage.ArtifactInput{
		BuildID: "deadbeef", Type: "debuginfo", FilePath: debugPath,
	}, 1); err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/buildid/deadbeef/executable", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("executable status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/buildid/deadbeef/debuginfo", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("debuginfo status = %d", rec.Code)
	}
}

func TestHandlerSource(t *testing.T) {
	tmp := t.TempDir()
	store, err := storage.New(filepath.Join(tmp, "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	srcPath := filepath.Join(tmp, "main.c")
	if err := os.WriteFile(srcPath, []byte("int main(){}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.AddArtifact(storage.ArtifactInput{
		BuildID: "cafebabe", Type: "executable", FilePath: filepath.Join(tmp, "bin"),
	}, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.AddSource("cafebabe", "/project/main.c", srcPath, 1); err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/buildid/cafebabe/source/project/main.c", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("source status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestMetadataHandler(t *testing.T) {
	store, err := storage.New(filepath.Join(t.TempDir(), "meta.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_ = store.AddArtifact(storage.ArtifactInput{
		BuildID: "abc", Type: "executable", FilePath: "/opt/bin/tool",
	}, 1)

	handler := MetadataHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/metadata?key=glob&value=/opt/bin/*", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp storage.MetadataResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || !resp.Complete {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
