package webapi

import (
	"github.com/your-username/debuginfod-go/internal/dedup"
	"github.com/your-username/debuginfod-go/internal/storage"
)

// DedupAdapter адаптирует dedup.Service к HTTP-слою.
type DedupAdapter struct {
	svc *dedup.Service
}

// NewDedupAdapter создаёт адаптер или nil, если service не задан.
func NewDedupAdapter(svc *dedup.Service) *DedupAdapter {
	if svc == nil {
		return nil
	}
	return &DedupAdapter{svc: svc}
}

// RestoreToCache реализует DedupRestorer.
func (a *DedupAdapter) RestoreToCache(cacheDir, filePath string) (string, error) {
	if a == nil || a.svc == nil {
		return filePath, nil
	}
	return a.svc.RestoreToCache(cacheDir, filePath)
}

// RunBackfill реализует DedupRunner.
func (a *DedupAdapter) RunBackfill(project string, batch int, dryRun bool) (dedup.BackfillResult, error) {
	return a.svc.RunBackfill(project, batch, dryRun)
}

// Store возвращает storage для тестов.
func (a *DedupAdapter) Store() *storage.Storage {
	if a == nil {
		return nil
	}
	return a.svc.Store()
}
