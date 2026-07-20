package storage

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/your-username/debuginfod-go/internal/fnmatch"
)

// UIGroupedArtifact — артефакты одного build-id для Web UI.
type UIGroupedArtifact struct {
	BuildID      string            `json:"buildid"`
	Types        []string          `json:"types"`
	Type         string            `json:"type"`
	File         string            `json:"file"`
	RelativePath string            `json:"relative_path,omitempty"`
	Filename     string            `json:"filename,omitempty"`
	Directory    string            `json:"directory,omitempty"`
	Archive      string            `json:"archive,omitempty"`
	BuildIDKind  string            `json:"buildid_kind,omitempty"`
	RawBuildID   string            `json:"raw_buildid,omitempty"`
	ByType       map[string]string `json:"by_type,omitempty"`
	ByTypeRel    map[string]string `json:"by_type_rel,omitempty"`
}

// SearchBuildIDGroupedForUI ищет артефакты и группирует по build-id.
func (s *Storage) SearchBuildIDGroupedForUI(ctx context.Context, query string, limit int, scanRoots []string) ([]UIGroupedArtifact, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rawLimit := limit * 4
	if rawLimit > 200 {
		rawLimit = 200
	}

	records, err := s.SearchBuildIDForUI(ctx, query, rawLimit)
	if err != nil {
		return nil, err
	}

	groups := make(map[string]*UIGroupedArtifact)
	order := make([]string, 0, len(records))

	for _, rec := range records {
		EnrichArtifactRecord(&rec, scanRoots)
		g, ok := groups[rec.BuildID]
		if !ok {
			g = &UIGroupedArtifact{
				BuildID:   rec.BuildID,
				ByType:    make(map[string]string),
				ByTypeRel: make(map[string]string),
			}
			groups[rec.BuildID] = g
			order = append(order, rec.BuildID)
		}
		if !containsStr(g.Types, rec.Type) {
			g.Types = append(g.Types, rec.Type)
		}
		fileLabel := rec.File
		if rec.Archive != "" {
			fileLabel = rec.Archive + " → " + rec.File
		}
		g.ByType[rec.Type] = fileLabel
		g.ByTypeRel[rec.Type] = rec.RelativePath
		if rec.BuildIDKind != "" {
			g.BuildIDKind = rec.BuildIDKind
		}
		if rec.RawBuildID != "" {
			g.RawBuildID = rec.RawBuildID
		}
	}

	out := make([]UIGroupedArtifact, 0, len(order))
	for _, id := range order {
		g := groups[id]
		sort.Strings(g.Types)
		g.Type = primaryType(g.Types)
		g.File = g.ByType[g.Type]
		g.RelativePath = g.ByTypeRel[g.Type]
		if g.RelativePath == "" {
			for _, t := range g.Types {
				g.RelativePath = g.ByTypeRel[t]
				g.File = g.ByType[t]
				break
			}
		}
		g.Filename = filepath.Base(g.RelativePath)
		if i := strings.LastIndex(g.RelativePath, "/"); i >= 0 {
			g.Filename = g.RelativePath[i+1:]
			g.Directory = g.RelativePath[:i]
		}
		out = append(out, *g)
		if len(out) >= limit {
			break
		}
	}
	if out == nil {
		out = []UIGroupedArtifact{}
	}
	return out, nil
}

// SearchPathForUI ищет по относительному пути (подстрока или fnmatch) от scan root.
func (s *Storage) SearchPathForUI(ctx context.Context, scanRoots []string, query string, offset, limit int) (MetadataResponse, error) {
	rows, err := s.db.QueryContext(ctx, rebind(`
		SELECT build_id, file_path, type, archive_path, member_path, build_id_kind, raw_build_id
		FROM artifacts
		ORDER BY file_path, type
	`, s.dialect))
	if err != nil {
		return MetadataResponse{}, err
	}
	defer rows.Close()
	return collectMetadata(ctx, rows, func(rec ArtifactRecord) bool {
		rel := ArtifactDisplayPath(rec, scanRoots)
		return matchPathQuery(query, rel)
	}, offset, limit, func(rec *ArtifactRecord) {
		EnrichArtifactRecord(rec, scanRoots)
	})
}

