package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DedupBuildStatus — статус обработки каталога build_*.
type DedupBuildStatus string

const (
	DedupStatusPending    DedupBuildStatus = "pending"
	DedupStatusProcessing DedupBuildStatus = "processing"
	DedupStatusDone       DedupBuildStatus = "done"
	DedupStatusError      DedupBuildStatus = "error"
	DedupStatusSkipped    DedupBuildStatus = "skipped"
)

// DedupStorageKind — как хранится файл после dedup.
type DedupStorageKind string

const (
	DedupKindFull  DedupStorageKind = "full"
	DedupKindBase  DedupStorageKind = "base"
	DedupKindDelta DedupStorageKind = "delta"
)

// DedupProject — проект Quik (QuikServer, Front).
type DedupProject struct {
	ID   int64
	Name string
}

// DedupBuildDir — каталог build_* на диске.
type DedupBuildDir struct {
	ID          int64
	ProjectID   int64
	ProjectName string
	DirPath     string
	DirBuildNum int
	Status      DedupBuildStatus
	ErrorMsg    string
	ProcessedAt time.Time
}

// DedupFile — метаданные одного .debug для dedup.
type DedupFile struct {
	ID           int64
	BuildDirID   int64
	ProjectName  string
	FilePath     string
	Filename     string
	FileStem     string
	Version      string
	FileBuildNum int
	CommitTag    string
	StorageKind  DedupStorageKind
	BaseFileID   sql.NullInt64
	DeltaPath    string
	SHA256       string
	OriginalSize int64
	Status       DedupBuildStatus
	ErrorMsg     string
}

// DedupGroupKey — ключ группировки для xdelta.
type DedupGroupKey struct {
	Project   string
	FileStem  string
	Version   string
	CommitTag string
}

func dedupSchemaSQLite() string {
	return `
		CREATE TABLE IF NOT EXISTS dedup_projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		);

		CREATE TABLE IF NOT EXISTS dedup_build_dirs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			dir_path TEXT NOT NULL UNIQUE,
			dir_build_num INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			error_msg TEXT NOT NULL DEFAULT '',
			processed_at INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (project_id) REFERENCES dedup_projects(id)
		);
		CREATE INDEX IF NOT EXISTS idx_dedup_build_dirs_status ON dedup_build_dirs(status);

		CREATE TABLE IF NOT EXISTS dedup_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			build_dir_id INTEGER NOT NULL,
			file_path TEXT NOT NULL UNIQUE,
			filename TEXT NOT NULL,
			file_stem TEXT NOT NULL,
			version TEXT NOT NULL,
			file_build_num INTEGER NOT NULL,
			commit_tag TEXT NOT NULL DEFAULT '',
			storage_kind TEXT NOT NULL DEFAULT 'full',
			base_file_id INTEGER,
			delta_path TEXT NOT NULL DEFAULT '',
			sha256 TEXT NOT NULL DEFAULT '',
			original_size INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			error_msg TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (build_dir_id) REFERENCES dedup_build_dirs(id),
			FOREIGN KEY (base_file_id) REFERENCES dedup_files(id)
		);
		CREATE INDEX IF NOT EXISTS idx_dedup_files_status ON dedup_files(status);
		CREATE INDEX IF NOT EXISTS idx_dedup_files_group ON dedup_files(file_stem, version, commit_tag);
	`
}

func dedupSchemaPostgres() string {
	return `
		CREATE TABLE IF NOT EXISTS dedup_projects (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE
		);

		CREATE TABLE IF NOT EXISTS dedup_build_dirs (
			id BIGSERIAL PRIMARY KEY,
			project_id BIGINT NOT NULL REFERENCES dedup_projects(id),
			dir_path TEXT NOT NULL UNIQUE,
			dir_build_num BIGINT NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			error_msg TEXT NOT NULL DEFAULT '',
			processed_at BIGINT NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_dedup_build_dirs_status ON dedup_build_dirs(status);

		CREATE TABLE IF NOT EXISTS dedup_files (
			id BIGSERIAL PRIMARY KEY,
			build_dir_id BIGINT NOT NULL REFERENCES dedup_build_dirs(id),
			file_path TEXT NOT NULL UNIQUE,
			filename TEXT NOT NULL,
			file_stem TEXT NOT NULL,
			version TEXT NOT NULL,
			file_build_num BIGINT NOT NULL,
			commit_tag TEXT NOT NULL DEFAULT '',
			storage_kind TEXT NOT NULL DEFAULT 'full',
			base_file_id BIGINT REFERENCES dedup_files(id),
			delta_path TEXT NOT NULL DEFAULT '',
			sha256 TEXT NOT NULL DEFAULT '',
			original_size BIGINT NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			error_msg TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_dedup_files_status ON dedup_files(status);
		CREATE INDEX IF NOT EXISTS idx_dedup_files_group ON dedup_files(file_stem, version, commit_tag);
	`
}

