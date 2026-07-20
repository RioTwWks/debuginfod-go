package dedup

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/your-username/debuginfod-go/internal/storage"
)

// Options — параметры dedup pipeline (xdelta3 + preprocess).
type Options struct {
	Store        *storage.Storage
	ScanPaths    []string
	Xdelta       *Xdelta
	Preprocessor Preprocessor
	ObjcopyZstd  *ObjcopyZstd
	CompressBase bool
	Projects     []string
	Workers      int
	DryRun       bool
}

// BackfillResult — итог backfill/ingest.
type BackfillResult struct {
	BuildDirsProcessed int   `json:"build_dirs_processed"`
	FilesRegistered    int   `json:"files_registered"`
	GroupsProcessed    int   `json:"groups_processed"`
	FilesCompressed    int   `json:"files_compressed"`
	FilesDedupRef      int   `json:"files_dedup_ref"`
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

	groups := GroupFiles(files)
	result.GroupsProcessed = len(groups)
	compressed, skipped, errs, bytesBefore, bytesAfter := processGroups(opts, groups)
	result.FilesCompressed = compressed
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

	groups := GroupFiles(files)
	total.GroupsProcessed = len(groups)
	compressed, skipped, errs, bytesBefore, bytesAfter := processGroups(opts, groups)
	total.FilesCompressed = compressed
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
	groups := GroupFiles(files)
	result.GroupsProcessed = len(groups)
	compressed, skipped, errs, bytesBefore, bytesAfter := processGroups(opts, groups)
	result.FilesCompressed = compressed
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

func processGroups(opts Options, groups map[string][]storage.DedupFile) (compressed, skipped, errors int, bytesBefore, bytesAfter int64) {
	opts = normalizeOptions(opts)

	if !opts.DryRun {
		if !opts.Xdelta.Available() {
			slog.Error("xdelta3 not found", "bin", opts.Xdelta.Bin)
			return 0, 0, len(groups), 0, 0
		}
		if opts.Preprocessor != nil && opts.Preprocessor.Name() != "none" && !opts.Preprocessor.Available() {
			slog.Error("dedup preprocessor not available", "name", opts.Preprocessor.Name())
			return 0, 0, len(groups), 0, 0
		}
		if opts.CompressBase && opts.ObjcopyZstd != nil && !opts.ObjcopyZstd.Available() {
			slog.Error("objcopy not found for compress base")
			return 0, 0, len(groups), 0, 0
		}
	}

	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			if group[i].FileBuildNum != group[j].FileBuildNum {
				return group[i].FileBuildNum < group[j].FileBuildNum
			}
			return group[i].FilePath < group[j].FilePath
		})

		base := group[0]
		if len(group) == 1 {
			if !opts.DryRun {
				if err := markSingletonFull(opts, base); err != nil {
					_ = opts.Store.MarkDedupFileError(base.ID, err.Error())
					errors++
					continue
				}
			}
			skipped++
			continue
		}

		if opts.DryRun {
			compressed += len(group) - 1
			skipped++
			for _, f := range group {
				bytesBefore += f.OriginalSize
			}
			continue
		}

		c, bBefore, bAfter, err := processGroup(opts, group)
		compressed += c
		bytesBefore += bBefore
		bytesAfter += bAfter
		if err != nil {
			errors++
		}
		skipped++
	}
	return compressed, skipped, errors, bytesBefore, bytesAfter
}

func normalizeOptions(opts Options) Options {
	if opts.Xdelta == nil {
		opts.Xdelta = NewXdelta("")
	}
	if opts.Preprocessor == nil {
		opts.Preprocessor = NewDecompressDwzPreprocessor(ToolPaths{})
	}
	if opts.ObjcopyZstd == nil {
		opts.ObjcopyZstd = NewObjcopyZstd("")
	}
	return opts
}

func markSingletonFull(opts Options, f storage.DedupFile) error {
	if _, err := os.Stat(f.FilePath); err != nil {
		return fmt.Errorf("file missing: %w", err)
	}
	sha, err := FileSHA256(f.FilePath)
	if err != nil {
		return err
	}
	return opts.Store.MarkDedupFileDone(f.ID, storage.DedupKindFull, 0, "", sha, 0)
}

