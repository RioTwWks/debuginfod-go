package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// ScanRunRecord — история прохода индексатора.
type ScanRunRecord struct {
	ID             int64     `json:"id"`
	FinishedAt     time.Time `json:"finished_at"`
	DurationMs     int64     `json:"duration_ms"`
	Indexed        int       `json:"indexed"`
	Skipped        int       `json:"skipped"`
	Errors         int       `json:"errors"`
	ArtifactsTotal int64     `json:"artifacts_total"`
	ScannedFiles   int64     `json:"scanned_files"`
	BytesOnDisk    int64     `json:"bytes_on_disk"`
}

// DedupRunRecord — история dedup ingest/backfill.
type DedupRunRecord struct {
	ID                 int64     `json:"id"`
	FinishedAt         time.Time `json:"finished_at"`
	DurationMs         int64     `json:"duration_ms"`
	Project            string    `json:"project,omitempty"`
	DryRun             bool      `json:"dry_run"`
	BuildDirsProcessed int       `json:"build_dirs_processed"`
	FilesRegistered    int       `json:"files_registered"`
	FilesCompressed    int       `json:"files_compressed"`
	FilesDedupRef      int       `json:"files_dedup_ref"`
	FilesSkipped       int       `json:"files_skipped"`
	Errors             int       `json:"errors"`
	BytesBefore        int64     `json:"bytes_before"`
	BytesAfter         int64     `json:"bytes_after"`
	BytesSaved         int64     `json:"bytes_saved"`
	SavedPercent       float64   `json:"saved_percent"`
}

func historySchemaSQLite() string {
	return `
		CREATE TABLE IF NOT EXISTS scan_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			finished_at INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			indexed INTEGER NOT NULL,
			skipped INTEGER NOT NULL,
			errors INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_scan_runs_finished ON scan_runs(finished_at DESC);

		CREATE TABLE IF NOT EXISTS dedup_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			finished_at INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			dry_run INTEGER NOT NULL DEFAULT 0,
			build_dirs_processed INTEGER NOT NULL DEFAULT 0,
			files_registered INTEGER NOT NULL DEFAULT 0,
			files_compressed INTEGER NOT NULL DEFAULT 0,
			files_skipped INTEGER NOT NULL DEFAULT 0,
			errors INTEGER NOT NULL DEFAULT 0,
			bytes_before INTEGER NOT NULL DEFAULT 0,
			bytes_after INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_dedup_runs_finished ON dedup_runs(finished_at DESC);
	`
}

func historySchemaPostgres() string {
	return `
		CREATE TABLE IF NOT EXISTS scan_runs (
			id BIGSERIAL PRIMARY KEY,
			finished_at BIGINT NOT NULL,
			duration_ms BIGINT NOT NULL,
			indexed INTEGER NOT NULL,
			skipped INTEGER NOT NULL,
			errors INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_scan_runs_finished ON scan_runs(finished_at DESC);

		CREATE TABLE IF NOT EXISTS dedup_runs (
			id BIGSERIAL PRIMARY KEY,
			finished_at BIGINT NOT NULL,
			duration_ms BIGINT NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			dry_run BOOLEAN NOT NULL DEFAULT FALSE,
			build_dirs_processed BIGINT NOT NULL DEFAULT 0,
			files_registered BIGINT NOT NULL DEFAULT 0,
			files_compressed BIGINT NOT NULL DEFAULT 0,
			files_skipped BIGINT NOT NULL DEFAULT 0,
			errors BIGINT NOT NULL DEFAULT 0,
			bytes_before BIGINT NOT NULL DEFAULT 0,
			bytes_after BIGINT NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_dedup_runs_finished ON dedup_runs(finished_at DESC);
	`
}

func migrateHistory(db *sql.DB, dialect Dialect) error {
	schema := historySchemaSQLite()
	if dialect == DialectPostgres {
		schema = historySchemaPostgres()
	}
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate history: %w", err)
	}
	for _, stmt := range scanRunsMigrations(dialect) {
		_, _ = db.Exec(stmt)
	}
	for _, stmt := range dedupRunsMigrations(dialect) {
		_, _ = db.Exec(stmt)
	}
	return nil
}

