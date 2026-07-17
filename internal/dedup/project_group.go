package dedup

import (
	"path/filepath"
	"regexp"
	"strings"
)

// versionPathSegment — числовой сегмент пути вроде "16", "16.0.0", "1.6.2".
var versionPathSegment = regexp.MustCompile(`^\d+(\.\d+)*$`)

// NormalizeDedupGroupProject сужает путь проекта для отчётов:
// хвостовые версионные каталоги (16/16.0.0, 1.6.3) схлопываются до общего продукта.
// Имя проекта в БД/UI не меняется — только ключ группы.
func NormalizeDedupGroupProject(project string) string {
	project = filepath.ToSlash(strings.TrimSpace(project))
	if project == "" || project == "." {
		return project
	}
	parts := strings.Split(project, "/")
	for len(parts) > 1 && versionPathSegment.MatchString(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, "/")
}
