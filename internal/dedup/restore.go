package dedup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/your-username/debuginfod-go/internal/storage"
)

// RestoreToCache восстанавливает delta-файл в cacheDir и возвращает путь.
func RestoreToCache(store *storage.Storage, xdelta *Xdelta, cacheDir, filePath string) (string, error) {
	df, err := store.GetDedupFileByPath(filePath)
	if err != nil {
		if _, statErr := os.Stat(filePath); statErr != nil {
			return "", fmt.Errorf("artifact file missing: %w", statErr)
		}
		return filePath, nil
	}
	if df.StorageKind != storage.DedupKindDelta {
		if _, statErr := os.Stat(filePath); statErr == nil {
			return filePath, nil
		}
		return "", fmt.Errorf("dedup file missing: %s", filePath)
	}
	if xdelta == nil {
		xdelta = NewXdelta("xdelta3")
	}

	base, err := store.GetDedupFileByID(df.BaseFileID.Int64)
	if err != nil {
		return "", fmt.Errorf("base file record: %w", err)
	}
	if _, err := os.Stat(base.FilePath); err != nil {
		return "", fmt.Errorf("base file on disk: %w", err)
	}
	if _, err := os.Stat(df.DeltaPath); err != nil {
		return "", fmt.Errorf("delta file on disk: %w", err)
	}

	outDir := filepath.Join(cacheDir, "dedup-restored")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}

	baseInfo, _ := os.Stat(base.FilePath)
	deltaInfo, _ := os.Stat(df.DeltaPath)
	cacheName := fmt.Sprintf("%d-%d-%s", df.ID, df.FileBuildNum, filepath.Base(filePath))
	outPath := filepath.Join(outDir, cacheName)

	if info, err := os.Stat(outPath); err == nil {
		if baseInfo != nil && deltaInfo != nil &&
			info.Size() == df.OriginalSize {
			return outPath, nil
		}
	}

	if err := xdelta.Decode(base.FilePath, df.DeltaPath, outPath); err != nil {
		return "", err
	}
	if df.SHA256 != "" {
		got, err := FileSHA256(outPath)
		if err != nil {
			return "", err
		}
		if got != df.SHA256 {
			os.Remove(outPath)
			return "", fmt.Errorf("restored sha256 mismatch")
		}
	}
	return outPath, nil
}
