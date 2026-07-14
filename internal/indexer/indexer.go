package indexer

import (
	"bytes"
	"debug/dwarf"
	"debug/elf"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/your-username/debuginfod-go/internal/archive"
	"github.com/your-username/debuginfod-go/internal/cache"
	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/buildid"
)

type scanJob struct {
	path  string
	mtime int64
	size  int64
	kind  string // elf | archive | sourcepkg
}

// Indexer сканирует файловую систему и заполняет индекс.
type Indexer struct {
	storage       *storage.Storage
	paths         []string
	cacheDir      string
	cacheMaxBytes int64
	workers       int
	metrics       *metrics.Collector
	lazyExtract   bool
}

// Options — параметры индексатора.
type Options struct {
	Storage       *storage.Storage
	Paths         []string
	CacheDir      string
	CacheMaxBytes int64
	Workers       int
	Metrics       *metrics.Collector
	LazyExtract   bool
}

// NewIndexer создаёт индексатор.
func NewIndexer(opts Options) *Indexer {
	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}
	return &Indexer{
		storage:       opts.Storage,
		paths:         opts.Paths,
		cacheDir:      opts.CacheDir,
		cacheMaxBytes: opts.CacheMaxBytes,
		workers:       workers,
		metrics:       opts.Metrics,
		lazyExtract:   opts.LazyExtract,
	}
}

// Scan обходит пути и индексирует ELF/архивы с параллельными воркерами.
func (i *Indexer) Scan() error {
	start := time.Now()
	var indexed, skipped, errorsCount atomic.Int64

	jobs := make(chan scanJob, i.workers*2)
	var wg sync.WaitGroup

	for w := 0; w < i.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				var err error
				switch job.kind {
				case "archive":
					var count int
					count, err = i.indexArchive(job.path, job.mtime)
					if err == nil {
						indexed.Add(int64(count))
						_ = i.storage.MarkScanned(job.path, job.mtime, job.size, "archive")
					}
				case "sourcepkg":
					var count int
					count, err = i.indexSourcePackage(job.path, job.mtime)
					if err == nil {
						indexed.Add(int64(count))
						_ = i.storage.MarkScanned(job.path, job.mtime, job.size, "sourcepkg")
					}
				case "elf":
					err = i.indexELF(job.path, job.mtime, "", "")
					if err == nil {
						indexed.Add(1)
						_ = i.storage.MarkScanned(job.path, job.mtime, job.size, "elf")
					}
				}
				if err != nil {
					errorsCount.Add(1)
					slog.Warn("index failed", "path", job.path, "err", err)
				}
			}
		}()
	}

	for _, root := range i.paths {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				slog.Warn("walk skip", "path", path, "err", err)
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			mtime := info.ModTime().UnixNano()
			size := info.Size()

			var kind string
			switch {
			case archive.IsArchive(path):
				kind = "archive"
			case archive.IsSourcePackage(path):
				kind = "sourcepkg"
			case buildid.IsELF(path):
				kind = "elf"
			default:
				return nil
			}

			needs, err := i.storage.NeedsScan(path, mtime, size)
			if err != nil {
				slog.Warn("NeedsScan", "path", path, "err", err)
			} else if !needs {
				skipped.Add(1)
				return nil
			}

			jobs <- scanJob{path: path, mtime: mtime, size: size, kind: kind}
			return nil
		})
	}

	close(jobs)
	wg.Wait()

	if i.cacheMaxBytes > 0 {
		removed, freed, err := cache.Prune(i.cacheDir, i.cacheMaxBytes)
		if err != nil {
			slog.Warn("cache prune", "err", err)
		} else if removed > 0 {
			slog.Info("cache pruned", "removed", removed, "freed_bytes", freed)
		}
	}

	stats := metrics.ScanStats{
		Duration: time.Since(start),
		Indexed:  int(indexed.Load()),
		Skipped:  int(skipped.Load()),
		Errors:   int(errorsCount.Load()),
		Finished: time.Now(),
	}
	if i.metrics != nil {
		i.metrics.RecordScan(stats)
	}
	slog.Info("scan complete",
		"indexed", stats.Indexed,
		"skipped", stats.Skipped,
		"errors", stats.Errors,
		"duration", stats.Duration,
	)
	return nil
}

func (i *Indexer) indexArchive(archivePath string, mtimeNS int64) (int, error) {
	members, err := archive.ListELFMembers(archivePath)
	if err != nil {
		return 0, err
	}

	var count int
	for _, member := range members {
		var indexErr error
		if i.lazyExtract {
			indexErr = i.indexArchiveMemberLazy(member, mtimeNS)
		} else {
			cached, extractErr := archive.ExtractToCache(i.cacheDir, member.ArchivePath, member.MemberPath, member.Reader)
			if extractErr != nil {
				slog.Warn("extract failed", "archive", member.ArchivePath, "member", member.MemberPath, "err", extractErr)
				continue
			}
			indexErr = i.indexELF(cached, mtimeNS, member.ArchivePath, member.MemberPath)
		}
		if indexErr != nil {
			slog.Warn("index archive member", "member", member.MemberPath, "err", indexErr)
			continue
		}
		count++
	}
	return count, nil
}

