package dedup

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/your-username/debuginfod-go/internal/storage"
)

// Options — параметры dedup pipeline (zstd + CAS).
type Options struct {
	Store     *storage.Storage
	ScanPaths []string
	BlobStore *BlobStore
	Projects  []string
	Workers   int
	DryRun    bool
}

// BackfillResult — итог backfill/ingest.
type BackfillResult struct {
	BuildDirsProcessed int   `json:"build_dirs_processed"`
	FilesRegistered    int   `json:"files_registered"`
	GroupsProcessed    int   `json:"groups_processed"`
	FilesCompressed    int   `json:"files_compressed"`
	FilesDedupRef        int   `json:"files_dedup_ref"`
	FilesSkipped       int   `json:"files_skipped"`
	Errors             int   `json:"errors"`
	BytesBefore        int64 `json:"bytes_before"`
	BytesAfter         int64 `json:"bytes_after"`
	DryRun             bool  `json:"dry_run"`
}

// RunBackfill обрабатывает pending build dirs порциями.
func RunBackfill(opts Options, project string, batch int) (BackfillResult, error) {
	if batch <= 0 {
		batch = 50
	}
	var result BackfillResult
	result.DryRun = opts.DryRun

	n, err := Discover(opts.Store, opts.ScanPaths, filterProjects(opts.Projects, project))
	if err != nil {
		return result, err
	}
	result.FilesRegistered = n

	dirs, err := opts.Store.ListPendingBuildDirs(project, batch)
	if err != nil {
		return result, err
	}
	if len(dirs) == 0 {
		return result, nil
	}

	dirIDs := make([]int64, len(dirs))
	for i, d := range dirs {
		dirIDs[i] = d.ID
		if !opts.DryRun {
			_ = opts.Store.SetDedupBuildDirStatus(d.ID, storage.DedupStatusProcessing)
		}
	}

	files, err := opts.Store.ListPendingDedupFiles(dirIDs)
	if err != nil {
		return result, err
	}

	compressed, dedupRef, skipped, errs, bytesBefore, bytesAfter := processFiles(opts, files)
	result.GroupsProcessed = len(files)
	result.FilesCompressed = compressed
	result.FilesDedupRef = dedupRef
	result.FilesSkipped = skipped
	result.Errors = errs
	result.BytesBefore = bytesBefore
	result.BytesAfter = bytesAfter
	result.BuildDirsProcessed = len(dirs)

	if !opts.DryRun {
		for _, d := range dirs {
			if err := opts.Store.FinishBuildDirIfDone(d.ID); err != nil {
				slog.Warn("dedup finish build dir", "id", d.ID, "err", err)
			}
		}
	}
	return result, nil
}

// RunIngestForProject обрабатывает все pending файлы проекта (после scan).
func RunIngestForProject(opts Options, project string) (BackfillResult, error) {
	var result BackfillResult
	result.DryRun = opts.DryRun

	filter := filterProjects(opts.Projects, project)
	n, err := Discover(opts.Store, opts.ScanPaths, filter)
	if err != nil {
		return result, err
	}
	result.FilesRegistered = n

	return ingestProjectPending(opts, project, &result)
}

// RunIngestAll запускает ingest для всех обнаруженных проектов.
func RunIngestAll(opts Options) (BackfillResult, error) {
	var total BackfillResult
	total.DryRun = opts.DryRun

	n, err := Discover(opts.Store, opts.ScanPaths, opts.Projects)
	if err != nil {
		return total, err
	}
	total.FilesRegistered = n

	files, err := opts.Store.ListAllPendingDedupFiles()
	if err != nil {
		return total, err
	}
	files = filterFilesByProjects(files, opts.Projects)
	if len(files) == 0 {
		return total, nil
	}

	compressed, dedupRef, skipped, errs, bytesBefore, bytesAfter := processFiles(opts, files)
	total.GroupsProcessed = len(files)
	total.FilesCompressed = compressed
	total.FilesDedupRef = dedupRef
	total.FilesSkipped = skipped
	total.Errors = errs
	total.BytesBefore = bytesBefore
	total.BytesAfter = bytesAfter

	if !opts.DryRun {
		seen := make(map[int64]struct{})
		for _, f := range files {
			if _, ok := seen[f.BuildDirID]; ok {
				continue
			}
			seen[f.BuildDirID] = struct{}{}
			_ = opts.Store.FinishBuildDirIfDone(f.BuildDirID)
		}
	}
	return total, nil
}

func ingestProjectPending(opts Options, project string, base *BackfillResult) (BackfillResult, error) {
	var result BackfillResult
	if base != nil {
		result = *base
	}
	result.DryRun = opts.DryRun

	files, err := listPendingForGroupProject(opts.Store, project)
	if err != nil {
		return result, err
	}
	compressed, dedupRef, skipped, errs, bytesBefore, bytesAfter := processFiles(opts, files)
	result.GroupsProcessed = len(files)
	result.FilesCompressed = compressed
	result.FilesDedupRef = dedupRef
	result.FilesSkipped = skipped
	result.Errors = errs
	result.BytesBefore = bytesBefore
	result.BytesAfter = bytesAfter

	if !opts.DryRun {
		seen := make(map[int64]struct{})
		for _, f := range files {
			if _, ok := seen[f.BuildDirID]; ok {
				continue
			}
			seen[f.BuildDirID] = struct{}{}
			_ = opts.Store.FinishBuildDirIfDone(f.BuildDirID)
		}
	}
	return result, nil
}

