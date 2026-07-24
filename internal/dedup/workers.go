package dedup

import (
	"log/slog"
	"sort"
	"sync"

	"github.com/your-username/debuginfod-go/internal/storage"
)

type groupJob struct {
	files []storage.DedupFile
}

type groupTotals struct {
	compressed int
	skipped    int
	errors     int
	bytesBefore int64
	bytesAfter  int64
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

	jobs := make([]groupJob, 0, len(groups))
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		sorted := append([]storage.DedupFile(nil), group...)
		sortGroupFiles(sorted)
		jobs = append(jobs, groupJob{files: sorted})
	}

	pool := newCompressPool(fileWorkersFor(opts))

	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}
	if opts.DryRun || workers == 1 || len(jobs) <= 1 {
		var total groupTotals
		for _, job := range jobs {
			total.add(runGroupJob(opts, job, pool))
		}
		return total.compressed, total.skipped, total.errors, total.bytesBefore, total.bytesAfter
	}

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var total groupTotals

	for _, job := range jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := runGroupJob(opts, job, pool)
			mu.Lock()
			total.add(result)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return total.compressed, total.skipped, total.errors, total.bytesBefore, total.bytesAfter
}

func sortGroupFiles(group []storage.DedupFile) {
	sort.Slice(group, func(i, j int) bool {
		if group[i].FileBuildNum != group[j].FileBuildNum {
			return group[i].FileBuildNum < group[j].FileBuildNum
		}
		return group[i].FilePath < group[j].FilePath
	})
}

func runGroupJob(opts Options, job groupJob, pool *compressPool) groupTotals {
	var total groupTotals
	group := job.files
	if len(group) == 0 {
		return total
	}

	base := group[0]
	if len(group) == 1 {
		if !opts.DryRun {
			existing, baseErr := findGroupBase(opts.Store, base)
			if baseErr == nil {
				after, compressErr := compressOne(opts, existing, base)
				if compressErr != nil {
					_ = opts.Store.MarkDedupFileError(base.ID, compressErr.Error())
					total.errors++
					return total
				}
				total.compressed++
				total.bytesBefore += base.OriginalSize
				total.bytesAfter += after
				total.skipped++
				return total
			}
			if !isNotFound(baseErr) {
				_ = opts.Store.MarkDedupFileError(base.ID, baseErr.Error())
				total.errors++
				return total
			}
			if err := markSingletonFull(opts, base); err != nil {
				_ = opts.Store.MarkDedupFileError(base.ID, err.Error())
				total.errors++
				return total
			}
		} else if existing, baseErr := findGroupBase(opts.Store, base); baseErr == nil {
			_ = existing
			total.compressed++
		}
		total.skipped++
		return total
	}

	if opts.DryRun {
		total.compressed += len(group) - 1
		total.skipped++
		for _, f := range group {
			total.bytesBefore += f.OriginalSize
		}
		return total
	}

	c, bBefore, bAfter, err := processGroup(opts, group, pool)
	total.compressed += c
	total.bytesBefore += bBefore
	total.bytesAfter += bAfter
	if err != nil {
		total.errors++
	}
	total.skipped++
	return total
}

func (t *groupTotals) add(o groupTotals) {
	t.compressed += o.compressed
	t.skipped += o.skipped
	t.errors += o.errors
	t.bytesBefore += o.bytesBefore
	t.bytesAfter += o.bytesAfter
}
