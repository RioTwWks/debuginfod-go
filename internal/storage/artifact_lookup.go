package storage

import (
	"database/sql"
	"strings"
)

const artifactLookupChunk = 400

// GitCommitByFilePath возвращает git_commit из artifacts по пути файла.
func (s *Storage) GitCommitByFilePath(filePath string) (string, bool, error) {
	commits, err := s.GitCommitsByFilePaths([]string{filePath})
	if err != nil {
		return "", false, err
	}
	commit, ok := commits[filePath]
	return commit, ok, nil
}

// GitCommitsByFilePaths возвращает map[file_path]git_commit для уже проиндексированных файлов.
func (s *Storage) GitCommitsByFilePaths(paths []string) (map[string]string, error) {
	out := make(map[string]string, len(paths))
	if len(paths) == 0 {
		return out, nil
	}
	for i := 0; i < len(paths); i += artifactLookupChunk {
		end := i + artifactLookupChunk
		if end > len(paths) {
			end = len(paths)
		}
		chunk := paths[i:end]
		part, err := s.gitCommitsChunk(chunk)
		if err != nil {
			return nil, err
		}
		for k, v := range part {
			out[k] = v
		}
	}
	return out, nil
}

func (s *Storage) gitCommitsChunk(paths []string) (map[string]string, error) {
	out := make(map[string]string, len(paths))
	if len(paths) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(paths))
	args := make([]any, len(paths))
	for i, p := range paths {
		placeholders[i] = "?"
		args[i] = p
	}
	q := rebind(`
		SELECT file_path, git_commit
		FROM artifacts
		WHERE file_path IN (`+strings.Join(placeholders, ",")+`)
		  AND git_commit != ''
		ORDER BY CASE WHEN type = 'debuginfo' THEN 0 ELSE 1 END
	`, s.dialect)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var path, commit string
		if err := rows.Scan(&path, &commit); err != nil {
			return nil, err
		}
		if _, exists := out[path]; !exists {
			out[path] = commit
		}
	}
	return out, rows.Err()
}

// DedupSnapshotsByPaths возвращает снимки dedup_files для набора путей.
func (s *Storage) DedupSnapshotsByPaths(paths []string) (map[string]DedupFileSnapshot, error) {
	out := make(map[string]DedupFileSnapshot, len(paths))
	if len(paths) == 0 {
		return out, nil
	}
	for i := 0; i < len(paths); i += artifactLookupChunk {
		end := i + artifactLookupChunk
		if end > len(paths) {
			end = len(paths)
		}
		chunk := paths[i:end]
		part, err := s.dedupSnapshotsChunk(chunk)
		if err != nil {
			return nil, err
		}
		for k, v := range part {
			out[k] = v
		}
	}
	return out, nil
}

func (s *Storage) dedupSnapshotsChunk(paths []string) (map[string]DedupFileSnapshot, error) {
	out := make(map[string]DedupFileSnapshot, len(paths))
	placeholders := make([]string, len(paths))
	args := make([]any, len(paths))
	for i, p := range paths {
		placeholders[i] = "?"
		args[i] = p
	}
	q := rebind(`
		SELECT file_path, status, original_size, commit_tag
		FROM dedup_files
		WHERE file_path IN (`+strings.Join(placeholders, ",")+`)
	`, s.dialect)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var snap DedupFileSnapshot
		var path string
		if err := rows.Scan(&path, &snap.Status, &snap.OriginalSize, &snap.CommitTag); err != nil {
			return nil, err
		}
		out[path] = snap
	}
	return out, rows.Err()
}

// UpsertDedupFilesBatch регистрирует файлы в одной транзакции.
func (s *Storage) UpsertDedupFilesBatch(files []DedupFile) (int, error) {
	if len(files) == 0 {
		return 0, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	n := 0
	for _, f := range files {
		if _, err := s.upsertDedupFileTx(tx, f); err != nil {
			return n, err
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Storage) upsertDedupFileTx(tx *sql.Tx, f DedupFile) (int64, error) {
	q := rebind(`
		INSERT INTO dedup_files (
			build_dir_id, file_path, filename, file_stem, version,
			file_build_num, commit_tag, original_size, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending')
		ON CONFLICT(file_path) DO UPDATE SET
			commit_tag = excluded.commit_tag,
			original_size = excluded.original_size,
			status = CASE
				WHEN dedup_files.storage_kind IN ('full', 'compressed', 'ref') AND dedup_files.delta_path = '' THEN 'pending'
				WHEN dedup_files.storage_kind IN ('base', 'delta') THEN 'pending'
				ELSE dedup_files.status
			END,
			storage_kind = CASE
				WHEN dedup_files.storage_kind IN ('base', 'delta') THEN 'full'
				ELSE dedup_files.storage_kind
			END
	`, s.dialect)
	if s.dialect == DialectPostgres {
		q = rebind(`
			INSERT INTO dedup_files (
				build_dir_id, file_path, filename, file_stem, version,
				file_build_num, commit_tag, original_size, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending')
			ON CONFLICT (file_path) DO UPDATE SET
				commit_tag = EXCLUDED.commit_tag,
				original_size = EXCLUDED.original_size,
				status = CASE
					WHEN dedup_files.storage_kind IN ('full', 'compressed', 'ref') AND dedup_files.delta_path = '' THEN 'pending'
					WHEN dedup_files.storage_kind IN ('base', 'delta') THEN 'pending'
					ELSE dedup_files.status
				END,
				storage_kind = CASE
					WHEN dedup_files.storage_kind IN ('base', 'delta') THEN 'full'
					ELSE dedup_files.storage_kind
				END
			RETURNING id
		`, s.dialect)
		var id int64
		err := tx.QueryRow(q,
			f.BuildDirID, f.FilePath, f.Filename, f.FileStem, f.Version,
			f.FileBuildNum, f.CommitTag, f.OriginalSize,
		).Scan(&id)
		return id, err
	}
	if _, err := tx.Exec(q,
		f.BuildDirID, f.FilePath, f.Filename, f.FileStem, f.Version,
		f.FileBuildNum, f.CommitTag, f.OriginalSize,
	); err != nil {
		return 0, err
	}
	var id int64
	err := tx.QueryRow(rebind(`SELECT id FROM dedup_files WHERE file_path = ?`, s.dialect), f.FilePath).Scan(&id)
	return id, err
}
