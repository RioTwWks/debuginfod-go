package indexer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/buildid"
)

func TestIndexerScanELF(t *testing.T) {
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

	idx := NewIndexer(store, []string{tmp}, filepath.Join(tmp, "cache"))
	if err := idx.Scan(); err != nil {
		t.Fatal(err)
	}

	id, err := buildid.FromPath(bin)
	if err != nil {
		t.Fatal(err)
	}
	id = buildid.Normalize(id)

	path, err := store.GetArtifactPath(id, "executable")
	if err != nil {
		t.Fatalf("artifact not indexed: %v", err)
	}
	if path != bin {
		t.Fatalf("path = %q, want %q", path, bin)
	}
}

func TestIndexerScanGoBinary(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.go")
	bin := filepath.Join(tmp, "hello")
	if err := os.WriteFile(src, []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	store, err := storage.New(filepath.Join(tmp, "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	idx := NewIndexer(store, []string{tmp}, filepath.Join(tmp, "cache"))
	if err := idx.Scan(); err != nil {
		t.Fatal(err)
	}

	result, err := buildid.FromPathDetailed(bin)
	if err != nil {
		t.Fatal(err)
	}

	path, err := store.GetArtifactPath(result.Value, "executable")
	if err != nil {
		t.Fatalf("go binary not indexed: %v", err)
	}
	if path == "" {
		t.Fatal("empty artifact path")
	}
}
