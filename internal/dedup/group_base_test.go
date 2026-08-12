package dedup

import (
	"testing"

	"github.com/your-username/debuginfod-go/internal/storage"
)

func TestFindGroupBaseFromFull(t *testing.T) {
	store, err := storage.New(t.TempDir() + "/group-base.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	pid, _ := store.EnsureDedupProject("Released/Quik")
	bid, _ := store.UpsertDedupBuildDir(pid, "/tmp/build_1", 1)
	fullID, _ := store.UpsertDedupFile(storage.DedupFile{
		BuildDirID: bid, FilePath: "/tmp/build_1/lib.so.1.0.0.100.debug",
		Filename: "lib.so.1.0.0.100.debug", FileStem: "lib.so",
		FileBuildNum: 100, OriginalSize: 100,
	})
	_ = store.MarkDedupFileDone(fullID, storage.DedupKindFull, 0, "", "sha", 100)

	target := storage.DedupFile{
		ID: 2, ProjectName: "Released/Quik", FileStem: "lib.so", FileBuildNum: 101,
	}
	got, err := findGroupBase(store, target)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != fullID {
		t.Fatalf("base id=%d want %d", got.ID, fullID)
	}
	if got.StorageKind != storage.DedupKindFull {
		t.Fatalf("kind=%s want full", got.StorageKind)
	}
}

func TestFindGroupBasePrefersExistingBase(t *testing.T) {
	store, err := storage.New(t.TempDir() + "/group-base-pref.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	pid, _ := store.EnsureDedupProject("Released/Quik")
	bid1, _ := store.UpsertDedupBuildDir(pid, "/tmp/build_1", 1)
	bid2, _ := store.UpsertDedupBuildDir(pid, "/tmp/build_2", 2)

	fullID, _ := store.UpsertDedupFile(storage.DedupFile{
		BuildDirID: bid1, FilePath: "/tmp/build_1/lib.so.1.0.0.100.debug",
		Filename: "lib.so.1.0.0.100.debug", FileStem: "lib.so",
		FileBuildNum: 100, OriginalSize: 100,
	})
	_ = store.MarkDedupFileDone(fullID, storage.DedupKindFull, 0, "", "sha-full", 100)

	baseID, _ := store.UpsertDedupFile(storage.DedupFile{
		BuildDirID: bid2, FilePath: "/tmp/build_2/lib.so.1.0.0.200.debug",
		Filename: "lib.so.1.0.0.200.debug", FileStem: "lib.so",
		FileBuildNum: 200, OriginalSize: 200,
	})
	_ = store.MarkDedupFileDone(baseID, storage.DedupKindBase, 0, "", "sha-base", 200)

	target := storage.DedupFile{
		ID: 3, ProjectName: "Released/Quik", FileStem: "lib.so", FileBuildNum: 201,
	}
	got, err := findGroupBase(store, target)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != fullID {
		t.Fatalf("base id=%d want earliest full %d", got.ID, fullID)
	}
	if got.StorageKind != storage.DedupKindFull {
		t.Fatalf("kind=%s want full", got.StorageKind)
	}
}
