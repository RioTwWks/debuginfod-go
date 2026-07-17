package dedup

import (
	"fmt"
	"log/slog"
	"os"
	"sort"

	"github.com/your-username/debuginfod-go/internal/storage"
)

// Options — параметры dedup pipeline.
type Options struct {
	Store     *storage.Storage
	ScanPaths []string
	Xdelta    *Xdelta
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

	compressed, skipped, errs, bytesBefore, bytesAfter := processGroups(opts, GroupFiles(files))
	result.GroupsProcessed = len(GroupFiles(files))
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

	n, err := Discover(opts.Store, opts.ScanPaths, filterProjects(opts.Projects, project))
	if err != nil {
		return result, err
	}
	result.FilesRegistered = n

	files, err := opts.Store.ListPendingDedupFilesByProject(project)
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

// RunIngestAll запускает ingest для всех проектов из конфига.
func RunIngestAll(opts Options) (BackfillResult, error) {
	var total BackfillResult
	total.DryRun = opts.DryRun
	for _, project := range opts.Projects {
		r, err := RunIngestForProject(opts, project)
		if err != nil {
			return total, err
		}
		total.FilesRegistered += r.FilesRegistered
		total.GroupsProcessed += r.GroupsProcessed
		total.FilesCompressed += r.FilesCompressed
		total.FilesSkipped += r.FilesSkipped
		total.Errors += r.Errors
		total.BuildDirsProcessed += r.BuildDirsProcessed
		total.BytesBefore += r.BytesBefore
		total.BytesAfter += r.BytesAfter
	}
	return total, nil
}

func filterProjects(all []string, single string) []string {
	if single != "" {
		return []string{single}
	}
	return all
}

func processGroups(opts Options, groups map[string][]storage.DedupFile) (compressed, skipped, errors int, bytesBefore, bytesAfter int64) {
	if opts.Xdelta == nil {
		opts.Xdelta = NewXdelta("xdelta3")
	}
	if !opts.DryRun && !opts.Xdelta.Available() {
		slog.Error("xdelta3 not found", "bin", opts.Xdelta.Bin)
		return 0, 0, len(groups), 0, 0
	}

	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			return group[i].FileBuildNum < group[j].FileBuildNum
		})
		base := group[0]
		if base.CommitTag == "" {
			skipped += len(group)
			if !opts.DryRun {
				for _, f := range group {
					_ = opts.Store.MarkDedupFileDone(f.ID, storage.DedupKindFull, 0, "", "")
				}
			}
			continue
		}
		if len(group) == 1 {
			if !opts.DryRun {
				sha, _ := FileSHA256(base.FilePath)
				_ = opts.Store.MarkDedupFileDone(base.ID, storage.DedupKindBase, 0, "", sha)
			}
			skipped++
			continue
		}

		if opts.DryRun {
			compressed += len(group) - 1
			skipped++
			continue
		}

		baseSHA, err := FileSHA256(base.FilePath)
		if err != nil {
			_ = opts.Store.MarkDedupFileError(base.ID, err.Error())
			errors++
			continue
		}
		_ = opts.Store.MarkDedupFileDone(base.ID, storage.DedupKindBase, 0, "", baseSHA)

		for _, target := range group[1:] {
			if err := compressOne(opts, base, target); err != nil {
				slog.Warn("dedup compress", "target", target.FilePath, "err", err)
				_ = opts.Store.MarkDedupFileError(target.ID, err.Error())
				errors++
				continue
			}
			compressed++
			bytesBefore += target.OriginalSize
			if st, err := fileSizeOnDisk(DeltaPathFor(target.FilePath)); err == nil {
				bytesAfter += st
			}
		}
	}
	return compressed, skipped, errors, bytesBefore, bytesAfter
}

func fileSizeOnDisk(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

func compressOne(opts Options, base, target storage.DedupFile) error {
	if _, err := os.Stat(base.FilePath); err != nil {
		return fmt.Errorf("base missing: %w", err)
	}
	if _, err := os.Stat(target.FilePath); err != nil {
		return fmt.Errorf("target missing: %w", err)
	}

	origSHA, err := FileSHA256(target.FilePath)
	if err != nil {
		return err
	}

	deltaPath := DeltaPathFor(target.FilePath)
	if err := opts.Xdelta.Encode(base.FilePath, target.FilePath, deltaPath); err != nil {
		return err
	}

	tmpPath := target.FilePath + ".restore-verify"
	defer os.Remove(tmpPath)

	if err := opts.Xdelta.Decode(base.FilePath, deltaPath, tmpPath); err != nil {
		os.Remove(deltaPath)
		return fmt.Errorf("verify decode: %w", err)
	}
	restoredSHA, err := FileSHA256(tmpPath)
	if err != nil {
		os.Remove(deltaPath)
		return err
	}
	if restoredSHA != origSHA {
		os.Remove(deltaPath)
		return fmt.Errorf("sha256 mismatch after restore")
	}

	if err := os.Remove(target.FilePath); err != nil {
		return fmt.Errorf("remove original: %w", err)
	}

	return opts.Store.MarkDedupFileDone(
		target.ID, storage.DedupKindDelta, base.ID, deltaPath, origSHA,
	)
}
