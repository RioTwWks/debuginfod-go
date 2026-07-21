package storage

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func openPostgres(url string) (*Storage, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	if err := migratePostgres(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Storage{db: db, dialect: DialectPostgres}, nil
}

func migratePostgres(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS artifacts (
			build_id TEXT NOT NULL,
			file_path TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			archive_path TEXT NOT NULL DEFAULT '',
			member_path TEXT NOT NULL DEFAULT '',
			build_id_kind TEXT NOT NULL DEFAULT 'gnu',
			raw_build_id TEXT NOT NULL DEFAULT '',
			mtime_ns BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (build_id, type)
		);
		CREATE INDEX IF NOT EXISTS idx_artifacts_build_id ON artifacts(build_id);

		CREATE TABLE IF NOT EXISTS sources (
			build_id TEXT NOT NULL,
			source_path TEXT NOT NULL,
			file_path TEXT NOT NULL,
			archive_path TEXT NOT NULL DEFAULT '',
			member_path TEXT NOT NULL DEFAULT '',
			mtime_ns BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (build_id, source_path)
		);
		CREATE INDEX IF NOT EXISTS idx_sources_build_id ON sources(build_id);

		CREATE TABLE IF NOT EXISTS scanned_files (
			path TEXT PRIMARY KEY,
			mtime_ns BIGINT NOT NULL,
			size BIGINT NOT NULL,
			kind TEXT NOT NULL
		);
	`
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate postgres: %w", err)
	}
	for _, stmt := range []string{
		"ALTER TABLE sources ADD COLUMN IF NOT EXISTS archive_path TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE sources ADD COLUMN IF NOT EXISTS member_path TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE artifacts ADD COLUMN IF NOT EXISTS git_commit TEXT NOT NULL DEFAULT ''",
	} {
		_, _ = db.Exec(stmt)
	}
	if err := migrateDedup(db, DialectPostgres); err != nil {
		return err
	}
	return migrateHistory(db, DialectPostgres)
}
