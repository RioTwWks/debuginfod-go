package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// ErrNotFound возвращается, когда запись не найдена в базе.
var ErrNotFound = errors.New("not found")

// ArtifactRecord описывает проиндексированный артефакт для metadata API.
type ArtifactRecord struct {
	BuildID     string `json:"buildid"`
	Type        string `json:"type"`
	File        string `json:"file"`
	Archive     string `json:"archive,omitempty"`
	BuildIDKind string `json:"buildid_kind,omitempty"`
	RawBuildID  string `json:"raw_buildid,omitempty"`
}

// MetadataResponse — JSON-ответ эндпоинта /metadata.
type MetadataResponse struct {
	Results  []ArtifactRecord `json:"results"`
	Complete bool             `json:"complete"`
}

// Storage — SQLite-хранилище метаданных debuginfod.
type Storage struct {
	db *sql.DB
}

// New открывает (или создаёт) базу данных и схему таблиц.
func New(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Storage{db: db}, nil
}

func migrate(db *sql.DB) error {
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
			mtime_ns INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (build_id, source_path)
		);
		CREATE INDEX IF NOT EXISTS idx_sources_build_id ON sources(build_id);
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}

	for _, stmt := range []string{
		"ALTER TABLE artifacts ADD COLUMN archive_path TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE artifacts ADD COLUMN member_path TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE artifacts ADD COLUMN build_id_kind TEXT NOT NULL DEFAULT 'gnu'",
		"ALTER TABLE artifacts ADD COLUMN raw_build_id TEXT NOT NULL DEFAULT ''",
	} {
		_, _ = db.Exec(stmt)
	}
	return nil
}

// Close закрывает соединение с базой данных.
func (s *Storage) Close() error {
	return s.db.Close()
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
}

type artifactRow struct {
	BuildID     string
	Type        string
	FilePath    string
	ArchivePath string
	MemberPath  string
	BuildIDKind string
	RawBuildID  string
}

// AddArtifact сохраняет или обновляет артефакт.
func (s *Storage) AddArtifact(in ArtifactInput, mtimeNS int64) error {
	_, err := s.db.Exec(`
		INSERT INTO artifacts (
			build_id, file_path, type, archive_path, member_path,
			build_id_kind, raw_build_id, mtime_ns
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(build_id, type) DO UPDATE SET
			file_path = excluded.file_path,
			archive_path = excluded.archive_path,
			member_path = excluded.member_path,
			build_id_kind = excluded.build_id_kind,
			raw_build_id = excluded.raw_build_id,
			mtime_ns = excluded.mtime_ns
		WHERE excluded.mtime_ns >= artifacts.mtime_ns
	`, in.BuildID, in.FilePath, in.Type, in.ArchivePath, in.MemberPath,
		in.BuildIDKind, in.RawBuildID, mtimeNS)
	return err
}

// GetArtifactPath возвращает путь на диске для отдачи файла клиенту.
func (s *Storage) GetArtifactPath(buildID, artifactType string) (string, error) {
	var filePath string
	err := s.db.QueryRow(
		`SELECT file_path FROM artifacts WHERE build_id = ? AND type = ?`,
		buildID, artifactType,
	).Scan(&filePath)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return filePath, err
}

// AddSource сохраняет или обновляет исходный файл, привязанный к build-id.
func (s *Storage) AddSource(buildID, sourcePath, filePath string, mtimeNS int64) error {
	_, err := s.db.Exec(`
		INSERT INTO sources (build_id, source_path, file_path, mtime_ns)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(build_id, source_path) DO UPDATE SET
			file_path = excluded.file_path,
			mtime_ns = excluded.mtime_ns
		WHERE excluded.mtime_ns >= sources.mtime_ns
	`, buildID, sourcePath, filePath, mtimeNS)
	return err
}

