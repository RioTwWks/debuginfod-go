package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndexSummary(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "summary.sqlite")
	store, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	filePath := filepath.Join(tmp, "lib.so.debug")
	if err := os.WriteFile(filePath, []byte("debug-bytes-here"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := store.AddArtifact(ArtifactInput{
		BuildID:  "deadbeef",
		Type:     "debuginfo",
		FilePath: filePath,
	}, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkScanned(filePath, 1, int64(len("debug-bytes-here")), "elf"); err != nil {
		t.Fatal(err)
	}

	summary, err := store.IndexSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.ArtifactsTotal != 1 {
		t.Fatalf("artifacts_total=%d", summary.ArtifactsTotal)
	}
	if summary.ArtifactsDebuginfo != 1 {
		t.Fatalf("artifacts_debuginfo=%d", summary.ArtifactsDebuginfo)
	}
	if summary.ArtifactsExecutable != 0 {
		t.Fatalf("artifacts_executable=%d", summary.ArtifactsExecutable)
	}
	if summary.ScannedFilesTotal != 1 {
		t.Fatalf("scanned_files_total=%d", summary.ScannedFilesTotal)
	}
	if summary.BytesOnDisk != int64(len("debug-bytes-here")) {
		t.Fatalf("bytes_on_disk=%d", summary.BytesOnDisk)
	}
}

func TestIndexedBytesOnDiskDedupesPaths(t *testing.T) {
	tmp := t.TempDir()
	store, err := New(filepath.Join(tmp, "dedupe.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	filePath := filepath.Join(tmp, "same.debug")
	if err := os.WriteFile(filePath, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, bid := range []string{"aaa", "bbb"} {
		if err := store.AddArtifact(ArtifactInput{
			BuildID:  bid,
			Type:     "debuginfo",
			FilePath: filePath,
		}, 1); err != nil {
			t.Fatal(err)
		}
	}

	bytes, err := store.IndexedBytesOnDisk()
	if err != nil {
		t.Fatal(err)
	}
	if bytes != 5 {
		t.Fatalf("bytes=%d want 5", bytes)
	}
}
