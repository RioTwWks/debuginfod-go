package dedup

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/debugfilename"
	"github.com/your-username/debuginfod-go/pkg/elfcomment"
)

// Discover рекурсивно ищет каталоги build_* под scan roots и регистрирует .debug в БД.
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

func registerDebugFiles(store *storage.Storage, buildDirID int64, dirPath string) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".debug") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(name), ".xdelta") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(name), ".zst") {
			continue
		}
		fullPath := filepath.Join(dirPath, name)
		info, err := ent.Info()
		if err != nil {
			continue
		}
		meta, err := debugfilename.MetadataFromName(name)
		if err != nil {
			slog.Debug("dedup skip file", "path", fullPath, "err", err)
			continue
		}
		tag, err := elfcomment.FromPath(fullPath)
		if err != nil {
			slog.Debug("dedup no commit tag", "path", fullPath, "err", err)
			tag = ""
		}
		_, err = store.UpsertDedupFile(storage.DedupFile{
			BuildDirID:   buildDirID,
			FilePath:     fullPath,
			Filename:     meta.Filename,
			FileStem:     meta.Stem,
			Version:      meta.Version,
			FileBuildNum: meta.BuildNum,
			CommitTag:    tag,
			OriginalSize: info.Size(),
		})
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
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
