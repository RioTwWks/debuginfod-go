package benchdedup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectSkipsNonDebug(t *testing.T) {
	root := t.TempDir()
	buildDir := filepath.Join(root, "Released", "Quik", "build_1_2026-01-01")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "lib.so.1.0.0.1.debug"), []byte("elf"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := Collect(CollectOptions{ScanRoots: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("files=%d want 1", len(files))
	}
	if files[0].Project != "Released/Quik" {
		t.Fatalf("project=%q", files[0].Project)
	}
}

func TestBenchmarkGroupSynthetic(t *testing.T) {
	if !NewXdelta3("").Available() {
		t.Skip("xdelta3 not in PATH")
	}
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.bin")
	targetPath := filepath.Join(dir, "target.bin")
	// similar files for delta
	base := make([]byte, 4096)
	for i := range base {
		base[i] = byte(i % 251)
	}
	target := append(append([]byte{}, base...), []byte("extra-tail-data")...)
	if err := os.WriteFile(basePath, base, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, target, 0o644); err != nil {
		t.Fatal(err)
	}

	g := FileGroup{
		Key: GroupKey{Project: "P", FileStem: "lib", Version: "1", Mode: GroupModeStem},
		Files: []DebugFile{
			{Path: basePath, FileBuildNum: 1, Size: int64(len(base))},
			{Path: targetPath, FileBuildNum: 2, Size: int64(len(target))},
		},
	}

	report, err := RunBenchmark(RunOptions{
		WorkDir:       filepath.Join(dir, "work"),
		Algos:         []DiffAlgo{NewXdelta3("")},
		Preprocessors: []Preprocessor{NoPreprocessor{}},
		Groups:        []FileGroup{g},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Scenarios) != 1 {
		t.Fatalf("scenarios=%d", len(report.Scenarios))
	}
	sc := report.Scenarios[0]
	if sc.Skipped != "" {
		t.Fatalf("skipped: %s", sc.Skipped)
	}
	if sc.Summary.VerifyFailures > 0 || sc.Summary.ErrorCount > 0 {
		t.Fatalf("failures verify=%d errors=%d", sc.Summary.VerifyFailures, sc.Summary.ErrorCount)
	}
	if sc.Summary.SavingsPct <= 0 {
		t.Fatalf("expected positive savings, got %f", sc.Summary.SavingsPct)
	}
}
