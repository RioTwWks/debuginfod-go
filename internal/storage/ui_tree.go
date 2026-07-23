package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"sort"
	"strings"

	"github.com/your-username/debuginfod-go/internal/fnmatch"
)

const uiNoCommitLabel = "(no commit)"

// UITreeFile — лист дерева (.debug файл).
type UITreeFile struct {
	Filename     string         `json:"filename"`
	RelativePath string         `json:"relative_path"`
	Project      string         `json:"project,omitempty"`
	BuildID      string         `json:"buildid,omitempty"`
	DedupID      int64          `json:"dedup_id,omitempty"`
	Source       string         `json:"source,omitempty"`
	Type         string         `json:"type"`
	GitCommit    string         `json:"git_commit,omitempty"`
	Comment      *UICommentInfo `json:"comment,omitempty"`
}

// UITreeNode — узел дерева (группа по git commit).
type UITreeNode struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	Files    []UITreeFile `json:"files,omitempty"`
	Children []UITreeNode `json:"children,omitempty"`
}

// UIProjectFromRelativePath возвращает проект: Released/X или Unsorted/X.
func UIProjectFromRelativePath(rel string) string {
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	parts := strings.Split(rel, "/")
	if len(parts) >= 2 && (parts[0] == "Released" || parts[0] == "Unsorted") {
		return parts[0] + "/" + parts[1]
	}
	if len(parts) > 0 {
		return parts[0]
	}
	return rel
}

// IsDebugUIFile — артефакт для дерева .debug.
func IsDebugUIFile(rec ArtifactRecord) bool {
	if rec.Type == "debuginfo" {
		return true
	}
	name := strings.ToLower(rec.Filename)
	if name == "" {
		name = strings.ToLower(filepath.Base(rec.File))
	}
	return strings.HasSuffix(name, ".debug")
}

// SearchDebugFilesForUI — единый поиск по пути, имени, commit, build-id.
// limit <= 0 — без ограничения (для дерева browse).
func (s *Storage) SearchDebugFilesForUI(ctx context.Context, scanRoots []string, query string, limit int) ([]ArtifactRecord, error) {
	if limit > 50000 {
		limit = 50000
	}

	query = strings.TrimSpace(query)
	var rows *sql.Rows
	var err error
	if isSimpleSearchQuery(query) {
		where, args := artifactSearchSQLFilter(query)
		rows, err = s.db.QueryContext(ctx, rebind(`
			SELECT `+artifactSelectColumns+`
			FROM artifacts
			`+where+`
			ORDER BY file_path
		`, s.dialect), args...)
	} else {
		rows, err = s.db.QueryContext(ctx, rebind(`
			SELECT `+artifactSelectColumns+`
			FROM artifacts
			ORDER BY file_path
		`, s.dialect))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ArtifactRecord
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		rec, err := scanArtifactRow(rows)
		if err != nil {
			return nil, err
		}
		if !IsDebugUIFile(rec) {
			continue
		}
		EnrichArtifactRecord(&rec, scanRoots)
		if query != "" && !matchesUnifiedQuery(query, rec) {
			continue
		}
		out = append(out, rec)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	if out == nil {
		out = []ArtifactRecord{}
	}
	return out, rows.Err()
}

func isSimpleSearchQuery(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}
	return !strings.ContainsAny(query, "*?[")
}

func artifactSearchSQLFilter(query string) (string, []any) {
	pattern := "%" + strings.ToLower(query) + "%"
	return `
		WHERE (type = 'debuginfo' OR LOWER(file_path) LIKE '%.debug')
		AND (
			LOWER(file_path) LIKE ? OR
			LOWER(git_commit) LIKE ? OR
			LOWER(build_id) LIKE ? OR
			LOWER(raw_build_id) LIKE ?
		)
	`, []any{pattern, pattern, pattern, pattern}
}

func matchesUnifiedQuery(query string, rec ArtifactRecord) bool {
	q := strings.TrimSpace(query)
	if q == "" {
		return true
	}
	if strings.ContainsAny(q, "*?[") {
		return fnmatch.Match(q, rec.RelativePath, fnmatch.Pathname) ||
			fnmatch.Match(q, rec.RelativePath, 0) ||
			fnmatch.Match(q, rec.Filename, 0) ||
			fnmatch.Match(q, rec.GitCommit, 0) ||
			fnmatch.Match(q, rec.BuildID, 0) ||
			fnmatch.Match(q, rec.RawBuildID, 0)
	}
	lower := strings.ToLower(q)
	return strings.Contains(strings.ToLower(rec.RelativePath), lower) ||
		strings.Contains(strings.ToLower(rec.Filename), lower) ||
		strings.Contains(strings.ToLower(rec.GitCommit), lower) ||
		strings.Contains(strings.ToLower(rec.BuildID), lower) ||
		strings.Contains(strings.ToLower(rec.RawBuildID), lower)
}

// UICommitKey возвращает ключ группировки дерева по git commit.
func UICommitKey(file UITreeFile) string {
	if c := strings.TrimSpace(file.GitCommit); c != "" {
		return c
	}
	if file.Comment != nil {
		if c := strings.TrimSpace(file.Comment.GitCommit); c != "" {
			return c
		}
	}
	return uiNoCommitLabel
}

// UICommitLabel — короткая подпись узла commit в дереве.
func UICommitLabel(commit string) string {
	if commit == uiNoCommitLabel {
		return commit
	}
	if len(commit) > 16 {
		return commit[:12] + "…"
	}
	return commit
}

// BuildUITree группирует файлы по git commit.
func BuildUITree(scanRoots []string, records []ArtifactRecord) []UITreeNode {
	files := make([]UITreeFile, 0, len(records))
	for _, rec := range records {
		files = append(files, ArtifactRecordToUITreeFile(rec, scanRoots, true))
	}
	return BuildUITreeFromFiles(files)
}

// BuildUITreeFromFiles строит дерево: commit → *.debug.
func BuildUITreeFromFiles(files []UITreeFile) []UITreeNode {
	commits := make(map[string][]UITreeFile)

	for _, file := range files {
		key := UICommitKey(file)
		commits[key] = append(commits[key], file)
	}

	names := make([]string, 0, len(commits))
	for name := range commits {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == uiNoCommitLabel {
			return false
		}
		if names[j] == uiNoCommitLabel {
			return true
		}
		return names[i] < names[j]
	})

	out := make([]UITreeNode, 0, len(names))
	for _, commit := range names {
		out = append(out, UITreeNode{
			Name:  UICommitLabel(commit),
			Path:  commit,
			Files: sortUITreeFilesByPath(commits[commit]),
		})
	}
	return out
}

func sortUITreeFilesByPath(files []UITreeFile) []UITreeFile {
	if len(files) == 0 {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].RelativePath == files[j].RelativePath {
			return files[i].Filename < files[j].Filename
		}
		return files[i].RelativePath < files[j].RelativePath
	})
	out := make([]UITreeFile, len(files))
	copy(out, files)
	return out
}
