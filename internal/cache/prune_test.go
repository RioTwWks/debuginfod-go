package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneRemovesOldest(t *testing.T) {
	tmp := t.TempDir()
	old := filepath.Join(tmp, "old.bin")
	newf := filepath.Join(tmp, "new.bin")
	_ = os.WriteFile(old, make([]byte, 100), 0o644)
	_ = os.WriteFile(newf, make([]byte, 100), 0o644)

	removed, _, err := Prune(tmp, 150)
	if err != nil {
		t.Fatal(err)
	}
	if removed == 0 {
		t.Fatal("expected files removed")
	}
	size, _ := DirSize(tmp)
	if size > 150 {
		t.Fatalf("size=%d want <=150", size)
	}
}
