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
	c.RecordScan(ScanStats{Duration: time.Second, Indexed: 5, Skipped: 10})
	s := c.LastScan()
	if s.Indexed != 5 || s.Skipped != 10 {
		t.Fatalf("scan stats=%+v", s)
	}
}
