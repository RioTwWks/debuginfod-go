package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/your-username/debuginfod-go/internal/fnmatch"

	_ "github.com/mattn/go-sqlite3"
)

// ErrNotFound возвращается, когда запись не найдена в базе.
var ErrNotFound = errors.New("not found")

// ErrMetadataTimeout — metadata-запрос прерван по таймауту.
var ErrMetadataTimeout = errors.New("metadata query timeout")

// ArtifactRecord описывает проиндексированный артефакт для metadata API.
type ArtifactRecord struct {
	BuildID      string `json:"buildid"`
	Type         string `json:"type"`
	File         string `json:"file"`
	FilePath     string `json:"file_path,omitempty"`
	Archive      string `json:"archive,omitempty"`
	ArchivePath  string `json:"archive_path,omitempty"`
	ArchiveRel   string `json:"archive_rel,omitempty"`
	MemberPath   string `json:"member_path,omitempty"`
	BuildIDKind  string `json:"buildid_kind,omitempty"`
	RawBuildID   string `json:"raw_buildid,omitempty"`
	RelativePath string `json:"relative_path,omitempty"`
	Filename     string `json:"filename,omitempty"`
	Directory    string `json:"directory,omitempty"`
	MtimeNs      int64  `json:"mtime_ns,omitempty"`
	Mtime        string `json:"mtime,omitempty"`
	Sources      []UISourceRecord `json:"sources,omitempty"`
	SourcesCount int              `json:"sources_count,omitempty"`
	Comment      *UICommentInfo   `json:"comment,omitempty"`
	GitCommit    string           `json:"git_commit,omitempty"`
}

// MetadataResponse — JSON-ответ эндпоинта /metadata.
type MetadataResponse struct {
	Results    []ArtifactRecord `json:"results"`
	Complete   bool             `json:"complete"`
	NextOffset int              `json:"next_offset,omitempty"`
}

// MetadataQuery — параметры поиска metadata с пагинацией.
type MetadataQuery struct {
	Key    string
	Value  string
	Offset int
	Limit  int // 0 = без лимита (все совпадения)
}

// Storage — SQLite-хранилище метаданных debuginfod.
type Storage struct {
	db      *sql.DB
	dialect Dialect
}

// New открывает SQLite по пути (обратная совместимость).
func New(dbPath string) (*Storage, error) {
	return openSQLite(dbPath)
}

func openSQLite(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := migrateSQLite(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Storage{db: db, dialect: DialectSQLite}, nil
}

func migrateSQLite(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS artifacts (
			build_id TEXT NOT NULL,
			file_path TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			archive_path TEXT NOT NULL DEFAULT '',
			member_path TEXT NOT NULL DEFAULT '',
			build_id_kind TEXT NOT NULL DEFAULT 'gnu',
			raw_build_id TEXT NOT NULL DEFAULT '',
			mtime_ns INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (build_id, type)
		);
		CREATE INDEX IF NOT EXISTS idx_artifacts_build_id ON artifacts(build_id);

		CREATE TABLE IF NOT EXISTS sources (
			build_id TEXT NOT NULL,
			source_path TEXT NOT NULL,
			file_path TEXT NOT NULL,
			archive_path TEXT NOT NULL DEFAULT '',
			member_path TEXT NOT NULL DEFAULT '',
			mtime_ns INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (build_id, source_path)
		);
		CREATE INDEX IF NOT EXISTS idx_sources_build_id ON sources(build_id);

		CREATE TABLE IF NOT EXISTS scanned_files (
			path TEXT PRIMARY KEY,
			mtime_ns INTEGER NOT NULL,
			size INTEGER NOT NULL,
			kind TEXT NOT NULL
		);
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}

	for _, stmt := range []string{
		"ALTER TABLE artifacts ADD COLUMN archive_path TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE artifacts ADD COLUMN member_path TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE artifacts ADD COLUMN build_id_kind TEXT NOT NULL DEFAULT 'gnu'",
		"ALTER TABLE artifacts ADD COLUMN raw_build_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE artifacts ADD COLUMN git_commit TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE sources ADD COLUMN archive_path TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE sources ADD COLUMN member_path TEXT NOT NULL DEFAULT ''",
	} {
		_, _ = db.Exec(stmt)
	}
	if err := migrateDedup(db, DialectSQLite); err != nil {
		return err
	}
	return migrateHistory(db, DialectSQLite)
}

// Close закрывает соединение с базой данных.
func (s *Storage) Close() error {
	return s.db.Close()
}

// ArtifactLocation описывает, где лежит артефакт (на диске или в архиве).
type ArtifactLocation struct {
	FilePath    string
	ArchivePath string
	MemberPath  string
}

// SourceLocation описывает расположение исходного файла.
type SourceLocation struct {
	SourcePath  string
	FilePath    string
	ArchivePath string
	MemberPath  string
}

