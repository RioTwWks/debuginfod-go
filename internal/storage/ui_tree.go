package storage

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/your-username/debuginfod-go/internal/fnmatch"
)

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

// UITreeNode — узел дерева (проект или каталог).
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
func (s *Storage) SearchDebugFilesForUI(ctx context.Context, scanRoots []string, query string, limit int) ([]ArtifactRecord, error) {
	if limit <= 0 || limit > 5000 {
		limit = 2000
	}
	rows, err := s.db.QueryContext(ctx, rebind(`
		SELECT `+artifactSelectColumns+`
		FROM artifacts
		ORDER BY file_path
	`, s.dialect))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	query = strings.TrimSpace(query)
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

// BuildUITree группирует файлы: проект → каталоги → *.debug.
func BuildUITree(scanRoots []string, records []ArtifactRecord) []UITreeNode {
	files := make([]UITreeFile, 0, len(records))
	for _, rec := range records {
		files = append(files, ArtifactRecordToUITreeFile(rec, scanRoots))
	}
	return BuildUITreeFromFiles(files)
}

// BuildUITreeFromFiles строит дерево из готовых листьев.
func BuildUITreeFromFiles(files []UITreeFile) []UITreeNode {
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
		root := UITreeNode{Name: pname, Path: pname}
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
	return treeNodeToUI(project, root), sortUITreeFiles(root.files)
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
			Files:    sortUITreeFiles(child.files),
			Children: treeNodeToUI(path, child),
		}
		out = append(out, node)
	}
	return out
}

func sortUITreeFiles(files []UITreeFile) []UITreeFile {
	if len(files) == 0 {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Filename < files[j].Filename
	})
	out := make([]UITreeFile, len(files))
	copy(out, files)
	return out
}
