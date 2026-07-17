package dedup

import (
	"testing"

	"github.com/your-username/debuginfod-go/internal/storage"
)

func TestGroupFilesBaseSelection(t *testing.T) {
	files := []storage.DedupFile{
		{ID: 1, ProjectName: "QuikServer", FileStem: "lib.so", Version: "19.1.5", FileBuildNum: 3000, CommitTag: "DEVOPS-110"},
		{ID: 2, ProjectName: "QuikServer", FileStem: "lib.so", Version: "19.1.5", FileBuildNum: 2899, CommitTag: "DEVOPS-110"},
	}
	groups := GroupFiles(files)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	for _, g := range groups {
		if len(g) != 2 {
			t.Fatalf("expected 2 files in group")
		}
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
		Xdelta:    NewXdelta("xdelta3"),
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
