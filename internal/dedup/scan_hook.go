package dedup

import (
	"log/slog"
	"time"

	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/storage"
)

// ScanHook адаптирует Service к scanrunner.DedupAfterScan.
type ScanHook struct {
	svc     *Service
	metrics *metrics.Collector
}

// NewScanHook создаёт hook или nil.
func NewScanHook(svc *Service, m *metrics.Collector) *ScanHook {
	if svc == nil || !svc.Enabled() {
		return nil
	}
	return &ScanHook{svc: svc, metrics: m}
}

// RunIngestAfterScan запускает ingest после scan.
func (h *ScanHook) RunIngestAfterScan() error {
	if h == nil || h.svc == nil {
		return nil
	}
	start := time.Now()
	result, err := h.svc.RunIngestAfterScanWithMetrics(h.metrics)
	if err != nil {
		return err
	}
	slog.Info("dedup ingest complete",
		"registered", result.FilesRegistered,
		"compressed", result.FilesCompressed,
		"dedup_ref", result.FilesDedupRef,
		"skipped", result.FilesSkipped,
		"errors", result.Errors,
		"bytes_before", result.BytesBefore,
		"bytes_after", result.BytesAfter,
	)
	rec := storage.DedupRunRecord{
		FinishedAt:         time.Now(),
		DurationMs:         time.Since(start).Milliseconds(),
		BuildDirsProcessed: result.BuildDirsProcessed,
		FilesRegistered:    result.FilesRegistered,
		FilesCompressed:    result.FilesCompressed,
		FilesDedupRef:      result.FilesDedupRef,
		FilesSkipped:       result.FilesSkipped,
		Errors:             result.Errors,
		BytesBefore:        result.BytesBefore,
		BytesAfter:         result.BytesAfter,
	}
	if err := h.svc.Store().InsertDedupRun(rec); err != nil {
		slog.Warn("dedup run history", "err", err)
	}
	return nil
}