func dedupRunsMigrations(dialect Dialect) []string {
	if dialect == DialectPostgres {
		return []string{
			"ALTER TABLE dedup_runs ADD COLUMN IF NOT EXISTS files_dedup_ref BIGINT NOT NULL DEFAULT 0",
		}
	}
	return []string{
		"ALTER TABLE dedup_runs ADD COLUMN files_dedup_ref INTEGER NOT NULL DEFAULT 0",
	}
}

func scanRunsMigrations(dialect Dialect) []string {
	if dialect == DialectPostgres {
		return []string{
			"ALTER TABLE scan_runs ADD COLUMN IF NOT EXISTS artifacts_total BIGINT NOT NULL DEFAULT 0",
			"ALTER TABLE scan_runs ADD COLUMN IF NOT EXISTS scanned_files BIGINT NOT NULL DEFAULT 0",
			"ALTER TABLE scan_runs ADD COLUMN IF NOT EXISTS bytes_on_disk BIGINT NOT NULL DEFAULT 0",
		}
	}
	return []string{
		"ALTER TABLE scan_runs ADD COLUMN artifacts_total INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE scan_runs ADD COLUMN scanned_files INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE scan_runs ADD COLUMN bytes_on_disk INTEGER NOT NULL DEFAULT 0",
	}
}

