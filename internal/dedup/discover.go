package dedup

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/debugfilename"
	"github.com/your-username/debuginfod-go/pkg/elfcomment"
)

// Discover регистрирует build_* каталоги и .debug файлы в БД.
func Discover(store *storage.Storage, scanRoots []string, projects []string) (int, error) {
	projectSet := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		projectSet[p] = struct{}{}
	}
	registered := 0
	for _, root := range scanRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			slog.Warn("dedup discover read root", "root", root, "err", err)
			continue
		}
		for _, ent := range entries {
			if !ent.IsDir() {
				continue
			}
			projectName := ent.Name()
			if len(projectSet) > 0 {
				if _, ok := projectSet[projectName]; !ok {
					continue
				}
			}
			projectID, err := store.EnsureDedupProject(projectName)
			if err != nil {
				return registered, err
			}
			projectPath := filepath.Join(root, projectName)
			buildDirs, err := os.ReadDir(projectPath)
			if err != nil {
				slog.Warn("dedup discover project", "project", projectName, "err", err)
				continue
			}
			for _, bd := range buildDirs {
				if !bd.IsDir() || !strings.HasPrefix(bd.Name(), "build_") {
					continue
				}
				dirPath := filepath.Join(projectPath, bd.Name())
				dirNum, err := debugfilename.ParseBuildDir(bd.Name())
				if err != nil {
					slog.Debug("dedup skip dir", "path", dirPath, "err", err)
					continue
				}
				buildDirID, err := store.UpsertDedupBuildDir(projectID, dirPath, dirNum)
				if err != nil {
					return registered, err
				}
				n, err := registerDebugFiles(store, buildDirID, dirPath)
				registered += n
				if err != nil {
					return registered, err
				}
			}
		}
	}
	return registered, nil
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
		fullPath := filepath.Join(dirPath, name)
		info, err := ent.Info()
		if err != nil {
			continue
		}
		parsed, err := debugfilename.Parse(name)
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
			Filename:     parsed.Filename,
			FileStem:     parsed.Stem,
			Version:      parsed.Version,
			FileBuildNum: parsed.BuildNum,
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

// GroupKey возвращает ключ группировки для файла.
func GroupKey(f storage.DedupFile) storage.DedupGroupKey {
	return storage.DedupGroupKey{
		Project:   f.ProjectName,
		FileStem:  f.FileStem,
		Version:   f.Version,
		CommitTag: f.CommitTag,
	}
}

func groupKeyString(k storage.DedupGroupKey) string {
	return fmt.Sprintf("%s|%s|%s|%s", k.Project, k.FileStem, k.Version, k.CommitTag)
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
