package dedup

import "testing"

func TestFileWorkersFor(t *testing.T) {
	if got := fileWorkersFor(Options{FileWorkers: 6}); got != 6 {
		t.Fatalf("FileWorkers: got %d want 6", got)
	}
	if got := fileWorkersFor(Options{Workers: 5}); got != 10 {
		t.Fatalf("Workers*2: got %d want 10", got)
	}
	if got := fileWorkersFor(Options{}); got != 8 {
		t.Fatalf("default: got %d want 8", got)
	}
}
