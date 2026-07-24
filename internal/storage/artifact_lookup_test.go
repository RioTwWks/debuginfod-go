package storage

import (
	"path/filepath"
	"testing"
)

func TestGitCommitsByFilePathsPrefersDebuginfo(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "commits.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	path := "/data/proj/build_1/lib.so.1.0.0.1.debug"
	if err := store.AddArtifact(ArtifactInput{
		BuildID:   "exec-id",
		Type:      "executable",
		FilePath:  path,
		GitCommit: "wrong-commit",
	}, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.AddArtifact(ArtifactInput{
		BuildID:   "debug-id",
		Type:      "debuginfo",
		FilePath:  path,
		GitCommit: "tag:abc123",
	}, 2); err != nil {
		t.Fatal(err)
	}

	commits, err := store.GitCommitsByFilePaths([]string{path, "/missing.debug"})
	if err != nil {
		t.Fatal(err)
	}
	if got := commits[path]; got != "tag:abc123" {
		t.Fatalf("commit=%q want tag:abc123", got)
	}
	if _, ok := commits["/missing.debug"]; ok {
		t.Fatal("missing path should not be in map")
	}

	commit, ok, err := store.GitCommitByFilePath(path)
	if err != nil || !ok || commit != "tag:abc123" {
		t.Fatalf("GitCommitByFilePath=%q ok=%v err=%v", commit, ok, err)
	}
}

func TestUpsertDedupFilesBatch(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "batch.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	pid, err := store.EnsureDedupProject("proj")
	if err != nil {
		t.Fatal(err)
	}
	bid, err := store.UpsertDedupBuildDir(pid, "/data/proj/build_1", 1)
	if err != nil {
		t.Fatal(err)
	}

	files := []DedupFile{
		{
			BuildDirID: bid, FilePath: "/data/a.debug", Filename: "a.debug",
			FileStem: "a", Version: "1", FileBuildNum: 1, CommitTag: "c1", OriginalSize: 10,
		},
		{
			BuildDirID: bid, FilePath: "/data/b.debug", Filename: "b.debug",
			FileStem: "b", Version: "1", FileBuildNum: 2, CommitTag: "c2", OriginalSize: 20,
		},
	}
	n, err := store.UpsertDedupFilesBatch(files)
	if err != nil || n != 2 {
		t.Fatalf("batch n=%d err=%v", n, err)
	}

	gotA, err := store.GetDedupFileByPath("/data/a.debug")
	if err != nil || gotA.CommitTag != "c1" {
		t.Fatalf("a: %+v err=%v", gotA, err)
	}
	gotB, err := store.GetDedupFileByPath("/data/b.debug")
	if err != nil || gotB.CommitTag != "c2" {
		t.Fatalf("b: %+v err=%v", gotB, err)
	}

	files[0].CommitTag = "c1-updated"
	files[0].OriginalSize = 11
	n, err = store.UpsertDedupFilesBatch(files[:1])
	if err != nil || n != 1 {
		t.Fatalf("update batch n=%d err=%v", n, err)
	}
	gotA, err = store.GetDedupFileByPath("/data/a.debug")
	if err != nil || gotA.CommitTag != "c1-updated" || gotA.OriginalSize != 11 {
		t.Fatalf("updated a: %+v err=%v", gotA, err)
	}
}

func TestDedupSnapshotsByPaths(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "snap.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	pid, _ := store.EnsureDedupProject("proj")
	bid, _ := store.UpsertDedupBuildDir(pid, "/data/build_1", 1)
	path := "/data/lib.so.debug"
	id, err := store.UpsertDedupFile(DedupFile{
		BuildDirID: bid, FilePath: path, Filename: "lib.so.debug",
		FileStem: "lib.so", Version: "1", FileBuildNum: 1, CommitTag: "t", OriginalSize: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDedupFileDone(id, DedupKindFull, 0, "", "sha", 100); err != nil {
		t.Fatal(err)
	}

	snaps, err := store.DedupSnapshotsByPaths([]string{path, "/other.debug"})
	if err != nil {
		t.Fatal(err)
	}
	snap, ok := snaps[path]
	if !ok || snap.Status != DedupStatusDone || snap.OriginalSize != 100 {
		t.Fatalf("snap=%+v ok=%v", snap, ok)
	}
}
