package dedup

import (
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/debugfilename"
	"github.com/your-username/debuginfod-go/pkg/elfcomment"
)

// Discover рекурсивно ищет каталоги build_* под scan roots и регистрирует .debug в БД
// (включая вложенные подпапки внутри build_*).
// Имя «проекта» — относительный путь от scan root до родителя build_* (например Released/QuikServer_16.0).
// projectFilter пустой — все найденные папки; иначе только точное совпадение пути проекта.
func Discover(store *storage.Storage, scanRoots []string, projectFilter []string) (int, error) {
	allowed := projectFilterSet(projectFilter)
	registered := 0

	for _, root := range scanRoots {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			slog.Warn("dedup discover abs root", "root", root, "err", err)
			continue
		}
		n, err := discoverUnderRoot(store, rootAbs, allowed)
		registered += n
		if err != nil {
			return registered, err
		}
	}
	return registered, nil
}

func discoverUnderRoot(store *storage.Storage, rootAbs string, allowed map[string]struct{}) (int, error) {
	registered := 0
	err := filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			slog.Debug("dedup walk", "path", path, "err", walkErr)
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "build_") {
			return nil
		}

		projectName := projectNameForBuildDir(rootAbs, path)
		if !matchesProjectFilter(projectName, allowed) {
			return filepath.SkipDir
		}

		dirNum, err := debugfilename.ParseBuildDir(name)
		if err != nil {
			slog.Debug("dedup skip dir", "path", path, "err", err)
			return filepath.SkipDir
		}

		projectID, err := store.EnsureDedupProject(projectName)
		if err != nil {
			return err
		}
		buildDirID, err := store.UpsertDedupBuildDir(projectID, path, dirNum)
		if err != nil {
			return err
		}
		n, err := registerDebugFiles(store, buildDirID, path)
		registered += n
		if err != nil {
			return err
		}
		return filepath.SkipDir
	})
	return registered, err
}

func projectNameForBuildDir(scanRoot, buildDirPath string) string {
	parent := filepath.Dir(buildDirPath)
	rel, err := filepath.Rel(scanRoot, parent)
	if err != nil || rel == "." {
		return filepath.Base(scanRoot)
	}
	return filepath.ToSlash(rel)
}

func projectFilterSet(projects []string) map[string]struct{} {
	if len(projects) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out[filepath.ToSlash(p)] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func matchesProjectFilter(projectName string, allowed map[string]struct{}) bool {
	if allowed == nil {
		return true
	}
	projectName = filepath.ToSlash(projectName)
	_, ok := allowed[projectName]
	return ok
}

type discoverCandidate struct {
	path string
	size int64
	meta debugfilename.Info
}

func registerDebugFiles(store *storage.Storage, buildDirID int64, dirPath string) (int, error) {
	candidates := make([]discoverCandidate, 0, 64)
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			slog.Debug("dedup walk build dir", "path", path, "err", walkErr)
			return nil
		}
		if d.IsDir() {
			if path != dirPath && strings.HasPrefix(d.Name(), "build_") {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".debug") {
			return nil
		}
		if strings.HasSuffix(lower, ".xdelta") || strings.HasSuffix(lower, ".zst") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		meta, err := debugfilename.MetadataFromName(name)
		if err != nil {
			slog.Debug("dedup skip file", "path", path, "err", err)
			return nil
		}
		candidates = append(candidates, discoverCandidate{
			path: path,
			size: info.Size(),
			meta: meta,
		})
		return nil
	})
	if err != nil {
		return 0, err
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	paths := make([]string, len(candidates))
	for i, c := range candidates {
		paths[i] = c.path
	}

	snapshots, err := store.DedupSnapshotsByPaths(paths)
	if err != nil {
		return 0, err
	}

	pending := make([]discoverCandidate, 0, len(candidates))
	for _, c := range candidates {
		if snap, ok := snapshots[c.path]; ok &&
			snap.Status == storage.DedupStatusDone &&
			snap.OriginalSize == c.size {
			continue
		}
		pending = append(pending, c)
	}
	if len(pending) == 0 {
		return 0, nil
	}

	pendingPaths := make([]string, len(pending))
	for i, c := range pending {
		pendingPaths[i] = c.path
	}
	commits, err := store.GitCommitsByFilePaths(pendingPaths)
	if err != nil {
		return 0, err
	}

	records := make([]storage.DedupFile, 0, len(pending))
	for _, c := range pending {
		tag := commits[c.path]
		if tag == "" {
			tag = elfcomment.FromPathOrEmpty(c.path)
			if tag == "" {
				slog.Debug("dedup no commit tag", "path", c.path, "err", elfcomment.ErrNotFound)
			}
		}
		records = append(records, storage.DedupFile{
			BuildDirID:   buildDirID,
			FilePath:     c.path,
			Filename:     c.meta.Filename,
			FileStem:     c.meta.Stem,
			Version:      c.meta.Version,
			FileBuildNum: c.meta.BuildNum,
			CommitTag:    tag,
			OriginalSize: c.size,
		})
	}

	return store.UpsertDedupFilesBatch(records)
}

// GroupKey возвращает ключ группировки (метаданные; на pipeline не влияет).
// Тег из .comment — git-метка (tag:/commit:/semver/SHA); JIRA не используется.
// на группировку не влияют: сжимаем одну библиотеку между build_* каталогами.
// ProjectName нормализуется (схлопываются хвостовые версии в пути).
func GroupKey(f storage.DedupFile) storage.DedupGroupKey {
	return storage.DedupGroupKey{
		Project:  NormalizeDedupGroupProject(f.ProjectName),
		FileStem: f.FileStem,
	}
}

func groupKeyString(k storage.DedupGroupKey) string {
	return fmt.Sprintf("%s|%s", k.Project, k.FileStem)
}

// GroupFiles группирует pending-файлы по ключу.
func GroupFiles(files []storage.DedupFile) map[string][]storage.DedupFile {
	groups := make(map[string][]storage.DedupFile)
	for _, f := range files {
		key := groupKeyString(GroupKey(f))
		groups[key] = append(groups[key], f)
	}
	return groups
}