func processGroup(opts Options, group []storage.DedupFile) (compressed int, bytesBefore, bytesAfter int64, groupErr error) {
	for _, f := range group {
		bytesBefore += f.OriginalSize
	}

	base := group[0]
	if _, err := os.Stat(base.FilePath); err != nil {
		_ = opts.Store.MarkDedupFileError(base.ID, fmt.Sprintf("base missing: %v", err))
		return 0, 0, 0, err
	}

	if err := opts.Preprocessor.ApplyInPlace(base.FilePath); err != nil {
		_ = opts.Store.MarkDedupFileError(base.ID, err.Error())
		return 0, 0, 0, err
	}

	baseSHA, err := FileSHA256(base.FilePath)
	if err != nil {
		_ = opts.Store.MarkDedupFileError(base.ID, err.Error())
		return 0, 0, 0, err
	}

	baseSize, _ := fileSizeOnDisk(base.FilePath)
	if err := opts.Store.MarkDedupFileDone(base.ID, storage.DedupKindBase, 0, "", baseSHA, baseSize); err != nil {
		_ = opts.Store.MarkDedupFileError(base.ID, err.Error())
		return 0, 0, 0, err
	}
	bytesAfter += baseSize

	for _, target := range group[1:] {
		after, err := compressOne(opts, base, target)
		if err != nil {
			slog.Warn("dedup compress", "target", target.FilePath, "err", err)
			_ = opts.Store.MarkDedupFileError(target.ID, err.Error())
			groupErr = err
			continue
		}
		compressed++
		bytesAfter += after
	}

	if opts.CompressBase && opts.ObjcopyZstd != nil && opts.ObjcopyZstd.Available() {
		compSize, err := opts.ObjcopyZstd.CompressInPlace(base.FilePath)
		if err != nil {
			slog.Warn("dedup compress base", "base", base.FilePath, "err", err)
			_ = opts.Store.MarkDedupFileError(base.ID, "compress base: "+err.Error())
			groupErr = err
		} else {
			_ = opts.Store.UpdateDedupFileCompressedSize(base.ID, compSize)
			bytesAfter = bytesAfter - baseSize + compSize
		}
	}

	return compressed, bytesBefore, bytesAfter, groupErr
}

func compressOne(opts Options, base, target storage.DedupFile) (deltaSize int64, err error) {
	if _, err := os.Stat(target.FilePath); err != nil {
		return 0, fmt.Errorf("target missing: %w", err)
	}

	workDir, err := os.MkdirTemp(filepath.Dir(target.FilePath), "dedup-prep-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(workDir)

	prepTarget := filepath.Join(workDir, filepath.Base(target.FilePath))
	if err := copyFileAtomic(target.FilePath, prepTarget); err != nil {
		return 0, err
	}
	if err := opts.Preprocessor.ApplyInPlace(prepTarget); err != nil {
		return 0, err
	}

	origSHA, err := FileSHA256(prepTarget)
	if err != nil {
		return 0, err
	}

	deltaPath := DeltaPathFor(target.FilePath)
	if err := opts.Xdelta.Encode(base.FilePath, prepTarget, deltaPath); err != nil {
		return 0, err
	}

	tmpPath := filepath.Join(workDir, "restore-verify.debug")
	if err := opts.Xdelta.Decode(base.FilePath, deltaPath, tmpPath); err != nil {
		os.Remove(deltaPath)
		return 0, fmt.Errorf("verify decode: %w", err)
	}
	restoredSHA, err := FileSHA256(tmpPath)
	if err != nil {
		os.Remove(deltaPath)
		return 0, err
	}
	if restoredSHA != origSHA {
		os.Remove(deltaPath)
		return 0, fmt.Errorf("sha256 mismatch after restore")
	}

	if err := os.Remove(target.FilePath); err != nil {
		return 0, fmt.Errorf("remove original: %w", err)
	}

	deltaSize, err = fileSizeOnDisk(deltaPath)
	if err != nil {
		return 0, err
	}

	if err := opts.Store.MarkDedupFileDone(
		target.ID, storage.DedupKindDelta, base.ID, deltaPath, origSHA, deltaSize,
	); err != nil {
		return 0, err
	}
	return deltaSize, nil
}

func fileSizeOnDisk(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}
