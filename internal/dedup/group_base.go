package dedup

import (
	"errors"
	"fmt"
	"os"

	"github.com/your-username/debuginfod-go/internal/storage"
)

func findGroupBase(store *storage.Storage, target storage.DedupFile) (storage.DedupFile, error) {
	norm := NormalizeDedupGroupProject(target.ProjectName)

	bases, err := store.ListDedupBasesByStem(target.FileStem, 64)
	if err != nil {
		return storage.DedupFile{}, err
	}
	fulls, err := store.ListDedupFullsByStem(target.FileStem, 64)
	if err != nil {
		return storage.DedupFile{}, err
	}

	candidates := make([]storage.DedupFile, 0, len(bases)+len(fulls))
	candidates = append(candidates, bases...)
	candidates = append(candidates, fulls...)

	if base, ok := bestGroupCandidate(candidates, target.ID, norm); ok {
		return base, nil
	}
	return storage.DedupFile{}, storage.ErrNotFound
}

// bestGroupCandidate выбирает кандидата в base с минимальным file_build_num в группе проекта.
func bestGroupCandidate(candidates []storage.DedupFile, skipID int64, normProject string) (storage.DedupFile, bool) {
	var best storage.DedupFile
	found := false
	for _, c := range candidates {
		if c.ID == skipID {
			continue
		}
		if NormalizeDedupGroupProject(c.ProjectName) != normProject {
			continue
		}
		if !found || c.FileBuildNum < best.FileBuildNum ||
			(c.FileBuildNum == best.FileBuildNum && c.ID < best.ID) {
			best = c
			found = true
		}
	}
	return best, found
}

// prepareBaseForDelta гарантирует, что base готов для xdelta (preprocess + kind=base).
func prepareBaseForDelta(opts Options, existing storage.DedupFile) (storage.DedupFile, error) {
	if existing.StorageKind == storage.DedupKindBase {
		return existing, nil
	}
	if existing.StorageKind != storage.DedupKindFull {
		return storage.DedupFile{}, fmt.Errorf("unexpected storage kind %q", existing.StorageKind)
	}
	if _, err := os.Stat(existing.FilePath); err != nil {
		return storage.DedupFile{}, fmt.Errorf("base missing: %w", err)
	}
	if err := opts.Preprocessor.ApplyInPlace(existing.FilePath); err != nil {
		return storage.DedupFile{}, err
	}
	baseSHA, err := FileSHA256(existing.FilePath)
	if err != nil {
		return storage.DedupFile{}, err
	}
	baseSize, err := fileSizeOnDisk(existing.FilePath)
	if err != nil {
		return storage.DedupFile{}, err
	}
	if err := opts.Store.MarkDedupFileDone(
		existing.ID, storage.DedupKindBase, 0, "", baseSHA, baseSize,
	); err != nil {
		return storage.DedupFile{}, err
	}
	return opts.Store.GetDedupFileByID(existing.ID)
}

func isNotFound(err error) bool {
	return errors.Is(err, storage.ErrNotFound)
}
