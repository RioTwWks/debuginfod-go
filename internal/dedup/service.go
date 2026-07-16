package dedup

import (
	"github.com/your-username/debuginfod-go/internal/config"
	"github.com/your-username/debuginfod-go/internal/storage"
)

// Service — фасад dedup для admin API и scanrunner.
type Service struct {
	store  *storage.Storage
	opts   Options
	cfg    config.DedupConfig
}

// NewService создаёт dedup service.
func NewService(store *storage.Storage, cfg config.DedupConfig, scanPaths []string) *Service {
	return &Service{
		store: store,
		cfg:   cfg,
		opts: Options{
			Store:     store,
			ScanPaths: scanPaths,
			Xdelta:    NewXdelta(cfg.XdeltaPath),
			Projects:  cfg.Projects,
			Workers:   cfg.Workers,
		},
	}
}

// Enabled возвращает true, если dedup включён.
func (s *Service) Enabled() bool {
	return s != nil && s.cfg.Enabled
}

// RunBackfill запускает backfill порциями.
func (s *Service) RunBackfill(project string, batch int, dryRun bool) (BackfillResult, error) {
	opts := s.opts
	opts.DryRun = dryRun
	return RunBackfill(opts, project, batch)
}

// RunIngestAfterScan вызывается после успешного scan.
func (s *Service) RunIngestAfterScan() (BackfillResult, error) {
	return RunIngestAll(s.opts)
}

// Store возвращает хранилище.
func (s *Service) Store() *storage.Storage {
	return s.store
}

// Xdelta возвращает xdelta runner.
func (s *Service) Xdelta() *Xdelta {
	return s.opts.Xdelta
}
