package dedup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/your-username/debuginfod-go/internal/storage"
)

// RestoreToCache распаковывает сжатый/CAS blob в cacheDir и возвращает путь к ELF.
func RestoreToCache(store *storage.Storage, cacheDir, filePath string) (string, error) {
	df, err := store.GetDedupFileByPath(filePath)
	if err != nil {
		if _, statErr := os.Stat(filePath); statErr != nil {
			return "", fmt.Errorf("artifact file missing: %w", statErr)
		}
		return filePath, nil
	}

	switch df.StorageKind {
	case storage.DedupKindCompressed, storage.DedupKindRef:
		// fall through to decompress
	case storage.DedupKindFull:
		if _, statErr := os.Stat(filePath); statErr == nil {
			return filePath, nil
		}
		return "", fmt.Errorf("dedup file missing: %s", filePath)
	default:
		// legacy xdelta records: оригинал мог остаться на диске
		if _, statErr := os.Stat(filePath); statErr == nil {
			return filePath, nil
		}
		if df.BlobPath != "" {
			break
		}
		return "", fmt.Errorf("dedup file missing: %s", filePath)
	}

	if df.BlobPath == "" {
		return "", fmt.Errorf("blob path empty for %s", filePath)
	}
	if _, err := os.Stat(df.BlobPath); err != nil {
		return "", fmt.Errorf("blob on disk: %w", err)
	}

	outDir := filepath.Join(cacheDir, "dedup-restored")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}

	cacheName := fmt.Sprintf("%d-%s", df.ID, filepath.Base(filePath))
	outPath := filepath.Join(outDir, cacheName)

	if info, err := os.Stat(outPath); err == nil {
		if df.OriginalSize > 0 && info.Size() == df.OriginalSize {
			if df.SHA256 != "" {
				got, hashErr := FileSHA256(outPath)
				if hashErr == nil && got == df.SHA256 {
					return outPath, nil
				}
			} else {
				return outPath, nil
			}
		}
	}

	if err := DecompressFileTo(df.BlobPath, outPath); err != nil {
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
