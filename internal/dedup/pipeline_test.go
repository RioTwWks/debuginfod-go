package dedup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/your-username/debuginfod-go/internal/storage"
)

func TestCompressAndVerify(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "test.debug")
	content := []byte("ELF debuginfo payload for zstd test\n")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}

	sha, err := FileSHA256(src)
	if err != nil {
		t.Fatal(err)
	}

	blobStore := NewBlobStore(filepath.Join(dir, "blobs"))
	blobPath := blobStore.PathForSHA(sha)

	compSize, err := CompressFileTo(src, blobPath)
	if err != nil {
		t.Fatal(err)
	}
	if compSize <= 0 {
		t.Fatalf("compressed size=%d", compSize)
	}
	if err := VerifyDecompress(blobPath, sha); err != nil {
		t.Fatal(err)
	}
}

func TestProcessFilesCompressAndCASRef(t *testing.T) {
	store, err := storage.New(t.TempDir() + "/dedup.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	dir := t.TempDir()
	blobDir := filepath.Join(dir, "blobs")
	buildDir := filepath.Join(dir, "proj", "build_1_2025-01-01")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := []byte("identical debug content\n")
	file1 := filepath.Join(buildDir, "lib.so.1.0.0.100.debug")
	file2 := filepath.Join(buildDir, "lib.so.1.0.0.101.debug")
	if err := os.WriteFile(file1, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, content, 0o644); err != nil {
		t.Fatal(err)
	}

	pid, _ := store.EnsureDedupProject("proj")
	bid, _ := store.UpsertDedupBuildDir(pid, buildDir, 1)
	id1, _ := store.UpsertDedupFile(storage.DedupFile{
		BuildDirID: bid, FilePath: file1, Filename: "lib.so.1.0.0.100.debug",
		FileStem: "lib.so", Version: "1.0.0", FileBuildNum: 100, OriginalSize: int64(len(content)),
	})
	id2, _ := store.UpsertDedupFile(storage.DedupFile{
		BuildDirID: bid, FilePath: file2, Filename: "lib.so.1.0.0.101.debug",
		FileStem: "lib.so", Version: "1.0.0", FileBuildNum: 101, OriginalSize: int64(len(content)),
	})

	opts := Options{
		Store:     store,
		BlobStore: NewBlobStore(blobDir),
	}
	files, _ := store.ListPendingDedupFiles([]int64{bid})
	compressed, dedupRef, _, errs, _, _ := processFiles(opts, files)
	if errs != 0 {
		t.Fatalf("errors=%d", errs)
	}
	if compressed != 1 {
		t.Fatalf("compressed=%d want 1", compressed)
	}
	if dedupRef != 1 {
		t.Fatalf("dedupRef=%d want 1", dedupRef)
	}

	f1, err := store.GetDedupFileByID(id1)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := store.GetDedupFileByID(id2)
	if err != nil {
		t.Fatal(err)
	}
	if f1.StorageKind != storage.DedupKindCompressed {
		t.Fatalf("f1 kind=%s", f1.StorageKind)
	}
	if f2.StorageKind != storage.DedupKindRef {
		t.Fatalf("f2 kind=%s", f2.StorageKind)
	}
	if f1.BlobPath == "" || f1.BlobPath != f2.BlobPath {
		t.Fatalf("blob paths: %q vs %q", f1.BlobPath, f2.BlobPath)
	}
	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Fatal("original file1 should be removed")
	}
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Fatal("original file2 should be removed")
	}
}

func TestRunBackfillDryRun(t *testing.T) {
	store, err := storage.New(t.TempDir() + "/test.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	opts := Options{
		Store:     store,
		ScanPaths: []string{t.TempDir()},
		BlobStore: NewBlobStore(t.TempDir()),
		Projects:  []string{"QuikServer"},
		DryRun:    true,
	}
	result, err := RunBackfill(opts, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DryRun {
		t.Fatal("expected dry run")
	}
}

func TestRestoreToCache(t *testing.T) {
	store, err := storage.New(t.TempDir() + "/dedup.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	blobDir := filepath.Join(dir, "blobs")
	buildDir := filepath.Join(dir, "build_1")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := []byte("restore me\n")
	origPath := filepath.Join(buildDir, "lib.so.1.0.0.1.debug")
	if err := os.WriteFile(origPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sha, _ := FileSHA256(origPath)
	blobPath := NewBlobStore(blobDir).PathForSHA(sha)
	compSize, err := CompressFileTo(origPath, blobPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(origPath)

	pid, _ := store.EnsureDedupProject("p")
	bid, _ := store.UpsertDedupBuildDir(pid, buildDir, 1)
	fid, _ := store.UpsertDedupFile(storage.DedupFile{
		BuildDirID: bid, FilePath: origPath, Filename: "lib.so.1.0.0.1.debug",
		FileStem: "lib.so", Version: "1.0.0", FileBuildNum: 1, OriginalSize: int64(len(content)),
	})
	_ = store.MarkDedupFileDone(fid, storage.DedupKindCompressed, 0, blobPath, sha, compSize)

	out, err := RestoreToCache(store, cacheDir, origPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := FileSHA256(out)
	if err != nil {
		t.Fatal(err)
	}
	if got != sha {
		t.Fatalf("sha mismatch: %s vs %s", got, sha)
	}
}
