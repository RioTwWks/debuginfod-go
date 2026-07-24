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

// UITreeNode — узел дерева (группа по commit или проект/каталог).
type UITreeNode struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	Group    string       `json:"group,omitempty"` // "commit" или "project"
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

// UICommitLabel — подпись узла commit в дереве (полный id/tag, без сокращения).
func UICommitLabel(commit string) string {
	return commit
}

// BuildUITree группирует файлы: с commit — по commit, без — проект → каталоги.
func BuildUITree(scanRoots []string, records []ArtifactRecord) []UITreeNode {
	files := make([]UITreeFile, 0, len(records))
	for _, rec := range records {
		files = append(files, ArtifactRecordToUITreeFile(rec, scanRoots, true))
	}
	return BuildUITreeFromFiles(files)
}

// BuildUITreeFromFiles строит дерево с гибридной группировкой.
func BuildUITreeFromFiles(files []UITreeFile) []UITreeNode {
	var withCommit, withoutCommit []UITreeFile
	for _, file := range files {
		if hasUICommit(file) {
			withCommit = append(withCommit, file)
		} else {
			withoutCommit = append(withoutCommit, file)
		}
	}

	out := buildCommitGroups(withCommit)
	if len(withoutCommit) > 0 {
		out = append(out, buildProjectDirTree(withoutCommit)...)
	}
	return out
}

func hasUICommit(file UITreeFile) bool {
	return UICommitKey(file) != uiNoCommitLabel
}

func buildCommitGroups(files []UITreeFile) []UITreeNode {
	commits := make(map[string][]UITreeFile)
	for _, file := range files {
		key := UICommitKey(file)
		commits[key] = append(commits[key], file)
	}

	names := make([]string, 0, len(commits))
	for name := range commits {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]UITreeNode, 0, len(names))
	for _, commit := range names {
		out = append(out, UITreeNode{
			Name:  UICommitLabel(commit),
			Path:  commit,
			Group: "commit",
			Files: sortUITreeFilesByPath(commits[commit]),
		})
	}
	return out
}

func buildProjectDirTree(files []UITreeFile) []UITreeNode {
	type projMap = map[string]map[string][]UITreeFile
	projects := make(projMap)

	for _, file := range files {
		project := file.Project
		if project == "" {
			project = UIProjectFromRelativePath(file.RelativePath)
		}
		rest := strings.TrimPrefix(file.RelativePath, project)
		rest = strings.TrimPrefix(rest, "/")
		dir := filepath.ToSlash(filepath.Dir(rest))
		if dir == "." {
			dir = ""
		}

		if projects[project] == nil {
			projects[project] = make(map[string][]UITreeFile)
		}
		projects[project][dir] = append(projects[project][dir], file)
	}

	projectNames := make([]string, 0, len(projects))
	for name := range projects {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)

	out := make([]UITreeNode, 0, len(projectNames))
	for _, pname := range projectNames {
		root := UITreeNode{Name: pname, Path: pname, Group: "project"}
		root.Children, root.Files = buildDirChildren(pname, projects[pname])
		out = append(out, root)
	}
	return out
}

type treeNode struct {
	children map[string]*treeNode
	files    []UITreeFile
}

func buildDirChildren(project string, dirs map[string][]UITreeFile) ([]UITreeNode, []UITreeFile) {
	if len(dirs) == 0 {
		return nil, nil
	}

	root := &treeNode{children: make(map[string]*treeNode)}
	for dir, files := range dirs {
		parts := []string{}
		if dir != "" {
			parts = strings.Split(dir, "/")
		}
		cur := root
		for _, part := range parts {
			if cur.children[part] == nil {
				cur.children[part] = &treeNode{children: make(map[string]*treeNode)}
			}
			cur = cur.children[part]
		}
		cur.files = append(cur.files, files...)
	}
	return treeNodeToUI(project, root), sortUITreeFilesByPath(root.files)
}

func treeNodeToUI(base string, n *treeNode) []UITreeNode {
	names := make([]string, 0, len(n.children))
	for name := range n.children {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]UITreeNode, 0, len(names))
	for _, name := range names {
		child := n.children[name]
		path := base + "/" + name
		node := UITreeNode{
			Name:     name,
			Path:     path,
			Files:    sortUITreeFilesByPath(child.files),
			Children: treeNodeToUI(path, child),
		}
		out = append(out, node)
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
