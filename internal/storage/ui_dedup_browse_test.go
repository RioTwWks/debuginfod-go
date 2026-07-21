package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowseFilesForUIIncludesDedupOnly(t *testing.T) {
	root := t.TempDir()
	buildDir := filepath.Join(root, "Released", "Qt_Library", "qt", "build_1_2026-01-01")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	debugPath := filepath.Join(buildDir, "libQt5Core.so.5.15.2.100.debug")
	if err := os.WriteFile(debugPath, []byte("fake-debug"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := New(filepath.Join(t.TempDir(), "browse-dedup.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	projectID, err := store.EnsureDedupProject("Released/Qt_Library/qt")
	if err != nil {
		t.Fatal(err)
	}
	buildDirID, err := store.UpsertDedupBuildDir(projectID, buildDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertDedupFile(DedupFile{
		BuildDirID:   buildDirID,
		FilePath:     debugPath,
		Filename:     "libQt5Core.so.5.15.2.100.debug",
		FileStem:     "libQt5Core.so",
		Version:      "5.15.2",
		FileBuildNum: 100,
		CommitTag:    "abc123commit",
		OriginalSize: 10,
		Status:       DedupStatusDone,
		StorageKind:  DedupKindFull,
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	files, complete, err := store.BrowseFilesForUI(ctx, []string{root}, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if !complete {
		t.Fatal("expected complete result")
	}
	if len(files) != 1 {
		t.Fatalf("files=%d want 1 dedup-only", len(files))
	}
	if files[0].Source != "dedup" || files[0].DedupID == 0 {
		t.Fatalf("unexpected file: %+v", files[0])
	}
	if files[0].Project != "Released/Qt_Library/qt" {
		t.Fatalf("project=%q", files[0].Project)
	}

	tree := BuildUITreeFromFiles(files)
	if len(tree) != 1 || tree[0].Name != "Released/Qt_Library/qt" {
		t.Fatalf("tree=%+v", tree)
	}
}

func TestBrowseFilesForUISkipsDedupWhenIndexed(t *testing.T) {
	root := t.TempDir()
	debugPath := filepath.Join(root, "Released", "ProjA", "build_1", "libfoo.so.debug")
	if err := os.MkdirAll(filepath.Dir(debugPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(debugPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := New(filepath.Join(t.TempDir(), "browse-dedup-dup.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_ = store.AddArtifact(ArtifactInput{
		BuildID: "aaa", Type: "debuginfo", FilePath: debugPath,
	}, 1)

	projectID, err := store.EnsureDedupProject("Released/ProjA")
	if err != nil {
		t.Fatal(err)
	}
	buildDirID, err := store.UpsertDedupBuildDir(projectID, filepath.Dir(debugPath), 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertDedupFile(DedupFile{
		BuildDirID:   buildDirID,
		FilePath:     debugPath,
		Filename:     "libfoo.so.debug",
		FileStem:     "libfoo.so",
		Version:      "1.0",
		FileBuildNum: 1,
		OriginalSize: 4,
		Status:       DedupStatusDone,
		StorageKind:  DedupKindFull,
	}); err != nil {
		t.Fatal(err)
	}

	files, _, err := store.BrowseFilesForUI(context.Background(), []string{root}, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Source != "artifact" {
		t.Fatalf("files=%+v", files)
	}
}
