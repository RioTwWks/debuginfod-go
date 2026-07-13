package storage

import (
	"path/filepath"
	"testing"
)

func TestStorageArtifactsAndSources(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	store, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.AddArtifact("abc123", "/bin/ls", "executable", 1); err != nil {
		t.Fatal(err)
	}
	if err := store.AddSource("abc123", "/src/main.c", "/src/main.c", 1); err != nil {
		t.Fatal(err)
	}

	path, err := store.GetArtifact("abc123", "executable")
	if err != nil || path != "/bin/ls" {
		t.Fatalf("GetArtifact = %q, %v", path, err)
	}

	src, err := store.GetSource("abc123", "/src/main.c")
	if err != nil || src != "/src/main.c" {
		t.Fatalf("GetSource = %q, %v", src, err)
	}

	ok, err := store.HasBuildID("abc123")
	if err != nil || !ok {
		t.Fatalf("HasBuildID = %v, %v", ok, err)
	}
}
