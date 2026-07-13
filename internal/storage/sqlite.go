package storage

import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)

type Storage struct {
    db *sql.DB
}

func New(dbPath string) (*Storage, error) {
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, err
    }
    // Создаем таблицу, если её нет
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS artifacts (
            build_id TEXT NOT NULL,
            file_path TEXT NOT NULL,
            type TEXT,
            PRIMARY KEY (build_id, file_path)
        );
        CREATE INDEX IF NOT EXISTS idx_build_id ON artifacts(build_id);
    `)
    if err != nil {
        return nil, err
    }
    return &Storage{db: db}, nil
}

func (s *Storage) AddArtifact(buildID, filePath, artifactType string) error {
    _, err := s.db.Exec(
        "INSERT OR REPLACE INTO artifacts (build_id, file_path, type) VALUES (?, ?, ?)",
        buildID, filePath, artifactType,
    )
    return err
}

func (s *Storage) GetArtifact(buildID, artifactType string) (string, error) {
    var filePath string
    err := s.db.QueryRow(
        "SELECT file_path FROM artifacts WHERE build_id = ? AND type = ? LIMIT 1",
        buildID, artifactType,
    ).Scan(&filePath)
    if err == sql.ErrNoRows {
        return "", nil // Не найдено
    }
    return filePath, err
}
