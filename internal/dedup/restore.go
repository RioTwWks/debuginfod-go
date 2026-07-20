package dedup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/your-username/debuginfod-go/internal/storage"
)

// RestoreOptions — параметры восстановления delta-файлов.
type RestoreOptions struct {
	Xdelta       *Xdelta
	Objcopy      string
	CompressBase bool
}

// RestoreToCache восстанавливает dedup-файл в cacheDir и возвращает путь к ELF.
func RestoreToCache(store *storage.Storage, cacheDir, filePath string) (string, error) {
	return RestoreToCacheWithOpts(store, RestoreOptions{
		Xdelta:       NewXdelta(""),
		Objcopy:      "objcopy",
		CompressBase: true,
	}, cacheDir, filePath)
}

// RestoreToCacheWithOpts восстанавливает файл с явными опциями (для тестов и service).
func RestoreToCacheWithOpts(store *storage.Storage, opts RestoreOptions, cacheDir, filePath string) (string, error) {
	if opts.Xdelta == nil {
		opts.Xdelta = NewXdelta("")
	}
	if opts.Objcopy == "" {
		opts.Objcopy = "objcopy"
	}

	df, err := store.GetDedupFileByPath(filePath)
	if err != nil {
		if _, statErr := os.Stat(filePath); statErr != nil {
			return "", fmt.Errorf("artifact file missing: %w", statErr)
		}
		return filePath, nil
	}

	switch df.StorageKind {
	case storage.DedupKindCompressed, storage.DedupKindRef:
		return restoreZstdBlob(store, cacheDir, filePath, df)
	case storage.DedupKindFull, storage.DedupKindBase:
		if _, statErr := os.Stat(filePath); statErr == nil {
			return filePath, nil
		}
		return "", fmt.Errorf("dedup file missing: %s", filePath)
	case storage.DedupKindDelta:
		return restoreDelta(store, opts, cacheDir, filePath, df)
	default:
		if _, statErr := os.Stat(filePath); statErr == nil {
			return filePath, nil
		}
		if df.BlobPath != "" {
			return restoreZstdBlob(store, cacheDir, filePath, df)
		}
		return "", fmt.Errorf("dedup file missing: %s", filePath)
	}
}

func restoreZstdBlob(store *storage.Storage, cacheDir, filePath string, df storage.DedupFile) (string, error) {
	_ = store
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
			_ = os.Remove(outPath)
			return "", fmt.Errorf("restored sha256 mismatch")
		}
	}
	return outPath, nil
}

func restoreDelta(store *storage.Storage, opts RestoreOptions, cacheDir, filePath string, df storage.DedupFile) (string, error) {
	if !df.BaseFileID.Valid {
		return "", fmt.Errorf("delta without base_file_id: %s", filePath)
	}

	base, err := store.GetDedupFileByID(df.BaseFileID.Int64)
	if err != nil {
		return "", fmt.Errorf("base file record: %w", err)
	}
	if _, err := os.Stat(base.FilePath); err != nil {
		return "", fmt.Errorf("base file on disk: %w", err)
	}
	if _, err := os.Stat(df.BlobPath); err != nil {
		return "", fmt.Errorf("delta file on disk: %w", err)
	}

	outDir := filepath.Join(cacheDir, "dedup-restored")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}

	cacheName := fmt.Sprintf("%d-%d-%s", df.ID, df.FileBuildNum, filepath.Base(filePath))
	outPath := filepath.Join(outDir, cacheName)

	if info, err := os.Stat(outPath); err == nil {
		if df.OriginalSize > 0 && info.Size() == df.OriginalSize && df.SHA256 != "" {
			got, hashErr := FileSHA256(outPath)
			if hashErr == nil && got == df.SHA256 {
				return outPath, nil
			}
		}
	}

	baseForDecode := base.FilePath
	if opts.CompressBase {
		tmpBase, err := os.CreateTemp(outDir, "dedup-base-*")
		if err != nil {
			return "", err
		}
		tmpBasePath := tmpBase.Name()
		tmpBase.Close()
		defer os.Remove(tmpBasePath)

		if err := DecompressDebugSections(opts.Objcopy, base.FilePath, tmpBasePath); err != nil {
			return "", fmt.Errorf("decompress base for delta: %w", err)
		}
		baseForDecode = tmpBasePath
	}

	if err := opts.Xdelta.Decode(baseForDecode, df.BlobPath, outPath); err != nil {
		return "", err
	}
	if df.SHA256 != "" {
		got, err := FileSHA256(outPath)
		if err != nil {
			return "", err
		}
		if got != df.SHA256 {
			_ = os.Remove(outPath)
			return "", fmt.Errorf("restored sha256 mismatch")
		}
	}
	return outPath, nil
}