// InsertScanRun сохраняет результат индексации.
func (s *Storage) InsertScanRun(rec ScanRunRecord) error {
	_, err := s.db.Exec(rebind(`
		INSERT INTO scan_runs (
			finished_at, duration_ms, indexed, skipped, errors,
			artifacts_total, scanned_files, bytes_on_disk
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, s.dialect),
		rec.FinishedAt.Unix(), rec.DurationMs, rec.Indexed, rec.Skipped, rec.Errors,
		rec.ArtifactsTotal, rec.ScannedFiles, rec.BytesOnDisk,
	)
	return err
}

// InsertDedupRun сохраняет результат dedup.
func (s *Storage) InsertDedupRun(rec DedupRunRecord) error {
	dry := 0
	if rec.DryRun {
		dry = 1
	}
	_, err := s.db.Exec(rebind(`
		INSERT INTO dedup_runs (
			finished_at, duration_ms, project, dry_run,
			build_dirs_processed, files_registered, files_compressed,
			files_dedup_ref, files_skipped, errors, bytes_before, bytes_after
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.dialect),
		rec.FinishedAt.Unix(), rec.DurationMs, rec.Project, dry,
		rec.BuildDirsProcessed, rec.FilesRegistered, rec.FilesCompressed,
		rec.FilesDedupRef, rec.FilesSkipped, rec.Errors, rec.BytesBefore, rec.BytesAfter,
	)
	return err
}

// ListScanRuns возвращает последние проходы индексатора.
func (s *Storage) ListScanRuns(limit int) ([]ScanRunRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(rebind(`
		SELECT id, finished_at, duration_ms, indexed, skipped, errors,
			artifacts_total, scanned_files, bytes_on_disk
		FROM scan_runs ORDER BY finished_at DESC LIMIT ?
	`, s.dialect), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScanRuns(rows)
}

// ListDedupRuns возвращает последние dedup-прогоны.
func (s *Storage) ListDedupRuns(limit int) ([]DedupRunRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(rebind(`
		SELECT id, finished_at, duration_ms, project, dry_run,
			build_dirs_processed, files_registered, files_compressed,
			files_dedup_ref, files_skipped, errors, bytes_before, bytes_after
		FROM dedup_runs ORDER BY finished_at DESC LIMIT ?
	`, s.dialect), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDedupRuns(rows)
}

// DedupStorageTotals — суммарная экономия dedup по всем обработанным файлам.
type DedupStorageTotals struct {
	FilesDone       int64   `json:"files_done"`
	FilesCompressed int64   `json:"files_compressed"`
	FilesCASRef       int64   `json:"files_cas_ref"`
	BytesOriginal   int64   `json:"bytes_original"`
	BytesOnDisk     int64   `json:"bytes_on_disk"`
	BytesSaved      int64   `json:"bytes_saved"`
	SavedPercent    float64 `json:"saved_percent"`
}

// DedupStorageTotals вычисляет текущую экономию хранения.
func (s *Storage) DedupStorageTotals() (DedupStorageTotals, error) {
	var out DedupStorageTotals
	_ = s.db.QueryRow(rebind(`SELECT COUNT(1) FROM dedup_files WHERE status = 'done'`, s.dialect)).Scan(&out.FilesDone)
	_ = s.db.QueryRow(rebind(`SELECT COUNT(1) FROM dedup_files WHERE storage_kind = 'compressed'`, s.dialect)).Scan(&out.FilesCompressed)
	_ = s.db.QueryRow(rebind(`SELECT COUNT(1) FROM dedup_files WHERE storage_kind = 'ref'`, s.dialect)).Scan(&out.FilesCASRef)
	_ = s.db.QueryRow(rebind(`SELECT COALESCE(SUM(original_size), 0) FROM dedup_files WHERE status = 'done'`, s.dialect)).Scan(&out.BytesOriginal)

	// Уникальные blob по SHA256 (без двойного подсчёта ref).
	_ = s.db.QueryRow(rebind(`
		SELECT COALESCE(SUM(blob_size), 0) FROM (
			SELECT sha256, MAX(compressed_size) AS blob_size
			FROM dedup_files
			WHERE status = 'done' AND storage_kind IN ('compressed', 'ref') AND sha256 != ''
			GROUP BY sha256
		) t
	`, s.dialect)).Scan(&out.BytesOnDisk)

	// Несжатые full-файлы (pending pipeline или legacy).
	var fullBytes int64
	_ = s.db.QueryRow(rebind(`
		SELECT COALESCE(SUM(original_size), 0) FROM dedup_files
		WHERE status = 'done' AND storage_kind = 'full'
	`, s.dialect)).Scan(&fullBytes)
	out.BytesOnDisk += fullBytes

	if out.BytesOriginal > 0 && out.BytesOnDisk < out.BytesOriginal {
		out.BytesSaved = out.BytesOriginal - out.BytesOnDisk
		out.SavedPercent = float64(out.BytesSaved) / float64(out.BytesOriginal) * 100
	}
	return out, nil
}

func scanScanRuns(rows *sql.Rows) ([]ScanRunRecord, error) {
	var out []ScanRunRecord
	for rows.Next() {
		var r ScanRunRecord
		var finished int64
		if err := rows.Scan(
			&r.ID, &finished, &r.DurationMs, &r.Indexed, &r.Skipped, &r.Errors,
			&r.ArtifactsTotal, &r.ScannedFiles, &r.BytesOnDisk,
		); err != nil {
			return nil, err
		}
		r.FinishedAt = time.Unix(finished, 0)
		out = append(out, r)
	}
	if out == nil {
		out = []ScanRunRecord{}
	}
	return out, rows.Err()
}

func scanDedupRuns(rows *sql.Rows) ([]DedupRunRecord, error) {
	var out []DedupRunRecord
	for rows.Next() {
		var r DedupRunRecord
		var finished int64
		var dry int
		if err := rows.Scan(
			&r.ID, &finished, &r.DurationMs, &r.Project, &dry,
			&r.BuildDirsProcessed, &r.FilesRegistered, &r.FilesCompressed,
			&r.FilesDedupRef, &r.FilesSkipped, &r.Errors, &r.BytesBefore, &r.BytesAfter,
		); err != nil {
			return nil, err
		}
		r.FinishedAt = time.Unix(finished, 0)
		r.DryRun = dry != 0
		if r.BytesBefore > 0 && r.BytesAfter < r.BytesBefore {
			r.BytesSaved = r.BytesBefore - r.BytesAfter
			r.SavedPercent = float64(r.BytesSaved) / float64(r.BytesBefore) * 100
		}
		out = append(out, r)
	}
	if out == nil {
		out = []DedupRunRecord{}
	}
	return out, rows.Err()
}

func fileSize(path string) (int64, error) {
	if path == "" {
		return 0, fmt.Errorf("empty path")
	}
	st, err := sqlOpenStat(path)
	return st, err
}

// sqlOpenStat обёртка для os.Stat без импорта os в тестах — реализуется через os.
func sqlOpenStat(path string) (int64, error) {
	return statFileSize(path)
}
