package indexer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/buildid"
)

func testIndexer(store *storage.Storage, tmp string) *Indexer {
	return NewIndexer(Options{
		Storage:  store,
		Paths:    []string{tmp},
		CacheDir: filepath.Join(tmp, "cache"),
		Workers:  2,
		Metrics:  metrics.New(),
	})
}

func TestIndexerScanELF(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.c")
	bin := filepath.Join(tmp, "hello")
	_ = os.WriteFile(src, []byte("int main(){return 0;}"), 0o644)
	_ = exec.Command("gcc", "-g", "-o", bin, src).Run()

	store, err := storage.New(filepath.Join(tmp, "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := testIndexer(store, tmp).Scan(); err != nil {
		t.Fatal(err)
	}

	id, _ := buildid.FromPath(bin)
	id = buildid.Normalize(id)
	path, err := store.GetArtifactPath(id, "executable")
	if err != nil || path != bin {
		t.Fatalf("path=%q err=%v", path, err)
	}
}

func TestIndexerIncrementalSkip(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "hello")
	_ = os.WriteFile(filepath.Join(tmp, "main.c"), []byte("int main(){return 0;}"), 0o644)
	_ = exec.Command("gcc", "-g", "-o", bin, filepath.Join(tmp, "main.c")).Run()

	store, _ := storage.New(filepath.Join(tmp, "index.sqlite"))
	defer store.Close()

	idx := testIndexer(store, tmp)
	_ = idx.Scan()
	_ = idx.Scan()

	st, _ := os.Stat(bin)
	needs, _ := store.NeedsScan(bin, st.ModTime().UnixNano(), st.Size())
	if needs {
		t.Fatal("expected skip on second scan")
	}
}