func migrateDedup(db *sql.DB, dialect Dialect) error {
	schema := dedupSchemaSQLite()
	if dialect == DialectPostgres {
		schema = dedupSchemaPostgres()
	}
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate dedup: %w", err)
	}
	return nil
}

// EnsureDedupProject создаёт проект, если его ещё нет.
func (s *Storage) EnsureDedupProject(name string) (int64, error) {
	if s.dialect == DialectPostgres {
		var id int64
		err := s.db.QueryRow(rebind(`
			INSERT INTO dedup_projects (name) VALUES (?)
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`, s.dialect), name).Scan(&id)
		if err == nil {
			return id, nil
		}
	}
	_, err := s.db.Exec(rebind(`INSERT OR IGNORE INTO dedup_projects (name) VALUES (?)`, s.dialect), name)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRow(rebind(`SELECT id FROM dedup_projects WHERE name = ?`, s.dialect), name).Scan(&id)
	return id, err
}

// UpsertDedupBuildDir регистрирует каталог build_*.
func (s *Storage) UpsertDedupBuildDir(projectID int64, dirPath string, dirBuildNum int) (int64, error) {
	if s.dialect == DialectPostgres {
		var id int64
		err := s.db.QueryRow(rebind(`
			INSERT INTO dedup_build_dirs (project_id, dir_path, dir_build_num, status)
			VALUES (?, ?, ?, 'pending')
			ON CONFLICT (dir_path) DO UPDATE SET dir_build_num = EXCLUDED.dir_build_num
			RETURNING id
		`, s.dialect), projectID, dirPath, dirBuildNum).Scan(&id)
		return id, err
	}
	_, err := s.db.Exec(rebind(`
		INSERT INTO dedup_build_dirs (project_id, dir_path, dir_build_num, status)
		VALUES (?, ?, ?, 'pending')
		ON CONFLICT(dir_path) DO UPDATE SET dir_build_num = excluded.dir_build_num
	`, s.dialect), projectID, dirPath, dirBuildNum)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRow(rebind(`SELECT id FROM dedup_build_dirs WHERE dir_path = ?`, s.dialect), dirPath).Scan(&id)
	return id, err
}

// UpsertDedupFile регистрирует .debug файл для dedup.
func (s *Storage) UpsertDedupFile(f DedupFile) (int64, error) {
	q := rebind(`
		INSERT INTO dedup_files (
			build_dir_id, file_path, filename, file_stem, version,
			file_build_num, commit_tag, original_size, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending')
		ON CONFLICT(file_path) DO UPDATE SET
			commit_tag = excluded.commit_tag,
			original_size = excluded.original_size
	`, s.dialect)
	if s.dialect == DialectPostgres {
		q = rebind(`
			INSERT INTO dedup_files (
				build_dir_id, file_path, filename, file_stem, version,
				file_build_num, commit_tag, original_size, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending')
			ON CONFLICT (file_path) DO UPDATE SET
				commit_tag = EXCLUDED.commit_tag,
				original_size = EXCLUDED.original_size
			RETURNING id
		`, s.dialect)
		var id int64
		err := s.db.QueryRow(q,
			f.BuildDirID, f.FilePath, f.Filename, f.FileStem, f.Version,
			f.FileBuildNum, f.CommitTag, f.OriginalSize,
		).Scan(&id)
		return id, err
	}
	_, err := s.db.Exec(q,
		f.BuildDirID, f.FilePath, f.Filename, f.FileStem, f.Version,
		f.FileBuildNum, f.CommitTag, f.OriginalSize,
	)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRow(rebind(`SELECT id FROM dedup_files WHERE file_path = ?`, s.dialect), f.FilePath).Scan(&id)
	return id, err
}

