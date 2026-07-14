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

func TestSearchBuildIDForUI(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "search.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_ = store.AddArtifact(ArtifactInput{BuildID: "deadbeef", Type: "executable", FilePath: "/a"}, 1)
	_ = store.AddArtifact(ArtifactInput{BuildID: "deadcafe", Type: "debuginfo", FilePath: "/b"}, 1)
	_ = store.AddArtifact(ArtifactInput{BuildID: "cafebabe", Type: "executable", FilePath: "/c"}, 1)

	ctx := context.Background()
	results, err := store.SearchBuildIDForUI(ctx, "dead", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results=%d, want 2", len(results))
	}

	all, err := store.SearchBuildIDForUI(ctx, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("empty query limit=2: got %d", len(all))
	}
}

func TestSearchMetadataPagination(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "page.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for _, id := range []string{"aaaa", "bbbb", "cccc", "dddd"} {
		_ = store.AddArtifact(ArtifactInput{
			BuildID: id, Type: "executable", FilePath: "/usr/bin/" + id,
		}, 1)
	}

	ctx := context.Background()
	resp, err := store.SearchMetadataQuery(ctx, MetadataQuery{
		Key: "glob", Value: "/usr/bin/*", Offset: 0, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 || resp.Complete {
		t.Fatalf("page1: %+v", resp)
	}
	if resp.NextOffset != 2 {
		t.Fatalf("next_offset=%d", resp.NextOffset)
	}

	resp, err = store.SearchMetadataQuery(ctx, MetadataQuery{
		Key: "glob", Value: "/usr/bin/*", Offset: 2, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("page2: %+v", resp)
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