// ArtifactInput — данные для сохранения артефакта.
type ArtifactInput struct {
	BuildID     string
	Type        string
	FilePath    string
	ArchivePath string
	MemberPath  string
	BuildIDKind string
	RawBuildID  string
	GitCommit   string
}

type artifactRow struct {
	BuildID     string
	Type        string
	FilePath    string
	ArchivePath string
	MemberPath  string
	BuildIDKind string
	RawBuildID  string
	GitCommit   string
	MtimeNs     int64
}

const artifactSelectColumns = `build_id, file_path, type, archive_path, member_path, build_id_kind, raw_build_id, git_commit, mtime_ns`

// AddArtifact сохраняет или обновляет артефакт.
func (s *Storage) AddArtifact(in ArtifactInput, mtimeNS int64) error {
	q := rebind(`
		INSERT INTO artifacts (
			build_id, file_path, type, archive_path, member_path,
			build_id_kind, raw_build_id, git_commit, mtime_ns
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(build_id, type) DO UPDATE SET
			file_path = excluded.file_path,
			archive_path = excluded.archive_path,
			member_path = excluded.member_path,
			build_id_kind = excluded.build_id_kind,
			raw_build_id = excluded.raw_build_id,
			git_commit = excluded.git_commit,
			mtime_ns = excluded.mtime_ns
		WHERE excluded.mtime_ns >= artifacts.mtime_ns
	`, s.dialect)
	_, err := s.db.Exec(q,
		in.BuildID, in.FilePath, in.Type, in.ArchivePath, in.MemberPath,
		in.BuildIDKind, in.RawBuildID, in.GitCommit, mtimeNS)
	return err
}

// GetArtifactPath возвращает путь на диске для отдачи файла клиенту.
func (s *Storage) GetArtifactPath(buildID, artifactType string) (string, error) {
	loc, err := s.GetArtifactLocation(buildID, artifactType)
	if err != nil {
		return "", err
	}
	return loc.FilePath, nil
}

// GetArtifactLocation возвращает расположение артефакта (файл или архив).
func (s *Storage) GetArtifactLocation(buildID, artifactType string) (ArtifactLocation, error) {
	var loc ArtifactLocation
	err := s.db.QueryRow(
		rebind(`SELECT file_path, archive_path, member_path FROM artifacts WHERE build_id = ? AND type = ?`, s.dialect),
		buildID, artifactType,
	).Scan(&loc.FilePath, &loc.ArchivePath, &loc.MemberPath)
	if errors.Is(err, sql.ErrNoRows) {
		return ArtifactLocation{}, ErrNotFound
	}
	return loc, err
}

// GetArtifactPaths возвращает расположения debuginfo и executable.
func (s *Storage) GetArtifactPaths(buildID string) (debuginfo, executable ArtifactLocation, err error) {
	debuginfo, err = s.getOptionalLocation(buildID, "debuginfo")
	if err != nil {
		return ArtifactLocation{}, ArtifactLocation{}, err
	}
	executable, err = s.getOptionalLocation(buildID, "executable")
	if err != nil {
		return ArtifactLocation{}, ArtifactLocation{}, err
	}
	return debuginfo, executable, nil
}

func (s *Storage) getOptionalLocation(buildID, artifactType string) (ArtifactLocation, error) {
	loc, err := s.GetArtifactLocation(buildID, artifactType)
	if errors.Is(err, ErrNotFound) {
		return ArtifactLocation{}, nil
	}
	return loc, err
}

// NeedsScan возвращает true, если файл нужно переиндексировать.
func (s *Storage) NeedsScan(path string, mtimeNS, size int64) (bool, error) {
	var storedMtime, storedSize int64
	err := s.db.QueryRow(
		rebind(`SELECT mtime_ns, size FROM scanned_files WHERE path = ?`, s.dialect), path,
	).Scan(&storedMtime, &storedSize)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	return storedMtime != mtimeNS || storedSize != size, nil
}

// MarkScanned сохраняет метаданные успешно просканированного файла.
func (s *Storage) MarkScanned(path string, mtimeNS, size int64, kind string) error {
	_, err := s.db.Exec(rebind(`
		INSERT INTO scanned_files (path, mtime_ns, size, kind)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			mtime_ns = excluded.mtime_ns,
			size = excluded.size,
			kind = excluded.kind
	`, s.dialect), path, mtimeNS, size, kind)
	return err
}

// AddSource сохраняет или обновляет исходный файл, привязанный к build-id.
func (s *Storage) AddSource(buildID, sourcePath, filePath string, mtimeNS int64) error {
	return s.AddSourceLocation(SourceInput{
		BuildID:    buildID,
		SourcePath: sourcePath,
		FilePath:   filePath,
	}, mtimeNS)
}

