package benchdedup

import (
	"fmt"
	"sort"
	"strings"
)

// GroupKey — ключ Strategy A: file_stem + version + commit-id в рамках проекта.
type GroupKey struct {
	Project   string
	FileStem  string
	Version   string
	CommitTag string
}

func (k GroupKey) String() string {
	return fmt.Sprintf("%s|%s|%s|%s", k.Project, k.FileStem, k.Version, k.CommitTag)
}

// FileGroup — набор .debug одной библиотеки между build_*.
type FileGroup struct {
	Key   GroupKey
	Files []DebugFile
}

// GroupByStrategyA группирует файлы по (project, file_stem, version, commit_tag).
func GroupByStrategyA(files []DebugFile) []FileGroup {
	m := make(map[string]*FileGroup)
	for _, f := range files {
		key := GroupKey{
			Project:   filepathSlash(f.Project),
			FileStem:  f.FileStem,
			Version:   f.Version,
			CommitTag: f.CommitTag,
		}
		id := key.String()
		g, ok := m[id]
		if !ok {
			g = &FileGroup{Key: key}
			m[id] = g
		}
		g.Files = append(g.Files, f)
	}
	out := make([]FileGroup, 0, len(m))
	for _, g := range m {
		sort.Slice(g.Files, func(i, j int) bool {
			if g.Files[i].FileBuildNum != g.Files[j].FileBuildNum {
				return g.Files[i].FileBuildNum < g.Files[j].FileBuildNum
			}
			if g.Files[i].BuildDirNum != g.Files[j].BuildDirNum {
				return g.Files[i].BuildDirNum < g.Files[j].BuildDirNum
			}
			return g.Files[i].Path < g.Files[j].Path
		})
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key.String() < out[j].Key.String()
	})
	return out
}

func filepathSlash(s string) string {
	return strings.ReplaceAll(s, "\\", "/")
}

// FilterGroups оставляет группы с минимум minFiles элементов.
func FilterGroups(groups []FileGroup, minFiles int) []FileGroup {
	if minFiles <= 1 {
		minFiles = 2
	}
	var out []FileGroup
	for _, g := range groups {
		if len(g.Files) >= minFiles {
			out = append(out, g)
		}
	}
	return out
}
