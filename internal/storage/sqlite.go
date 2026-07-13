package storage

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// ErrNotFound возвращается, когда запись не найдена в базе.
var ErrNotFound = errors.New("not found")

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
			file_path TEXT NOT NULL,
			type TEXT NOT NULL,
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
	return nil
}

// Close закрывает соединение с базой данных.
func (s *Storage) Close() error {
	return s.db.Close()
}

// AddArtifact сохраняет или обновляет артефакт (executable/debuginfo).
func (s *Storage) AddArtifact(buildID, filePath, artifactType string, mtimeNS int64) error {
	_, err := s.db.Exec(`
		INSERT INTO artifacts (build_id, file_path, type, mtime_ns)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(build_id, type) DO UPDATE SET
			file_path = excluded.file_path,
			mtime_ns = excluded.mtime_ns
		WHERE excluded.mtime_ns >= artifacts.mtime_ns
	`, buildID, filePath, artifactType, mtimeNS)
	return err
}

// GetArtifact возвращает путь к артефакту по build-id и типу.
func (s *Storage) GetArtifact(buildID, artifactType string) (string, error) {
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