// SourceInput — данные для сохранения исходника.
type SourceInput struct {
	BuildID     string
	SourcePath  string
	FilePath    string
	ArchivePath string
	MemberPath  string
}

// AddSourceLocation сохраняет исходник с поддержкой архивов.
func (s *Storage) AddSourceLocation(in SourceInput, mtimeNS int64) error {
	_, err := s.db.Exec(rebind(`
		INSERT INTO sources (build_id, source_path, file_path, archive_path, member_path, mtime_ns)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(build_id, source_path) DO UPDATE SET
			file_path = excluded.file_path,
			archive_path = excluded.archive_path,
			member_path = excluded.member_path,
			mtime_ns = excluded.mtime_ns
		WHERE excluded.mtime_ns >= sources.mtime_ns
	`, s.dialect), in.BuildID, in.SourcePath, in.FilePath, in.ArchivePath, in.MemberPath, mtimeNS)
	return err
}

// GetSource возвращает расположение исходника по build-id и пути из DWARF.
func (s *Storage) GetSource(buildID, sourcePath string) (SourceLocation, error) {
	loc, err := s.getSourceExact(buildID, sourcePath)
	if err == nil {
		return loc, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return SourceLocation{}, err
	}

	// Fallback: исходники из SRPM/DSC без привязки к build-id.
	loc, err = s.getSourceByPathSuffix(sourcePath)
	if err != nil {
		return SourceLocation{}, err
	}
	return loc, nil
}

func (s *Storage) getSourceExact(buildID, sourcePath string) (SourceLocation, error) {
	var loc SourceLocation
	var buildIDCol string
	err := s.db.QueryRow(
		rebind(`SELECT build_id, source_path, file_path, archive_path, member_path FROM sources WHERE build_id = ? AND source_path = ?`, s.dialect),
		buildID, sourcePath,
	).Scan(&buildIDCol, &loc.SourcePath, &loc.FilePath, &loc.ArchivePath, &loc.MemberPath)
	if errors.Is(err, sql.ErrNoRows) {
		return SourceLocation{}, ErrNotFound
	}
	return loc, err
}

func (s *Storage) getSourceByPathSuffix(sourcePath string) (SourceLocation, error) {
	base := filepath.Base(sourcePath)
	rows, err := s.db.Query(rebind(`
		SELECT source_path, file_path, archive_path, member_path
		FROM sources
		WHERE source_path = ? OR source_path LIKE ?
		ORDER BY length(source_path) DESC
		LIMIT 1
	`, s.dialect), sourcePath, "%"+base)
	if err != nil {
		return SourceLocation{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return SourceLocation{}, ErrNotFound
	}
	var loc SourceLocation
	if err := rows.Scan(&loc.SourcePath, &loc.FilePath, &loc.ArchivePath, &loc.MemberPath); err != nil {
		return SourceLocation{}, err
	}
	return loc, nil
}

// HasBuildID проверяет, известен ли серверу данный build-id.
func (s *Storage) HasBuildID(buildID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		rebind(`SELECT COUNT(1) FROM artifacts WHERE build_id = ?`, s.dialect),
		buildID,
	).Scan(&count)
	return count > 0, err
}

