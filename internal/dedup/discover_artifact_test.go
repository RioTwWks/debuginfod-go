package dedup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/your-username/debuginfod-go/internal/storage"
)

func TestDiscoverUsesArtifactGitCommit(t *testing.T) {
	root := t.TempDir()
	buildDir := filepath.Join(root, "proj", "build_1_2026-01-01")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	debugPath := filepath.Join(buildDir, "lib.so.1.0.0.1.debug")
	if err := os.WriteFile(debugPath, []byte("no-elf-comment-here"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := storage.New(filepath.Join(t.TempDir(), "artifact-commit.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.AddArtifact(storage.ArtifactInput{
		BuildID:   "idx-build",
		Type:      "debuginfo",
		FilePath:  debugPath,
		GitCommit: "tag:indexed-commit",
	}, 1); err != nil {
		t.Fatal(err)
	}

	n, err := Discover(store, []string{root}, nil)
	if err != nil || n != 1 {
		t.Fatalf("discover n=%d err=%v", n, err)
	}

	got, err := store.GetDedupFileByPath(debugPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.CommitTag != "tag:indexed-commit" {
		t.Fatalf("commit_tag=%q want tag:indexed-commit", got.CommitTag)
	}
}
