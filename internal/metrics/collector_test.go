package metrics

import (
	"testing"
	"time"
)

func TestCollectorHTTP(t *testing.T) {
	c := New()
	c.RecordHTTP(200, 100)
	c.RecordHTTP(404, 0)
	c.RecordHTTP(500, 50)
	if c.HTTPRequests() != 3 {
		t.Fatalf("requests=%d", c.HTTPRequests())
	}
	if c.HTTP2xx() != 1 || c.HTTP4xx() != 1 || c.HTTP5xx() != 1 {
		t.Fatalf("status counters wrong")
	}
}

func TestCollectorScan(t *testing.T) {
	c := New()
	if c.Ready() {
		t.Fatal("expected not ready before scan")
	}
	c.RecordScan(ScanStats{Duration: time.Second, Indexed: 5, Skipped: 10})
	s := c.LastScan()
	if s.Indexed != 5 || s.Skipped != 10 {
		t.Fatalf("scan stats=%+v", s)
	}
	if !c.Ready() {
		t.Fatal("expected ready after RecordScan")
	}
}

func TestCollectorScanProgress(t *testing.T) {
	c := New()
	p := c.ScanProgress()
	if p.Running {
		t.Fatal("expected not running initially")
	}

	c.BeginScan(ScanPhaseIndexing)
	c.UpdateIndexingProgress(3, 7, 1)
	c.SetScanCurrentPath("/tmp/foo.so")

	p = c.ScanProgress()
	if !p.Running || p.Phase != ScanPhaseIndexing {
		t.Fatalf("progress=%+v", p)
	}
	if p.Indexed != 3 || p.Skipped != 7 || p.Errors != 1 {
		t.Fatalf("counters=%+v", p)
	}
	if p.CurrentPath != "/tmp/foo.so" {
		t.Fatalf("path=%q", p.CurrentPath)
	}

	c.SetScanPhase(ScanPhaseDedup)
	c.SetDedupGroupsTotal(10)
	c.UpdateDedupProgress(4, 2, 1, 0, 1000, 400)

	p = c.ScanProgress()
	if p.Phase != ScanPhaseDedup {
		t.Fatalf("phase=%q", p.Phase)
	}
	if p.DedupGroupsTotal != 10 || p.DedupGroupsProcessed != 4 {
		t.Fatalf("dedup groups=%+v", p)
	}
	if p.DedupBytesBefore != 1000 || p.DedupBytesAfter != 400 {
		t.Fatalf("dedup bytes=%+v", p)
	}

	c.EndScan()
	p = c.ScanProgress()
	if p.Running {
		t.Fatal("expected not running after EndScan")
	}
}