// SearchBuildIDForUI ищет артефакты по префиксу build-id для Web UI.
func (s *Storage) SearchBuildIDForUI(ctx context.Context, query string, limit int) ([]ArtifactRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(query), "0x"))

	var rows *sql.Rows
	var err error
	if query == "" {
		rows, err = s.db.QueryContext(ctx, rebind(`
			SELECT `+artifactSelectColumns+`
			FROM artifacts
			ORDER BY build_id
			LIMIT ?
		`, s.dialect), limit)
	} else {
		pattern := query + "%"
		rows, err = s.db.QueryContext(ctx, rebind(`
			SELECT `+artifactSelectColumns+`
			FROM artifacts
			WHERE build_id LIKE ? OR lower(raw_build_id) LIKE ?
			ORDER BY build_id
			LIMIT ?
		`, s.dialect), pattern, pattern, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ArtifactRecord
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		rec, err := scanArtifactRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	if results == nil {
		results = []ArtifactRecord{}
	}
	return results, rows.Err()
}

// SearchMetadata ищет артефакты по ключу debuginfod metadata API.
func (s *Storage) SearchMetadata(ctx context.Context, key, value string) (MetadataResponse, error) {
	return s.SearchMetadataQuery(ctx, MetadataQuery{Key: key, Value: value})
}

// SearchMetadataQuery ищет артефакты с поддержкой offset/limit.
func (s *Storage) SearchMetadataQuery(ctx context.Context, q MetadataQuery) (MetadataResponse, error) {
	switch q.Key {
	case "glob":
		return s.searchGlob(ctx, q)
	case "file":
		return s.searchFile(ctx, q)
	case "buildid":
		return s.searchBuildID(ctx, q)
	default:
		return MetadataResponse{}, fmt.Errorf("unsupported metadata key: %s", q.Key)
	}
}

func (s *Storage) searchGlob(ctx context.Context, q MetadataQuery) (MetadataResponse, error) {
	rows, err := s.db.QueryContext(ctx, rebind(`
		SELECT `+artifactSelectColumns+`
		FROM artifacts
		ORDER BY build_id, type
	`, s.dialect))
	if err != nil {
		return MetadataResponse{}, err
	}
	defer rows.Close()
	return collectMetadata(ctx, rows, func(rec ArtifactRecord) bool {
		return matchGlob(q.Value, rec.File) || (rec.Archive != "" && matchGlob(q.Value, rec.Archive))
	}, q.Offset, q.Limit)
}

func (s *Storage) searchFile(ctx context.Context, q MetadataQuery) (MetadataResponse, error) {
	rows, err := s.db.QueryContext(ctx, rebind(`
		SELECT `+artifactSelectColumns+`
		FROM artifacts
		ORDER BY build_id, type
	`, s.dialect))
	if err != nil {
		return MetadataResponse{}, err
	}
	defer rows.Close()
	path := filepath.ToSlash(q.Value)
	return collectMetadata(ctx, rows, func(rec ArtifactRecord) bool {
		return rec.File == path
	}, q.Offset, q.Limit)
}

func (s *Storage) searchBuildID(ctx context.Context, q MetadataQuery) (MetadataResponse, error) {
	rows, err := s.db.QueryContext(ctx, rebind(`
		SELECT `+artifactSelectColumns+`
		FROM artifacts
		ORDER BY build_id, type
	`, s.dialect))
	if err != nil {
		return MetadataResponse{}, err
	}
	defer rows.Close()
	query := strings.ToLower(strings.TrimPrefix(q.Value, "0x"))
	return collectMetadata(ctx, rows, func(rec ArtifactRecord) bool {
		return rec.BuildID == query || strings.EqualFold(rec.RawBuildID, query)
	}, q.Offset, q.Limit)
}

func collectMetadata(ctx context.Context, rows *sql.Rows, keep func(ArtifactRecord) bool, offset, limit int, enrich ...func(*ArtifactRecord)) (MetadataResponse, error) {
	var results []ArtifactRecord
	complete := true
	skipped := 0
	hasLimit := limit > 0
	var enrichFn func(*ArtifactRecord)
	if len(enrich) > 0 {
		enrichFn = enrich[0]
	}

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			complete = false
			break
		}
		rec, err := scanArtifactRow(rows)
		if err != nil {
			return MetadataResponse{}, err
		}
		if !keep(rec) {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		if hasLimit && len(results) >= limit {
			complete = false
			break
		}
		if enrichFn != nil {
			enrichFn(&rec)
		}
		results = append(results, rec)
	}
	if err := rows.Err(); err != nil {
		return MetadataResponse{}, err
	}
	if !hasLimit {
		// Проверяем, не осталось ли ещё строк после таймаута.
		if !complete && len(results) > 0 {
			return MetadataResponse{
				Results:    results,
				Complete:   false,
				NextOffset: offset + len(results),
			}, nil
		}
	} else if !complete {
		return MetadataResponse{
			Results:    results,
			Complete:   false,
			NextOffset: offset + len(results),
		}, nil
	}
	if results == nil {
		results = []ArtifactRecord{}
	}
	return MetadataResponse{Results: results, Complete: complete}, nil
}

func scanArtifactRow(rows *sql.Rows) (ArtifactRecord, error) {
	var row artifactRow
	err := rows.Scan(
		&row.BuildID, &row.FilePath, &row.Type, &row.ArchivePath, &row.MemberPath,
		&row.BuildIDKind, &row.RawBuildID, &row.GitCommit, &row.MtimeNs,
	)
	if err != nil {
		return ArtifactRecord{}, err
	}
	return row.toRecord(), nil
}

func (r artifactRow) toRecord() ArtifactRecord {
	rec := ArtifactRecord{
		BuildID:     r.BuildID,
		Type:        r.Type,
		BuildIDKind: r.BuildIDKind,
		RawBuildID:  r.RawBuildID,
		GitCommit:   r.GitCommit,
		MtimeNs:     r.MtimeNs,
	}
	if r.ArchivePath != "" {
		rec.Archive = filepath.ToSlash(r.ArchivePath)
		rec.ArchivePath = rec.Archive
		rec.MemberPath = filepath.ToSlash(r.MemberPath)
		rec.File = rec.MemberPath
		rec.FilePath = rec.ArchivePath
	} else {
		rec.FilePath = filepath.ToSlash(r.FilePath)
		rec.File = rec.FilePath
	}
	return rec
}

func matchGlob(pattern, value string) bool {
	return fnmatch.Match(pattern, value, fnmatch.Pathname)
}