// GetSource возвращает путь к исходнику по build-id и пути из DWARF.
func (s *Storage) GetSource(buildID, sourcePath string) (string, error) {
	var filePath string
	err := s.db.QueryRow(
		`SELECT file_path FROM sources WHERE build_id = ? AND source_path = ?`,
		buildID, sourcePath,
	).Scan(&filePath)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return filePath, err
}

// HasBuildID проверяет, известен ли серверу данный build-id.
func (s *Storage) HasBuildID(buildID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(1) FROM artifacts WHERE build_id = ?`,
		buildID,
	).Scan(&count)
	return count > 0, err
}

// SearchMetadata ищет артефакты по ключу debuginfod metadata API.
func (s *Storage) SearchMetadata(key, value string) (MetadataResponse, error) {
	switch key {
	case "glob":
		return s.searchGlob(value)
	case "file":
		return s.searchFile(value)
	case "buildid":
		return s.searchBuildID(value)
	default:
		return MetadataResponse{}, fmt.Errorf("unsupported metadata key: %s", key)
	}
}

func (s *Storage) searchGlob(pattern string) (MetadataResponse, error) {
	rows, err := s.db.Query(`
		SELECT build_id, file_path, type, archive_path, member_path, build_id_kind, raw_build_id
		FROM artifacts
	`)
	if err != nil {
		return MetadataResponse{}, err
	}
	defer rows.Close()
	return collectMetadata(rows, func(rec ArtifactRecord) bool {
		return matchGlob(pattern, rec.File) || (rec.Archive != "" && matchGlob(pattern, rec.Archive))
	})
}

func (s *Storage) searchFile(path string) (MetadataResponse, error) {
	rows, err := s.db.Query(`
		SELECT build_id, file_path, type, archive_path, member_path, build_id_kind, raw_build_id
		FROM artifacts
	`)
	if err != nil {
		return MetadataResponse{}, err
	}
	defer rows.Close()
	path = filepath.ToSlash(path)
	return collectMetadata(rows, func(rec ArtifactRecord) bool {
		return rec.File == path
	})
}

func (s *Storage) searchBuildID(query string) (MetadataResponse, error) {
	rows, err := s.db.Query(`
		SELECT build_id, file_path, type, archive_path, member_path, build_id_kind, raw_build_id
		FROM artifacts
	`)
	if err != nil {
		return MetadataResponse{}, err
	}
	defer rows.Close()
	query = strings.ToLower(strings.TrimPrefix(query, "0x"))
	return collectMetadata(rows, func(rec ArtifactRecord) bool {
		return rec.BuildID == query || strings.EqualFold(rec.RawBuildID, query)
	})
}

func collectMetadata(rows *sql.Rows, keep func(ArtifactRecord) bool) (MetadataResponse, error) {
	var results []ArtifactRecord
	for rows.Next() {
		rec, err := scanArtifactRow(rows)
		if err != nil {
			return MetadataResponse{}, err
		}
		if keep(rec) {
			results = append(results, rec)
		}
	}
	if results == nil {
		results = []ArtifactRecord{}
	}
	return MetadataResponse{Results: results, Complete: true}, rows.Err()
}

func scanArtifactRow(rows *sql.Rows) (ArtifactRecord, error) {
	var row artifactRow
	err := rows.Scan(
		&row.BuildID, &row.FilePath, &row.Type, &row.ArchivePath, &row.MemberPath,
		&row.BuildIDKind, &row.RawBuildID,
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
	}
	if r.ArchivePath != "" {
		rec.Archive = filepath.ToSlash(r.ArchivePath)
		rec.File = filepath.ToSlash(r.MemberPath)
	} else {
		rec.File = filepath.ToSlash(r.FilePath)
	}
	return rec
}

func matchGlob(pattern, value string) bool {
	pattern = filepath.ToSlash(pattern)
	value = filepath.ToSlash(value)
	ok, err := filepath.Match(pattern, value)
	if err == nil && ok {
		return true
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(value, parts[0]) && strings.HasSuffix(value, parts[1])
		}
	}
	return false
}
