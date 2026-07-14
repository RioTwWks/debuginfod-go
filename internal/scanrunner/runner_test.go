package scanrunner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/your-username/debuginfod-go/internal/indexer"
	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/storage"
)

func TestRunDisabledMarksReady(t *testing.T) {
	store, err := storage.New(t.TempDir() + "/scan.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	collector := metrics.New()
	idx := indexer.NewIndexer(indexer.Options{Storage: store, Workers: 1})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go New(Options{
		Indexer: idx,
		Metrics: collector,
		Enabled: false,
	}).Run(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for !collector.Ready() {
		if time.Now().After(deadline) {
			t.Fatal("ready not set when scan disabled")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWebhookOnScanComplete(t *testing.T) {
	var received atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type=%q", ct)
		}
		received.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store, err := storage.New(t.TempDir() + "/wh.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	collector := metrics.New()
	idx := indexer.NewIndexer(indexer.Options{
		Storage: store,
		Paths:   []string{t.TempDir()},
		Workers: 1,
		Metrics: collector,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go New(Options{
		Indexer:    idx,
		Metrics:    collector,
		Interval:   time.Hour,
		Enabled:    true,
		WebhookURL: server.URL,
	}).Run(ctx)

	deadline := time.Now().Add(3 * time.Second)
	for !received.Load() {
		if time.Now().After(deadline) {
			t.Fatal("webhook not received")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestTriggerCoalesces(t *testing.T) {
	store, err := storage.New(t.TempDir() + "/tr.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	collector := metrics.New()
	idx := indexer.NewIndexer(indexer.Options{
		Storage: store,
		Paths:   []string{t.TempDir()},
		Workers: 1,
		Metrics: collector,
	})

	r := New(Options{
		Indexer:  idx,
		Metrics:  collector,
		Interval: time.Hour,
		Enabled:  true,
	})

	r.executeScan()
	r.Trigger()
	r.Trigger()
	r.Trigger()
	r.drainTriggers()
	r.executeScan()
}