// ListPendingBuildDirs возвращает каталоги build_* со статусом pending.
func (s *Storage) ListPendingBuildDirs(projectName string, limit int) ([]DedupBuildDir, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(rebind(`
		SELECT b.id, b.project_id, p.name, b.dir_path, b.dir_build_num,
			b.status, b.error_msg, b.processed_at
		FROM dedup_build_dirs b
		JOIN dedup_projects p ON p.id = b.project_id
		WHERE b.status = 'pending' AND (? = '' OR p.name = ?)
		ORDER BY b.dir_build_num
		LIMIT ?
	`, s.dialect), projectName, projectName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBuildDirs(rows)
}

// ListPendingDedupFiles возвращает файлы pending в указанных build dirs.
func (s *Storage) ListPendingDedupFiles(buildDirIDs []int64) ([]DedupFile, error) {
	if len(buildDirIDs) == 0 {
		return nil, nil
	}
	placeholders, args := inClause(buildDirIDs)
	q := fmt.Sprintf(`
		SELECT f.id, f.build_dir_id, p.name, f.file_path, f.filename,
			f.file_stem, f.version, f.file_build_num, f.commit_tag,
			f.storage_kind, f.base_file_id, f.delta_path, f.sha256,
			f.original_size, f.status, f.error_msg
		FROM dedup_files f
		JOIN dedup_build_dirs b ON b.id = f.build_dir_id
		JOIN dedup_projects p ON p.id = b.project_id
		WHERE f.status = 'pending' AND f.build_dir_id IN (%s)
	`, placeholders)
	rows, err := s.db.Query(rebind(q, s.dialect), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDedupFiles(rows)
}

// ListPendingDedupFilesByProject — все pending файлы проекта для группировки.
func (s *Storage) ListPendingDedupFilesByProject(projectName string) ([]DedupFile, error) {
	rows, err := s.db.Query(rebind(`
		SELECT f.id, f.build_dir_id, p.name, f.file_path, f.filename,
			f.file_stem, f.version, f.file_build_num, f.commit_tag,
			f.storage_kind, f.base_file_id, f.delta_path, f.sha256,
			f.original_size, f.status, f.error_msg
		FROM dedup_files f
		JOIN dedup_build_dirs b ON b.id = f.build_dir_id
		JOIN dedup_projects p ON p.id = b.project_id
		WHERE f.status = 'pending' AND p.name = ?
		ORDER BY f.file_build_num
	`, s.dialect), projectName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDedupFiles(rows)
}

// GetDedupFileByPath возвращает метаданные dedup по пути файла.
func (s *Storage) GetDedupFileByPath(filePath string) (DedupFile, error) {
	return lookupDedupByPath(s, filePath)
}

// GetDedupFileByID загружает запись по id.
func (s *Storage) GetDedupFileByID(id int64) (DedupFile, error) {
	rows, err := s.db.Query(rebind(`
		SELECT f.id, f.build_dir_id, p.name, f.file_path, f.filename,
			f.file_stem, f.version, f.file_build_num, f.commit_tag,
			f.storage_kind, f.base_file_id, f.delta_path, f.sha256,
			f.original_size, f.status, f.error_msg
		FROM dedup_files f
		JOIN dedup_build_dirs b ON b.id = f.build_dir_id
		JOIN dedup_projects p ON p.id = b.project_id
		WHERE f.id = ?
	`, s.dialect), id)
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

// MarkDedupFileDone обновляет статус файла после успешного dedup.
func (s *Storage) MarkDedupFileDone(id int64, kind DedupStorageKind, baseID int64, deltaPath, sha256 string) error {
	var base sql.NullInt64
	if baseID > 0 {
		base = sql.NullInt64{Int64: baseID, Valid: true}
	}
	_, err := s.db.Exec(rebind(`
		UPDATE dedup_files SET
			storage_kind = ?, base_file_id = ?, delta_path = ?,
			sha256 = ?, status = 'done', error_msg = ''
		WHERE id = ?
	`, s.dialect), string(kind), base, deltaPath, sha256, id)
	return err
}

// MarkDedupFileError сохраняет ошибку обработки файла.
func (s *Storage) MarkDedupFileError(id int64, msg string) error {
	_, err := s.db.Exec(rebind(`
		UPDATE dedup_files SET status = 'error', error_msg = ? WHERE id = ?
	`, s.dialect), msg, id)
	return err
}

// MarkDedupBuildDirDone помечает каталог build_* обработанным.
func (s *Storage) MarkDedupBuildDirDone(id int64) error {
	_, err := s.db.Exec(rebind(`
		UPDATE dedup_build_dirs SET status = 'done', processed_at = ?, error_msg = ''
		WHERE id = ?
	`, s.dialect), time.Now().Unix(), id)
	return err
}

// MarkDedupBuildDirError сохраняет ошибку каталога.
func (s *Storage) MarkDedupBuildDirError(id int64, msg string) error {
	_, err := s.db.Exec(rebind(`
		UPDATE dedup_build_dirs SET status = 'error', error_msg = ?, processed_at = ?
		WHERE id = ?
	`, s.dialect), msg, time.Now().Unix(), id)
	return err
}

// SetDedupBuildDirStatus устанавливает статус каталога.
func (s *Storage) SetDedupBuildDirStatus(id int64, status DedupBuildStatus) error {
	_, err := s.db.Exec(rebind(`UPDATE dedup_build_dirs SET status = ? WHERE id = ?`, s.dialect), string(status), id)
	return err
}

// DedupStats — агрегаты для мониторинга dedup.
type DedupStats struct {
	FilesTotal    int64
	FilesPending  int64
	FilesDone     int64
	FilesDelta    int64
	BytesOriginal int64
	BytesDelta    int64
}

// DedupStats возвращает счётчики dedup.
func (s *Storage) DedupStats() (DedupStats, error) {
	var st DedupStats
	_ = s.db.QueryRow(rebind(`SELECT COUNT(1) FROM dedup_files`, s.dialect)).Scan(&st.FilesTotal)
	_ = s.db.QueryRow(rebind(`SELECT COUNT(1) FROM dedup_files WHERE status = 'pending'`, s.dialect)).Scan(&st.FilesPending)
	_ = s.db.QueryRow(rebind(`SELECT COUNT(1) FROM dedup_files WHERE status = 'done'`, s.dialect)).Scan(&st.FilesDone)
	_ = s.db.QueryRow(rebind(`SELECT COUNT(1) FROM dedup_files WHERE storage_kind = 'delta'`, s.dialect)).Scan(&st.FilesDelta)
	_ = s.db.QueryRow(rebind(`SELECT COALESCE(SUM(original_size), 0) FROM dedup_files`, s.dialect)).Scan(&st.BytesOriginal)
	_ = s.db.QueryRow(rebind(`
		SELECT COALESCE(SUM(
			CASE WHEN delta_path != '' THEN original_size ELSE 0 END
		), 0) FROM dedup_files WHERE storage_kind = 'delta'
	`, s.dialect)).Scan(&st.BytesDelta)
	return st, nil
}

func scanBuildDirs(rows *sql.Rows) ([]DedupBuildDir, error) {
	var out []DedupBuildDir
	for rows.Next() {
		var b DedupBuildDir
		var processed int64
		if err := rows.Scan(
			&b.ID, &b.ProjectID, &b.ProjectName, &b.DirPath, &b.DirBuildNum,
			&b.Status, &b.ErrorMsg, &processed,
		); err != nil {
			return nil, err
		}
		if processed > 0 {
			b.ProcessedAt = time.Unix(processed, 0)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func scanDedupFiles(rows *sql.Rows) ([]DedupFile, error) {
	var out []DedupFile
	for rows.Next() {
		var f DedupFile
		if err := rows.Scan(
			&f.ID, &f.BuildDirID, &f.ProjectName, &f.FilePath, &f.Filename,
			&f.FileStem, &f.Version, &f.FileBuildNum, &f.CommitTag,
			&f.StorageKind, &f.BaseFileID, &f.DeltaPath, &f.SHA256,
			&f.OriginalSize, &f.Status, &f.ErrorMsg,
		); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func inClause(ids []int64) (string, []any) {
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	return joinStrings(ph, ","), args
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += sep + parts[i]
	}
	return out
}

// HasPendingFilesInBuildDir проверяет, остались ли pending файлы в каталоге.
func (s *Storage) HasPendingFilesInBuildDir(buildDirID int64) (bool, error) {
	var n int
	err := s.db.QueryRow(rebind(`
		SELECT COUNT(1) FROM dedup_files WHERE build_dir_id = ? AND status = 'pending'
	`, s.dialect), buildDirID).Scan(&n)
	return n > 0, err
}

// FinishBuildDirIfDone помечает каталог done, если все файлы обработаны.
func (s *Storage) FinishBuildDirIfDone(buildDirID int64) error {
	pending, err := s.HasPendingFilesInBuildDir(buildDirID)
	if err != nil {
		return err
	}
	if pending {
		return nil
	}
	var errCount int
	_ = s.db.QueryRow(rebind(`
		SELECT COUNT(1) FROM dedup_files WHERE build_dir_id = ? AND status = 'error'
	`, s.dialect), buildDirID).Scan(&errCount)
	if errCount > 0 {
		return s.MarkDedupBuildDirError(buildDirID, fmt.Sprintf("%d file(s) failed", errCount))
	}
	return s.MarkDedupBuildDirDone(buildDirID)
}

// ErrDedupConflict — конфликт при обновлении dedup.
var ErrDedupConflict = errors.New("dedup conflict")
