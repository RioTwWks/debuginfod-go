package dedup

import "log/slog"

// ScanHook адаптирует Service к scanrunner.DedupAfterScan.
type ScanHook struct {
	svc *Service
}

// NewScanHook создаёт hook или nil.
func NewScanHook(svc *Service) *ScanHook {
	if svc == nil || !svc.Enabled() {
		return nil
	}
	return &ScanHook{svc: svc}
}

// RunIngestAfterScan запускает ingest после scan.
func (h *ScanHook) RunIngestAfterScan() error {
	if h == nil || h.svc == nil {
		return nil
	}
	result, err := h.svc.RunIngestAfterScan()
	if err != nil {
		return err
	}
	slog.Info("dedup ingest complete",
		"registered", result.FilesRegistered,
		"compressed", result.FilesCompressed,
		"skipped", result.FilesSkipped,
		"errors", result.Errors,
	)
	return nil
}
