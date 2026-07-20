package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchPathForUI(t *testing.T) {
	root := t.TempDir()
	scanRoot := filepath.Join(root, "debug_linux")
	path := filepath.Join(scanRoot, "Released", "Quik", "build_1", "lib.so.debug")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := New(filepath.Join(t.TempDir(), "path.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_ = store.AddArtifact(ArtifactInput{
		BuildID: "abc123", Type: "debuginfo", FilePath: path,
	}, 1)

	ctx := context.Background()
	resp, err := store.SearchPathForUI(ctx, []string{scanRoot}, "Released/Quik", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("results=%d", len(resp.Results))
	}
	if resp.Results[0].RelativePath != "Released/Quik/build_1/lib.so.debug" {
		t.Fatalf("relative=%q", resp.Results[0].RelativePath)
	}
}

func TestSearchNameForUI(t *testing.T) {
	root := t.TempDir()
	scanRoot := filepath.Join(root, "scan")
	path := filepath.Join(scanRoot, "a", "quik-16.0.0.10.debug")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := New(filepath.Join(t.TempDir(), "name.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_ = store.AddArtifact(ArtifactInput{
		BuildID: "deadbeef", Type: "debuginfo", FilePath: path,
	}, 1)

	ctx := context.Background()
	resp, err := store.SearchNameForUI(ctx, []string{scanRoot}, "quik-16", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Filename != "quik-16.0.0.10.debug" {
		t.Fatalf("results=%+v", resp.Results)
	}
}