// listPendingForGroupProject собирает pending-файлы всех DB-проектов с тем же ключом группы.
func listPendingForGroupProject(store *storage.Storage, project string) ([]storage.DedupFile, error) {
	groupKey := NormalizeDedupGroupProject(project)
	all, err := store.ListAllPendingDedupFiles()
	if err != nil {
		return nil, err
	}
	if groupKey == "" {
		return all, nil
	}
	out := make([]storage.DedupFile, 0, len(all))
	for _, f := range all {
		if NormalizeDedupGroupProject(f.ProjectName) == groupKey {
			out = append(out, f)
		}
	}
	return out, nil
}

func filterFilesByProjects(files []storage.DedupFile, projects []string) []storage.DedupFile {
	allowed := projectFilterSet(projects)
	if allowed == nil {
		return files
	}
	out := make([]storage.DedupFile, 0, len(files))
	for _, f := range files {
		if matchesProjectFilter(f.ProjectName, allowed) {
			out = append(out, f)
		}
	}
	return out
}

func filterProjects(all []string, single string) []string {
	if single != "" {
		return []string{single}
	}
	return all
}

func processFiles(opts Options, files []storage.DedupFile) (compressed, dedupRef, skipped, errors int, bytesBefore, bytesAfter int64) {
	if opts.BlobStore == nil {
		slog.Error("dedup blob store not configured")
		return 0, 0, 0, len(files), 0, 0
	}

	// In-memory map для CAS в рамках одного прогона.
	batchBlobs := make(map[string]string)

	for _, f := range files {
		if opts.DryRun {
			compressed++
			bytesBefore += f.OriginalSize
			continue
		}

		if _, err := os.Stat(f.FilePath); err != nil {
			_ = opts.Store.MarkDedupFileError(f.ID, fmt.Sprintf("file missing: %v", err))
			errors++
			continue
		}

		sha, err := FileSHA256(f.FilePath)
		if err != nil {
			_ = opts.Store.MarkDedupFileError(f.ID, err.Error())
			errors++
			continue
		}

		blobPath, isRef, err := resolveBlob(opts, sha, batchBlobs)
		if err != nil {
			_ = opts.Store.MarkDedupFileError(f.ID, err.Error())
			errors++
			continue
		}

		if isRef {
			if err := os.Remove(f.FilePath); err != nil {
				_ = opts.Store.MarkDedupFileError(f.ID, fmt.Sprintf("remove original: %v", err))
				errors++
				continue
			}
			compSize, _ := fileSizeOnDisk(blobPath)
			if err := opts.Store.MarkDedupFileDone(f.ID, storage.DedupKindRef, 0, blobPath, sha, compSize); err != nil {
				_ = opts.Store.MarkDedupFileError(f.ID, err.Error())
				errors++
				continue
			}
			dedupRef++
			bytesBefore += f.OriginalSize
			continue
		}

		compSize, err := CompressFileTo(f.FilePath, blobPath)
		if err != nil {
			os.Remove(blobPath)
			_ = opts.Store.MarkDedupFileError(f.ID, err.Error())
			errors++
			continue
		}

		if err := VerifyDecompress(blobPath, sha); err != nil {
			os.Remove(blobPath)
			_ = opts.Store.MarkDedupFileError(f.ID, err.Error())
			errors++
			continue
		}

		if err := os.Remove(f.FilePath); err != nil {
			os.Remove(blobPath)
			_ = opts.Store.MarkDedupFileError(f.ID, fmt.Sprintf("remove original: %v", err))
			errors++
			continue
		}

		if err := opts.Store.MarkDedupFileDone(f.ID, storage.DedupKindCompressed, 0, blobPath, sha, compSize); err != nil {
			_ = opts.Store.MarkDedupFileError(f.ID, err.Error())
			errors++
			continue
		}

		batchBlobs[sha] = blobPath
		compressed++
		bytesBefore += f.OriginalSize
		bytesAfter += compSize
	}

	return compressed, dedupRef, skipped, errors, bytesBefore, bytesAfter
}

// resolveBlob ищет существующий blob по SHA256 (БД → текущий прогон → новый путь).
func resolveBlob(opts Options, sha string, batchBlobs map[string]string) (blobPath string, isRef bool, err error) {
	if p, ok := batchBlobs[sha]; ok {
		return p, true, nil
	}
	existing, err := opts.Store.FindBlobPathBySHA(sha)
	if err != nil {
		return "", false, err
	}
	if existing != "" {
		if _, statErr := os.Stat(existing); statErr == nil {
			return existing, true, nil
		}
	}
	return opts.BlobStore.PathForSHA(sha), false, nil
}

func fileSizeOnDisk(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}
