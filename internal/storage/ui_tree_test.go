package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestUIProjectFromRelativePath(t *testing.T) {
	tests := []struct {
		rel  string
		want string
	}{
		{"Released/QuikServer_16/build_1/foo.debug", "Released/QuikServer_16"},
		{"Unsorted/MyProj/build_2/bar.debug", "Unsorted/MyProj"},
		{"other/path/file.debug", "other"},
	}
	for _, tc := range tests {
		if got := UIProjectFromRelativePath(tc.rel); got != tc.want {
			t.Fatalf("UIProjectFromRelativePath(%q)=%q want %q", tc.rel, got, tc.want)
		}
	}
}

func TestSearchDebugFilesForUI(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "tree.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	scanRoot := t.TempDir()
	_ = store.AddArtifact(ArtifactInput{
		BuildID: "aaa", Type: "debuginfo",
		FilePath: scanRoot + "/Released/ProjA/build_1/libfoo.so.debug",
		GitCommit: "9ae10425c6bbb99c7ee1f71a3941fd7aee058227",
	}, 1)
	_ = store.AddArtifact(ArtifactInput{
		BuildID: "bbb", Type: "executable",
		FilePath: scanRoot + "/Released/ProjA/build_1/bin/quik",
	}, 1)
	_ = store.AddArtifact(ArtifactInput{
		BuildID: "ccc", Type: "debuginfo",
		FilePath: scanRoot + "/Released/ProjB/build_2/libbar.so.debug",
	}, 1)

	ctx := context.Background()
	roots := []string{scanRoot}

	all, err := store.SearchDebugFilesForUI(ctx, roots, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("all debug files=%d want 2", len(all))
	}

	byCommit, err := store.SearchDebugFilesForUI(ctx, roots, "9ae10425", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(byCommit) != 1 || byCommit[0].BuildID != "aaa" {
		t.Fatalf("by commit: %+v", byCommit)
	}

	byPath, err := store.SearchDebugFilesForUI(ctx, roots, "ProjB/build", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(byPath) != 1 || byPath[0].BuildID != "ccc" {
		t.Fatalf("by path: %+v", byPath)
	}

	byName, err := store.SearchDebugFilesForUI(ctx, roots, "libfoo", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(byName) != 1 || byName[0].BuildID != "aaa" {
		t.Fatalf("by name: %+v", byName)
	}
}

func TestBuildUITreeGroupsByCommit(t *testing.T) {
	scanRoot := "/home/ieme/debug_linux"
	commitA := "9ae10425c6bbb99c7ee1f71a3941fd7aee058227"
	commitB := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	records := []ArtifactRecord{
		{
			BuildID: "a", Type: "debuginfo", GitCommit: commitA,
			RelativePath: "Released/ProjA/build_1/sub/libfoo.so.debug",
			Filename:     "libfoo.so.debug",
		},
		{
			BuildID: "b", Type: "debuginfo", GitCommit: commitA,
			RelativePath: "Released/ProjA/build_2/libbar.so.debug",
			Filename:     "libbar.so.debug",
		},
		{
			BuildID: "c", Type: "debuginfo", GitCommit: commitB,
			RelativePath: "Unsorted/Other/build_1/x.debug",
			Filename:     "x.debug",
		},
	}

	tree := BuildUITree([]string{scanRoot}, records)
	if len(tree) != 2 {
		t.Fatalf("commit groups=%d want 2", len(tree))
	}
	if tree[0].Group != "commit" || tree[0].Path != commitA {
		t.Fatalf("first commit=%q", tree[0].Path)
	}
	if len(tree[0].Files) != 2 {
		t.Fatalf("commitA files=%d want 2", len(tree[0].Files))
	}
	if tree[1].Path != commitB || len(tree[1].Files) != 1 {
		t.Fatalf("commitB=%+v", tree[1])
	}
}

func TestBuildUITreeNoCommitUsesProjectDirs(t *testing.T) {
	scanRoot := "/home/ieme/debug_linux"
	records := []ArtifactRecord{
		{
			BuildID: "a", Type: "debuginfo",
			RelativePath: "Released/ProjA/build_1/sub/libfoo.so.debug",
			Filename:     "libfoo.so.debug",
		},
		{
			BuildID: "b", Type: "debuginfo",
			RelativePath: "Released/ProjA/build_2/libbar.so.debug",
			Filename:     "libbar.so.debug",
		},
		{
			BuildID: "c", Type: "debuginfo", GitCommit: "abc123commit",
			RelativePath: "Unsorted/Other/build_1/x.debug",
			Filename:     "x.debug",
		},
	}

	tree := BuildUITree([]string{scanRoot}, records)
	if len(tree) != 2 {
		t.Fatalf("top groups=%d want commit + project", len(tree))
	}
	if tree[0].Group != "commit" || tree[0].Path != "abc123commit" {
		t.Fatalf("commit group=%+v", tree[0])
	}
	if tree[1].Group != "project" || tree[1].Name != "Released/ProjA" {
		t.Fatalf("project group=%+v", tree[1])
	}
	if len(tree[1].Children) != 2 {
		t.Fatalf("ProjA children=%d want 2 (build_1, build_2)", len(tree[1].Children))
	}
}

func TestBuildUITreeNoCommitOnlyProjectDirs(t *testing.T) {
	files := []UITreeFile{
		{Filename: "a.debug", RelativePath: "Released/Proj/a.debug"},
		{Filename: "b.debug", RelativePath: "Released/Proj/b.debug"},
	}
	tree := BuildUITreeFromFiles(files)
	if len(tree) != 1 || tree[0].Group != "project" || tree[0].Name != "Released/Proj" {
		t.Fatalf("tree=%+v", tree)
	}
}