// SearchNameForUI ищет по имени файла (подстрока или fnmatch).
func (s *Storage) SearchNameForUI(ctx context.Context, scanRoots []string, query string, offset, limit int) (MetadataResponse, error) {
	rows, err := s.db.QueryContext(ctx, rebind(`
		SELECT build_id, file_path, type, archive_path, member_path, build_id_kind, raw_build_id
		FROM artifacts
		ORDER BY file_path, type
	`, s.dialect))
	if err != nil {
		return MetadataResponse{}, err
	}
	defer rows.Close()
	return collectMetadata(ctx, rows, func(rec ArtifactRecord) bool {
		return matchNameQuery(query, ArtifactFilename(rec))
	}, offset, limit, func(rec *ArtifactRecord) {
		EnrichArtifactRecord(rec, scanRoots)
	})
}

func matchPathQuery(query, relativePath string) bool {
	query = strings.TrimSpace(query)
	rel := filepath.ToSlash(relativePath)
	if query == "" {
		return true
	}
	q := filepath.ToSlash(query)
	if strings.ContainsAny(q, "*?[") {
		return fnmatch.Match(q, rel, fnmatch.Pathname) || fnmatch.Match(q, rel, 0)
	}
	lowerQ := strings.ToLower(q)
	lowerRel := strings.ToLower(rel)
	return strings.Contains(lowerRel, lowerQ) || strings.HasSuffix(lowerRel, lowerQ)
}

func matchNameQuery(query, filename string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}
	q := filepath.ToSlash(query)
	if strings.ContainsAny(q, "*?[") {
		return fnmatch.Match(q, filename, 0)
	}
	lowerQ := strings.ToLower(q)
	lowerName := strings.ToLower(filename)
	return strings.Contains(lowerName, lowerQ) || lowerName == lowerQ
}

func primaryType(types []string) string {
	for _, t := range types {
		if t == "debuginfo" {
			return "debuginfo"
		}
	}
	if len(types) > 0 {
		return types[0]
	}
	return "executable"
}

func containsStr(slice []string, v string) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}

// lookupDedupByPath ищет dedup-запись с нормализацией пути.
func lookupDedupByPath(s *Storage, filePath string) (DedupFile, error) {
	df, err := s.getDedupFileByPathExact(filePath)
	if err == nil {
		return df, nil
	}
	if err != ErrNotFound {
		return DedupFile{}, err
	}
	abs, absErr := filepath.Abs(filePath)
	if absErr == nil && abs != filePath {
		return s.getDedupFileByPathExact(abs)
	}
	clean := filepath.Clean(filePath)
	if clean != filePath {
		return s.getDedupFileByPathExact(clean)
	}
	return DedupFile{}, ErrNotFound
}

func (s *Storage) getDedupFileByPathExact(filePath string) (DedupFile, error) {
	rows, err := s.db.Query(rebind(`
		SELECT f.id, f.build_dir_id, p.name, f.file_path, f.filename,
			f.file_stem, f.version, f.file_build_num, f.commit_tag,
			f.storage_kind, f.base_file_id, f.delta_path, f.sha256,
			f.original_size, f.compressed_size, f.status, f.error_msg
		FROM dedup_files f
		JOIN dedup_build_dirs b ON b.id = f.build_dir_id
		JOIN dedup_projects p ON p.id = b.project_id
		WHERE f.file_path = ?
	`, s.dialect), filePath)
	if err != nil {
		return DedupFile{}, err
	}
	defer rows.Close()
	files, err := scanDedupFiles(rows)
	if err != nil || len(files) == 0 {
		return DedupFile{}, ErrNotFound
	}
	return files[0], nil
}