func (i *Indexer) indexArchiveMemberLazy(member archive.Member, mtimeNS int64) error {
	rc, err := member.Reader()
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	result, err := buildid.FromBytes(data)
	if err != nil {
		return err
	}
	result.Value = buildid.Normalize(result.Value)

	artifactType, err := buildid.ArtifactTypeFromBytes(member.MemberPath, data)
	if err != nil {
		return err
	}

	row := storage.ArtifactInput{
		BuildID:     result.Value,
		Type:        artifactType,
		ArchivePath: member.ArchivePath,
		MemberPath:  member.MemberPath,
		BuildIDKind: string(result.Kind),
		RawBuildID:  result.Raw,
	}
	if err := i.storage.AddArtifact(row, mtimeNS); err != nil {
		return err
	}

	elfFile, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return i.indexSourcesFromDWARF(result.Value, elfFile, member.MemberPath, mtimeNS)
}

func (i *Indexer) indexSourcePackage(pkgPath string, mtimeNS int64) (int, error) {
	members, err := archive.ListSourceMembers(pkgPath)
	if err != nil {
		return 0, err
	}

	var count int
	for _, member := range members {
		in := storage.SourceInput{
			SourcePath:  member.MemberPath,
			ArchivePath: member.ArchivePath,
			MemberPath:  member.MemberPath,
		}
		if !i.lazyExtract {
			cached, extractErr := archive.ExtractToCache(i.cacheDir, member.ArchivePath, member.MemberPath, member.Reader)
			if extractErr != nil {
				slog.Warn("extract source failed", "pkg", pkgPath, "member", member.MemberPath, "err", extractErr)
				continue
			}
			in.FilePath = cached
		}
		if err := i.storage.AddSourceLocation(in, mtimeNS); err != nil {
			slog.Warn("index source member", "member", member.MemberPath, "err", err)
			continue
		}
		count++
	}
	return count, nil
}

func (i *Indexer) indexELF(path string, mtimeNS int64, archivePath, memberPath string) error {
	result, err := buildid.FromPathDetailed(path)
	if err != nil {
		return err
	}
	result.Value = buildid.Normalize(result.Value)

	elfFile, err := buildid.OpenELF(path)
	if err != nil {
		return err
	}
	defer elfFile.Close()

	artifactType := buildid.ArtifactType(path, elfFile)
	row := storage.ArtifactInput{
		BuildID:     result.Value,
		Type:        artifactType,
		FilePath:    path,
		ArchivePath: archivePath,
		MemberPath:  memberPath,
		BuildIDKind: string(result.Kind),
		RawBuildID:  result.Raw,
	}
	if err := i.storage.AddArtifact(row, mtimeNS); err != nil {
		return err
	}

	return i.indexSourcesFromDWARF(result.Value, elfFile, path, mtimeNS)
}

func (i *Indexer) indexSourcesFromDWARF(buildID string, f *elf.File, elfPath string, mtimeNS int64) error {
	data, err := f.DWARF()
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	reader := data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil {
			return err
		}
		if entry == nil {
			break
		}
		if entry.Tag != dwarf.TagCompileUnit {
			continue
		}

		compDir := attrString(entry, dwarf.AttrCompDir)
		name := attrString(entry, dwarf.AttrName)
		if name == "" {
			continue
		}

		sourcePath := resolveSourcePath(compDir, name)
		if sourcePath == "" {
			continue
		}
		if _, ok := seen[sourcePath]; ok {
			continue
		}
		seen[sourcePath] = struct{}{}

		filePath := findSourceOnDisk(sourcePath, elfPath)
		if filePath == "" {
			continue
		}

		if err := i.storage.AddSource(buildID, sourcePath, filePath, mtimeNS); err != nil {
			slog.Warn("save source", "path", sourcePath, "err", err)
		}
	}
	return nil
}

func attrString(entry *dwarf.Entry, attr dwarf.Attr) string {
	val, ok := entry.Val(attr).(string)
	if !ok {
		return ""
	}
	return val
}

func resolveSourcePath(compDir, name string) string {
	name = filepath.ToSlash(name)
	if filepath.IsAbs(name) {
		return name
	}
	if compDir == "" {
		return ""
	}
	compDir = filepath.ToSlash(compDir)
	if !strings.HasSuffix(compDir, "/") {
		compDir += "/"
	}
	return compDir + name
}

func findSourceOnDisk(sourcePath, elfPath string) string {
	candidates := []string{sourcePath}
	if !filepath.IsAbs(sourcePath) {
		candidates = append(candidates, filepath.Join(filepath.Dir(elfPath), sourcePath))
	}

	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			abs, err := filepath.Abs(candidate)
			if err == nil {
				return abs
			}
			return candidate
		}
	}
	return ""
}

// IndexELFReader индексирует ELF из потока (для тестов).
func (i *Indexer) IndexELFReader(r io.Reader, mtimeNS int64) error {
	tmp, err := os.CreateTemp(i.cacheDir, "elf-*")
	if err != nil {
		return err
	}
	path := tmp.Name()
	defer os.Remove(path)

	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return i.indexELF(path, mtimeNS, "", "")
}
