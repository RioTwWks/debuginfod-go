package benchdedup

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// GroupMode — режим группировки .debug для бенчмарка.
type GroupMode string

const (
	// GroupModeStem — как production dedup: project + file_stem (рекомендуется для Quik).
	GroupModeStem GroupMode = "stem"
	// GroupModeStemVersion — project + file_stem + version из имени файла.
	GroupModeStemVersion GroupMode = "stem-version"
	// GroupModeStrategyA — project + file_stem + version + commit_tag (строгий; часто даёт singleton-группы).
	GroupModeStrategyA GroupMode = "strategy-a"
)

// ParseGroupMode разбирает флаг --group-by.
func ParseGroupMode(s string) (GroupMode, error) {
	switch GroupMode(strings.TrimSpace(strings.ToLower(s))) {
	case "", "stem":
		return GroupModeStem, nil
	case "stem-version", "stem_version":
		return GroupModeStemVersion, nil
	case "strategy-a", "strategy_a", "strategya":
		return GroupModeStrategyA, nil
	default:
		return "", fmt.Errorf("unknown group-by %q (use stem, stem-version, strategy-a)", s)
	}
}

// GroupKey — ключ группировки файлов для диффа.
type GroupKey struct {
	Project   string
	FileStem  string
	Version   string
	CommitTag string
	Mode      GroupMode
}

func (k GroupKey) String() string {
	switch k.Mode {
	case GroupModeStemVersion:
		return fmt.Sprintf("%s|%s|%s", k.Project, k.FileStem, k.Version)
	case GroupModeStrategyA:
		return fmt.Sprintf("%s|%s|%s|%s", k.Project, k.FileStem, k.Version, k.CommitTag)
	default:
		return fmt.Sprintf("%s|%s", k.Project, k.FileStem)
	}
}

// FileGroup — набор .debug одной библиотеки между build_*.
type FileGroup struct {
	Key   GroupKey
	Files []DebugFile
}

// GroupStats — диагностика группировки.
type GroupStats struct {
	TotalFiles     int
	TotalGroups    int
	GroupsGE2      int
	Singletons     int
	LargestGroup   int
	Mode           GroupMode
}

// GroupFiles группирует файлы по выбранному режиму.
func GroupFiles(files []DebugFile, mode GroupMode) []FileGroup {
	if mode == "" {
		mode = GroupModeStem
	}
	m := make(map[string]*FileGroup)
	for _, f := range files {
		key := groupKeyForFile(f, mode)
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

func groupKeyForFile(f DebugFile, mode GroupMode) GroupKey {
	key := GroupKey{
		Project:   normalizeProject(f.Project),
		FileStem:  f.FileStem,
		Version:   f.Version,
		CommitTag: f.CommitTag,
		Mode:      mode,
	}
	return key
}

// ComputeGroupStats считает статистику до фильтра min-files.
func ComputeGroupStats(files []DebugFile, mode GroupMode) GroupStats {
	groups := GroupFiles(files, mode)
	stats := GroupStats{
		TotalFiles:  len(files),
		TotalGroups: len(groups),
		Mode:        mode,
	}
	for _, g := range groups {
		n := len(g.Files)
		if n >= 2 {
			stats.GroupsGE2++
		} else {
			stats.Singletons++
		}
		if n > stats.LargestGroup {
			stats.LargestGroup = n
		}
	}
	return stats
}

func normalizeProject(project string) string {
	project = filepathSlash(strings.TrimSpace(project))
	if project == "" || project == "." {
		return project
	}
	parts := strings.Split(project, "/")
	for len(parts) > 1 && versionPathSegment.MatchString(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, "/")
}

var versionPathSegment = regexp.MustCompile(`^\d+(\.\d+)*$`)

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
