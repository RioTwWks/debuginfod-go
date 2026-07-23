package dedup

import (
	"errors"

	"github.com/your-username/debuginfod-go/internal/storage"
)

func findGroupBase(store *storage.Storage, target storage.DedupFile) (storage.DedupFile, error) {
	norm := NormalizeDedupGroupProject(target.ProjectName)
	candidates, err := store.ListDedupBasesByStem(target.FileStem, 64)
	if err != nil {
		return storage.DedupFile{}, err
	}
	for _, base := range candidates {
		if base.ID == target.ID {
			continue
		}
		if NormalizeDedupGroupProject(base.ProjectName) == norm {
			return base, nil
		}
	}
	return storage.DedupFile{}, storage.ErrNotFound
}

func isNotFound(err error) bool {
	return errors.Is(err, storage.ErrNotFound)
}
