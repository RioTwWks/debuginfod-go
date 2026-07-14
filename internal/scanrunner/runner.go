package scanrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/your-username/debuginfod-go/internal/indexer"
	"github.com/your-username/debuginfod-go/internal/metrics"
)

// Options — параметры фонового индексатора.
type Options struct {
	Indexer    *indexer.Indexer
	Metrics    *metrics.Collector
	Interval   time.Duration
	Enabled    bool
	WebhookURL string
}

// Runner управляет периодическим и ручным scan.
type Runner struct {
	indexer    *indexer.Indexer
	metrics    *metrics.Collector
	interval   time.Duration
	enabled    bool
	webhookURL string
	client     *http.Client

	trigger chan struct{}

	mu       sync.Mutex
	scanning bool
}

// New создаёт Runner.
func New(opts Options) *Runner {
	client := &http.Client{Timeout: 10 * time.Second}
	return &Runner{
		indexer:    opts.Indexer,
		metrics:    opts.Metrics,
		interval:   opts.Interval,
		enabled:    opts.Enabled,
		webhookURL: opts.WebhookURL,
		client:     client,
		trigger:    make(chan struct{}, 1),
	}
}

// Run выполняет scan при старте и далее по таймеру или Trigger.
// При Enabled=false scan не запускается, readiness выставляется сразу.
func (r *Runner) Run(ctx context.Context) {
	if !r.enabled {
		if r.metrics != nil {
			r.metrics.MarkReady()
		}
		slog.Info("scan disabled, serving index from database only")
		<-ctx.Done()
		return
	}

	r.executeScan()

	if r.interval <= 0 {
		slog.Info("periodic rescan disabled", "interval", r.interval)
	} else {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.executeScan()
			case <-r.trigger:
				r.drainTriggers()
				r.executeScan()
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.trigger:
			r.drainTriggers()
			r.executeScan()
		}
	}
}

// Trigger запрашивает внеочередной scan (неблокирующий).
func (r *Runner) Trigger() {
	if !r.enabled {
		return
	}
	select {
	case r.trigger <- struct{}{}:
	default:
	}
}

func (r *Runner) drainTriggers() {
	for len(r.trigger) > 0 {
		<-r.trigger
	}
}

func (r *Runner) executeScan() {
	r.mu.Lock()
	if r.scanning {
		r.mu.Unlock()
		slog.Info("scan already in progress, skipping")
		return
	}
	r.scanning = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.scanning = false
		r.mu.Unlock()
	}()

	if err := r.indexer.Scan(); err != nil {
		slog.Error("index scan", "err", err)
		return
	}

	if r.metrics != nil {
		r.metrics.MarkReady()
		stats := r.metrics.LastScan()
		r.postWebhook(stats)
	}
}

type webhookPayload struct {
	Indexed    int    `json:"indexed"`
	Skipped    int    `json:"skipped"`
	Errors     int    `json:"errors"`
	DurationMs int64  `json:"duration_ms"`
	FinishedAt string `json:"finished_at"`
}

func (r *Runner) postWebhook(stats metrics.ScanStats) {
	if r.webhookURL == "" {
		return
	}
	payload := webhookPayload{
		Indexed:    stats.Indexed,
		Skipped:    stats.Skipped,
		Errors:     stats.Errors,
		DurationMs: stats.Duration.Milliseconds(),
		FinishedAt: stats.Finished.UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("scan webhook marshal", "err", err)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.webhookURL, bytes.NewReader(body))
		if err != nil {
			slog.Warn("scan webhook request", "err", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := r.client.Do(req)
		if err != nil {
			slog.Warn("scan webhook failed", "url", r.webhookURL, "err", err)
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			slog.Warn("scan webhook bad status", "url", r.webhookURL, "status", resp.StatusCode)
			return
		}
		slog.Info("scan webhook delivered", "url", r.webhookURL, "status", resp.StatusCode)
	}()
}
