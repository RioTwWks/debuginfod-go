package storage

import (
	"testing"
)

func TestDedupMigrationAndCRUD(t *testing.T) {
	store, err := New(t.TempDir() + "/dedup.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	pid, err := store.EnsureDedupProject("QuikServer")
	if err != nil || pid <= 0 {
		t.Fatalf("project: id=%d err=%v", pid, err)
	}

	bid, err := store.UpsertDedupBuildDir(pid, "/data/QuikServer/build_1_2025-01-01", 1)
	if err != nil || bid <= 0 {
		t.Fatalf("build dir: id=%d err=%v", bid, err)
	}

	fid, err := store.UpsertDedupFile(DedupFile{
		BuildDirID:   bid,
		FilePath:     "/data/QuikServer/build_1/lib.so.1.0.0.100.debug",
		Filename:     "lib.so.1.0.0.100.debug",
		FileStem:     "lib.so",
		Version:      "1.0.0",
		FileBuildNum: 100,
		CommitTag:    "DEVOPS-1",
		OriginalSize: 1024,
	})
	if err != nil || fid <= 0 {
		t.Fatalf("file: id=%d err=%v", fid, err)
	}

	got, err := store.GetDedupFileByPath("/data/QuikServer/build_1/lib.so.1.0.0.100.debug")
	if err != nil {
		t.Fatal(err)
	}
	if got.CommitTag != "DEVOPS-1" {
		t.Fatalf("unexpected tag: %s", got.CommitTag)
	}

	dirs, err := store.ListPendingBuildDirs("QuikServer", 10)
	if err != nil || len(dirs) != 1 {
		t.Fatalf("pending dirs: %v err=%v", dirs, err)
	}

	st, err := store.DedupStats()
	if err != nil || st.FilesTotal != 1 {
		t.Fatalf("stats: %+v err=%v", st, err)
	}
}
