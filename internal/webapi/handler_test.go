package webapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/your-username/debuginfod-go/internal/indexer"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/buildid"
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

func TestHandlerSection(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "hello")
	if err := os.WriteFile(src, []byte("int main(){return 0;}"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("gcc", "-g", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, out)
	}

	store, err := storage.New(filepath.Join(tmp, "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	id, err := buildid.FromPath(bin)
	if err != nil {
		t.Fatal(err)
	}
	id = buildid.Normalize(id)

	if err := store.AddArtifact(storage.ArtifactInput{
		BuildID: id, Type: "executable", FilePath: bin,
	}, 1); err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/buildid/"+id+"/section/.note.gnu.build-id", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("section status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(rec.Body.Bytes()) == 0 {
		t.Fatal("empty section body")
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

	handler := MetadataHandler(store, 5*time.Second)
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

func TestIntegrationHTTPEndpoints(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "hello")
	if err := os.WriteFile(src, []byte("int main(){return 0;}"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("gcc", "-g", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, out)
	}

	dbPath := filepath.Join(tmp, "index.sqlite")
	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	idx := indexer.NewIndexer(store, []string{tmp}, filepath.Join(tmp, "cache"))
	if err := idx.Scan(); err != nil {
		t.Fatal(err)
	}

	id, err := buildid.FromPath(bin)
	if err != nil {
		t.Fatal(err)
	}
	id = buildid.Normalize(id)

	server := httptest.NewServer(NewMux(store, 5*time.Second))
	defer server.Close()

	t.Run("healthz", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/healthz")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})

	t.Run("executable", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/buildid/" + id + "/executable")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})

	t.Run("metadata-buildid", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/metadata?key=buildid&value=" + id)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d", resp.StatusCode)
		}
		var meta storage.MetadataResponse
		if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
			t.Fatal(err)
		}
		if len(meta.Results) != 1 || !meta.Complete {
			t.Fatalf("metadata=%+v", meta)
		}
	})

	t.Run("section", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/buildid/" + id + "/section/.note.gnu.build-id")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})

	t.Run("incremental-skip", func(t *testing.T) {
		if err := idx.Scan(); err != nil {
			t.Fatal(err)
		}
		needs, err := store.NeedsScan(bin, fileMtime(bin), fileSize(bin))
		if err != nil || needs {
			t.Fatalf("second scan should skip unchanged file: needs=%v err=%v", needs, err)
		}
	})
}

func fileMtime(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.ModTime().UnixNano()
}

func fileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}
