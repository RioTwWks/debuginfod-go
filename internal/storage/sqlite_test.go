package storage

import (
	"context"
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

	if err := store.AddArtifact(ArtifactInput{
		BuildID:  "abc123",
		Type:     "executable",
		FilePath: "/bin/ls",
	}, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.AddSource("abc123", "/src/main.c", "/src/main.c", 1); err != nil {
		t.Fatal(err)
	}

	path, err := store.GetArtifactPath("abc123", "executable")
	if err != nil || path != "/bin/ls" {
		t.Fatalf("GetArtifactPath = %q, %v", path, err)
	}

	src, err := store.GetSource("abc123", "/src/main.c")
	if err != nil || src.FilePath != "/src/main.c" {
		t.Fatalf("GetSource = %+v, %v", src, err)
	}

	ok, err := store.HasBuildID("abc123")
	if err != nil || !ok {
		t.Fatalf("HasBuildID = %v, %v", ok, err)
	}
}

func TestSearchMetadataGlob(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "meta.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_ = store.AddArtifact(ArtifactInput{
		BuildID:  "deadbeef",
		Type:     "executable",
		FilePath: "/usr/bin/hello",
	}, 1)
	_ = store.AddArtifact(ArtifactInput{
		BuildID:     "cafebabe",
		Type:        "executable",
		FilePath:    "usr/bin/world",
		ArchivePath: "/packages/app.rpm",
		MemberPath:  "usr/bin/world",
	}, 1)

	ctx := context.Background()
	resp, err := store.SearchMetadata(ctx, "glob", "/usr/bin/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("glob results = %d, want 1", len(resp.Results))
	}

	resp, err = store.SearchMetadata(ctx, "buildid", "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].BuildID != "deadbeef" {
		t.Fatalf("buildid search failed: %+v", resp.Results)
	}
}

func TestNeedsScanIncremental(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "scan.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	path := "/tmp/hello"
	needs, err := store.NeedsScan(path, 100, 1024)
	if err != nil || !needs {
		t.Fatalf("initial NeedsScan = %v, %v", needs, err)
	}

	if err := store.MarkScanned(path, 100, 1024, "elf"); err != nil {
		t.Fatal(err)
	}

	needs, err = store.NeedsScan(path, 100, 1024)
	if err != nil || needs {
		t.Fatalf("unchanged NeedsScan = %v, want false", needs)
	}

	needs, err = store.NeedsScan(path, 200, 1024)
	if err != nil || !needs {
		t.Fatalf("mtime changed NeedsScan = %v, want true", needs)
	}
}

func TestSearchMetadataGlobNoNestedMatch(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "glob.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_ = store.AddArtifact(ArtifactInput{
		BuildID:  "aaa",
		Type:     "executable",
		FilePath: "/usr/bin/sub/hello",
	}, 1)

	resp, err := store.SearchMetadata(context.Background(), "glob", "/usr/bin/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("nested path should not match /usr/bin/*: %+v", resp.Results)
	}
}
