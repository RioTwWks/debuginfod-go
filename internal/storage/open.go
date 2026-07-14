package storage

import (
	"fmt"
	"strings"
)

// Dialect определяет SQL-диалект backend.
type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

// Open открывает SQLite (по пути) или PostgreSQL (по URL).
func Open(dbPath, databaseURL string) (*Storage, error) {
	if databaseURL != "" {
		return openPostgres(databaseURL)
	}
	return openSQLite(dbPath)
}

func rebind(query string, dialect Dialect) string {
	if dialect != DialectPostgres {
		return query
	}
	var b strings.Builder
	n := 1
	for _, ch := range query {
		if ch == '?' {
			b.WriteString(fmt.Sprintf("$%d", n))
			n++
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// Stats — агрегированные счётчики для мониторинга.
type Stats struct {
	ArtifactsTotal      int64
	ArtifactsExecutable int64
	ArtifactsDebuginfo  int64
	SourcesTotal        int64
	ScannedFilesTotal   int64
}

// Stats возвращает счётчики из БД.
func (s *Storage) Stats() (Stats, error) {
	var out Stats
	if err := s.db.QueryRow(rebind(`SELECT COUNT(1) FROM artifacts`, s.dialect)).Scan(&out.ArtifactsTotal); err != nil {
		return out, err
	}
	_ = s.db.QueryRow(rebind(`SELECT COUNT(1) FROM artifacts WHERE type = 'executable'`, s.dialect)).Scan(&out.ArtifactsExecutable)
	_ = s.db.QueryRow(rebind(`SELECT COUNT(1) FROM artifacts WHERE type = 'debuginfo'`, s.dialect)).Scan(&out.ArtifactsDebuginfo)
	_ = s.db.QueryRow(rebind(`SELECT COUNT(1) FROM sources`, s.dialect)).Scan(&out.SourcesTotal)
	_ = s.db.QueryRow(rebind(`SELECT COUNT(1) FROM scanned_files`, s.dialect)).Scan(&out.ScannedFilesTotal)
	return out, nil
}
