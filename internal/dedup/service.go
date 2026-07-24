package dedup

import (
	"log/slog"
	"time"

	"github.com/your-username/debuginfod-go/internal/config"
	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/storage"
)

// Service — фасад dedup для admin API и scanrunner.
type Service struct {
	store        *storage.Storage
	opts         Options
	cfg          config.DedupConfig
	restoreOpts  RestoreOptions
}

// NewService создаёт dedup service.
func NewService(store *storage.Storage, cfg config.DedupConfig, scanPaths []string, blobDir string) *Service {
	_ = blobDir // legacy zstd CAS; сохраняем сигнатуру для main.go
	tools := ToolPaths{Dwz: cfg.DwzPath, Objcopy: cfg.ObjcopyPath}
	return &Service{
		store: store,
		cfg:   cfg,
		opts: Options{
			Store:        store,
			ScanPaths:    scanPaths,
			Xdelta:       NewXdelta(cfg.XdeltaPath),
			Preprocessor: ResolvePreprocessor(cfg.Strategy, tools),
			ObjcopyZstd:  NewObjcopyZstd(cfg.ObjcopyPath),
			CompressBase: cfg.CompressBase,
			Projects:     cfg.Projects,
			Workers:      cfg.Workers,
			FileWorkers:  cfg.FileWorkers,
		},
		restoreOpts: RestoreOptions{
			Xdelta:       NewXdelta(cfg.XdeltaPath),
			Objcopy:      cfg.ObjcopyPath,
			CompressBase: cfg.CompressBase,
		},
	}
}

// Enabled возвращает true, если dedup включён.
func (s *Service) Enabled() bool {
	return s != nil && s.cfg.Enabled
}

// RestoreToCache восстанавливает dedup-файл для HTTP/GDB.
func (s *Service) RestoreToCache(cacheDir, filePath string) (string, error) {
	return RestoreToCacheWithOpts(s.store, s.restoreOpts, cacheDir, filePath)
}

// RunBackfill запускает backfill порциями.
func (s *Service) RunBackfill(project string, batch int, dryRun bool) (BackfillResult, error) {
	start := time.Now()
	opts := s.opts
	opts.DryRun = dryRun
	result, err := RunBackfill(opts, project, batch)
	if err != nil {
		return result, err
	}
	if s.store != nil && !dryRun {
		rec := storage.DedupRunRecord{
			FinishedAt:         time.Now(),
			DurationMs:         time.Since(start).Milliseconds(),
			Project:            project,
			DryRun:             dryRun,
			BuildDirsProcessed: result.BuildDirsProcessed,
			FilesRegistered:    result.FilesRegistered,
			FilesCompressed:    result.FilesCompressed,
			FilesDedupRef:      result.FilesDedupRef,
			FilesSkipped:       result.FilesSkipped,
			Errors:             result.Errors,
			BytesBefore:        result.BytesBefore,
			BytesAfter:         result.BytesAfter,
		}
		if err := s.store.InsertDedupRun(rec); err != nil {
			slog.Warn("dedup run history", "err", err)
		}
	}
	return result, nil
}

// RunIngestAfterScan вызывается после успешного scan.
func (s *Service) RunIngestAfterScan() (BackfillResult, error) {
	return RunIngestAll(s.opts)
}

// RunIngestAfterScanWithMetrics запускает ingest с публикацией прогресса.
func (s *Service) RunIngestAfterScanWithMetrics(m *metrics.Collector) (BackfillResult, error) {
	opts := s.opts
	opts.Metrics = m
	return RunIngestAll(opts)
}

// Store возвращает хранилище.
func (s *Service) Store() *storage.Storage {
	return s.store
}
