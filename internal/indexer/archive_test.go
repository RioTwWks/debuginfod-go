package indexer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/internal/webapi"
	"github.com/your-username/debuginfod-go/pkg/buildid"
)

func TestIndexerLazyTarArchive(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "hello")
	_ = os.WriteFile(src, []byte("int main(){return 0;}"), 0o644)
	_ = exec.Command("gcc", "-g", "-o", bin, src).Run()
	elfData, _ := os.ReadFile(bin)

	archivePath := filepath.Join(tmp, "symbols.tar.gz")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "usr/bin/hello", Mode: 0o755, Size: int64(len(elfData))})
	_, _ = tw.Write(elfData)
	_ = tw.Close()
	_ = gz.Close()
	_ = os.WriteFile(archivePath, buf.Bytes(), 0o644)

	store, err := storage.New(filepath.Join(tmp, "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cacheDir := filepath.Join(tmp, "cache")
	idx := NewIndexer(Options{
		Storage:     store,
		Paths:       []string{tmp},
		CacheDir:    cacheDir,
		Workers:     2,
		Metrics:     metrics.New(),
		LazyExtract: true,
	})
	if err := idx.Scan(); err != nil {
		t.Fatal(err)
	}

	id, _ := buildid.FromPath(bin)
	id = buildid.Normalize(id)

	loc, err := store.GetArtifactLocation(id, "executable")
	if err != nil {
		t.Fatal(err)
	}
	if loc.ArchivePath == "" || loc.MemberPath == "" {
		t.Fatalf("expected lazy archive ref, got %+v", loc)
	}
	if loc.FilePath != "" {
		t.Fatalf("expected empty file_path for lazy, got %q", loc.FilePath)
	}

	handler := webapi.NewHandler(webapi.ServerOpts{
		Store:    store,
		CacheDir: cacheDir,
		Metrics:  metrics.New(),
	})
	req := httptest.NewRequest(http.MethodGet, "/buildid/"+id+"/executable", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
