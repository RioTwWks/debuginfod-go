package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"sort"
	"strings"
)

// BrowseFilesForUI объединяет артефакты индекса и dedup-файлы без build-id.
// limit <= 0 — вернуть все совпадения (без обрезки).
func (s *Storage) BrowseFilesForUI(ctx context.Context, scanRoots []string, query string, limit int) ([]UITreeFile, bool, error) {
	if limit > 50000 {
		limit = 50000
	}

	artifacts, err := s.SearchDebugFilesForUI(ctx, scanRoots, query, 0)
	if err != nil {
		return nil, false, err
	}

	indexed := artifactPathSet(artifacts)
	dedupFiles, err := s.SearchDedupFilesForUI(ctx, scanRoots, query, indexed)
	if err != nil {
		return nil, false, err
	}

	enrichComment := strings.TrimSpace(query) != ""
	files := make([]UITreeFile, 0, len(artifacts)+len(dedupFiles))
	for i := range artifacts {
		files = append(files, ArtifactRecordToUITreeFile(artifacts[i], scanRoots, enrichComment))
	}
	for _, df := range dedupFiles {
		files = append(files, DedupFileToUITreeFile(df, scanRoots))
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].RelativePath == files[j].RelativePath {
			return files[i].Filename < files[j].Filename
		}
		return files[i].RelativePath < files[j].RelativePath
	})

	complete := true
	if limit > 0 && len(files) > limit {
		files = files[:limit]
		complete = false
	}
	if files == nil {
		files = []UITreeFile{}
	}
	return files, complete, nil
}

// SearchDedupFilesForUI возвращает dedup-файлы, отсутствующие в индексе артефактов.
func (s *Storage) SearchDedupFilesForUI(ctx context.Context, scanRoots []string, query string, skipPaths map[string]struct{}) ([]DedupFile, error) {
	query = strings.TrimSpace(query)
	var rows *sql.Rows
	var err error
	if isSimpleSearchQuery(query) {
		where, args := dedupSearchSQLFilter(query)
		rows, err = s.db.QueryContext(ctx, rebind(`
			SELECT f.id, f.build_dir_id, p.name, f.file_path, f.filename,
				f.file_stem, f.version, f.file_build_num, f.commit_tag,
				f.storage_kind, f.base_file_id, f.delta_path, f.sha256,
				f.original_size, f.compressed_size, f.status, f.error_msg
			FROM dedup_files f
			JOIN dedup_build_dirs b ON b.id = f.build_dir_id
			JOIN dedup_projects p ON p.id = b.project_id
			WHERE f.status != 'error'
			`+where+`
			ORDER BY f.file_path
		`, s.dialect), args...)
	} else {
		rows, err = s.db.QueryContext(ctx, rebind(`
			SELECT f.id, f.build_dir_id, p.name, f.file_path, f.filename,
				f.file_stem, f.version, f.file_build_num, f.commit_tag,
				f.storage_kind, f.base_file_id, f.delta_path, f.sha256,
				f.original_size, f.compressed_size, f.status, f.error_msg
			FROM dedup_files f
			JOIN dedup_build_dirs b ON b.id = f.build_dir_id
			JOIN dedup_projects p ON p.id = b.project_id
			WHERE f.status != 'error'
			ORDER BY f.file_path
		`, s.dialect))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DedupFile
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		var df DedupFile
		if err := rows.Scan(
			&df.ID, &df.BuildDirID, &df.ProjectName, &df.FilePath, &df.Filename,
			&df.FileStem, &df.Version, &df.FileBuildNum, &df.CommitTag,
			&df.StorageKind, &df.BaseFileID, &df.BlobPath, &df.SHA256,
			&df.OriginalSize, &df.CompressedSize, &df.Status, &df.ErrorMsg,
		); err != nil {
			return nil, err
		}
		if skipPaths != nil {
			if _, skip := skipPaths[normalizeBrowsePath(df.FilePath)]; skip {
				continue
			}
		}
		rel := RelativeToScanRoots(df.FilePath, scanRoots)
		if query != "" && !matchesUnifiedQueryForDedup(query, df, rel) {
			continue
		}
		out = append(out, df)
	}
	if out == nil {
		out = []DedupFile{}
	}
	return out, rows.Err()
}

func artifactPathSet(records []ArtifactRecord) map[string]struct{} {
	set := make(map[string]struct{}, len(records))
	for _, rec := range records {
		if p := artifactAbsPath(rec); p != "" {
			set[normalizeBrowsePath(p)] = struct{}{}
		}
	}
	return set
}

func artifactAbsPath(rec ArtifactRecord) string {
	if rec.FilePath != "" {
		return rec.FilePath
	}
	return strings.TrimSpace(rec.File)
}

func normalizeBrowsePath(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func matchesUnifiedQueryForDedup(query string, df DedupFile, rel string) bool {
	rec := ArtifactRecord{
		RelativePath: rel,
		Filename:     df.Filename,
		GitCommit:    df.CommitTag,
	}
	return matchesUnifiedQuery(query, rec)
}

func dedupSearchSQLFilter(query string) (string, []any) {
	pattern := "%" + strings.ToLower(query) + "%"
	return `
		AND (
			LOWER(f.file_path) LIKE ? OR
			LOWER(f.filename) LIKE ? OR
			LOWER(f.commit_tag) LIKE ?
		)
	`, []any{pattern, pattern, pattern}
}

// ArtifactRecordToUITreeFile конвертирует артефакт в лист дерева.
func ArtifactRecordToUITreeFile(rec ArtifactRecord, scanRoots []string, enrichComment bool) UITreeFile {
	EnrichArtifactRecord(&rec, scanRoots)
	if enrichComment {
		EnrichArtifactComment(&rec)
	}
	rel := rec.RelativePath
	if rel == "" {
		rel = ArtifactDisplayPath(rec, scanRoots)
	}
	gitCommit := rec.GitCommit
	if gitCommit == "" && rec.Comment != nil {
		gitCommit = rec.Comment.GitCommit
	}
	return UITreeFile{
		Filename:     rec.Filename,
		RelativePath: rel,
		Project:      UIProjectFromRelativePath(rel),
		BuildID:      rec.BuildID,
		Source:       "artifact",
		Type:         rec.Type,
		GitCommit:    gitCommit,
		Comment:      rec.Comment,
	}
}

// DedupFileToUITreeFile конвертирует dedup-запись в лист дерева.
func DedupFileToUITreeFile(df DedupFile, scanRoots []string) UITreeFile {
	rel := RelativeToScanRoots(df.FilePath, scanRoots)
	return UITreeFile{
		Filename:     df.Filename,
		RelativePath: rel,
		Project:      df.ProjectName,
		DedupID:      df.ID,
		Source:       "dedup",
		Type:         "debuginfo",
		GitCommit:    df.CommitTag,
	}
}
