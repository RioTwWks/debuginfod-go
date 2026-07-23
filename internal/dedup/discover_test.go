package dedup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/your-username/debuginfod-go/internal/storage"
)

func TestDiscoverNestedLayout(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "Released", "QuikServer_16.0_Common_Linux")
	buildDir := filepath.Join(projectDir, "build_2_2026-04_17_15_58_51")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	debugPath := filepath.Join(buildDir, "lib.so.19.1.5.2899.debug")
	if err := os.WriteFile(debugPath, []byte("fake-debug"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := storage.New(filepath.Join(t.TempDir(), "discover.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	n, err := Discover(store, []string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("registered=%d want 1", n)
	}

	projects, err := store.ListDedupProjectNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0] != "Released/QuikServer_16.0_Common_Linux" {
		t.Fatalf("projects=%v", projects)
	}
}

func TestDiscoverProjectFilter(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"Released/QuikServer/build_1_2026-01-01",
		"Other/Front/build_1_2026-01-01",
	} {
		dir := filepath.Join(root, rel)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "lib.so.1.0.0.1.debug"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	store, err := storage.New(filepath.Join(t.TempDir(), "filter.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	n, err := Discover(store, []string{root}, []string{"Released/QuikServer"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("registered=%d want 1", n)
	}
	projects, err := store.ListDedupProjectNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0] != "Released/QuikServer" {
		t.Fatalf("projects=%v", projects)
	}
}

func TestDiscoverArbitraryDebugName(t *testing.T) {
	root := t.TempDir()
	buildDir := filepath.Join(root, "proj", "build_1_2026-01-01")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	debugPath := filepath.Join(buildDir, "totally-custom-name.debug")
	if err := os.WriteFile(debugPath, []byte("fake-debug"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := storage.New(filepath.Join(t.TempDir(), "arbitrary.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	n, err := Discover(store, []string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("registered=%d want 1", n)
	}
}

func TestDiscoverQuikHyphenFilename(t *testing.T) {
	root := t.TempDir()
	buildDir := filepath.Join(root, "Released", "Quik", "build_1_2026-01-01")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	debugPath := filepath.Join(buildDir, "quik-16.0.0.10.debug")
	if err := os.WriteFile(debugPath, []byte("fake-debug"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := storage.New(filepath.Join(t.TempDir(), "quik-hyphen.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	n, err := Discover(store, []string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("registered=%d want 1", n)
	}

	got, err := store.GetDedupFileByPath(debugPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.FileStem != "quik" || got.Version != "16.0.0" || got.FileBuildNum != 10 {
		t.Fatalf("unexpected record: %+v", got)
	}
}

func TestDiscoverNestedDebugInSubdirs(t *testing.T) {
	root := t.TempDir()
	buildDir := filepath.Join(root, "Released", "Qt_Library", "qt", "build_1_2026-01-01")
	nested := filepath.Join(buildDir, "lib", "x86_64", "release")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	debugPath := filepath.Join(nested, "libQt5Core.so.5.15.2.100.debug")
	if err := os.WriteFile(debugPath, []byte("fake-debug"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := storage.New(filepath.Join(t.TempDir(), "nested.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	n, err := Discover(store, []string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("registered=%d want 1", n)
	}

	got, err := store.GetDedupFileByPath(debugPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.FilePath != debugPath {
		t.Fatalf("path=%q", got.FilePath)
	}

	projects, err := store.ListDedupProjectNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0] != "Released/Qt_Library/qt" {
		t.Fatalf("projects=%v", projects)
	}
}

func TestDiscoverSkipsUnchangedDone(t *testing.T) {
	root := t.TempDir()
	buildDir := filepath.Join(root, "proj", "build_1_2026-01-01")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	debugPath := filepath.Join(buildDir, "lib.so.1.0.0.1.debug")
	content := []byte("fake-debug")
	if err := os.WriteFile(debugPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := storage.New(filepath.Join(t.TempDir(), "incr.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	n1, err := Discover(store, []string{root}, nil)
	if err != nil || n1 != 1 {
		t.Fatalf("first discover n=%d err=%v", n1, err)
	}
	f, err := store.GetDedupFileByPath(debugPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDedupFileDone(f.ID, storage.DedupKindFull, 0, "", "sha", int64(len(content))); err != nil {
		t.Fatal(err)
	}

	n2, err := Discover(store, []string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("second discover registered=%d want 0 unchanged", n2)
	}
}

func TestProjectNameForBuildDir(t *testing.T) {
	root := "/data/debug_linux"
	build := "/data/debug_linux/Released/Quik/build_1_2026-01-01"
	got := projectNameForBuildDir(root, build)
	if got != "Released/Quik" {
		t.Fatalf("got %q", got)
	}
}
