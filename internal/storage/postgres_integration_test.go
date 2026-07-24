//go:build integration

package storage

import (
	"os"
	"testing"
)

func openTestPostgres(t *testing.T) *Storage {
	t.Helper()
	url := os.Getenv("DEBUGINFOD_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set DEBUGINFOD_TEST_DATABASE_URL (see deploy/docker-compose/README.md)")
	}
	store, err := Open("", url)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestPostgresOpenAndMigrate(t *testing.T) {
	store := openTestPostgres(t)
	if store.dialect != DialectPostgres {
		t.Fatalf("dialect=%s", store.dialect)
	}
	var n int64
	if err := store.db.QueryRow(`SELECT COUNT(1) FROM dedup_projects`).Scan(&n); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresDedupCRUD(t *testing.T) {
	store := openTestPostgres(t)

	pid, err := store.EnsureDedupProject("integration-proj")
	if err != nil || pid <= 0 {
		t.Fatalf("project id=%d err=%v", pid, err)
	}
	bid, err := store.UpsertDedupBuildDir(pid, "/tmp/build_1", 1)
	if err != nil || bid <= 0 {
		t.Fatalf("build dir id=%d err=%v", bid, err)
	}

	path := "/tmp/integration/lib.so.1.0.0.1.debug"
	n, err := store.UpsertDedupFilesBatch([]DedupFile{{
		BuildDirID: bid, FilePath: path, Filename: "lib.so.1.0.0.1.debug",
		FileStem: "lib.so", Version: "1.0.0", FileBuildNum: 1,
		CommitTag: "tag:test", OriginalSize: 512,
	}})
	if err != nil || n != 1 {
		t.Fatalf("batch n=%d err=%v", n, err)
	}

	got, err := store.GetDedupFileByPath(path)
	if err != nil || got.CommitTag != "tag:test" {
		t.Fatalf("file %+v err=%v", got, err)
	}

	if err := store.AddArtifact(ArtifactInput{
		BuildID: "pg-test", Type: "debuginfo", FilePath: path, GitCommit: "tag:from-index",
	}, 1); err != nil {
		t.Fatal(err)
	}
	commits, err := store.GitCommitsByFilePaths([]string{path})
	if err != nil || commits[path] != "tag:from-index" {
		t.Fatalf("commits=%v err=%v", commits, err)
	}
}
