#!/usr/bin/env bash
# Миграция индекса SQLite → PostgreSQL (без полного rescan).
# Использование:
#   sudo systemctl stop debuginfod-go
#   sudo deploy/postgresql/migrate-sqlite-to-postgres.sh \
#     /var/lib/debuginfod-go/debuginfod.sqlite \
#     'postgres://debuginfod:secret@localhost:5432/debuginfod'
#   # обновить DEBUGINFOD_DATABASE_URL в .env, убрать/закомментировать DEBUGINFOD_DB_PATH
#   sudo systemctl start debuginfod-go
set -euo pipefail

SQLITE="${1:-}"
PG_URL="${2:-}"

if [ -z "$SQLITE" ] || [ -z "$PG_URL" ]; then
	echo "Использование: $0 <sqlite-path> <postgres-url>" >&2
	exit 1
fi

if [ ! -f "$SQLITE" ]; then
	echo "SQLite не найден: $SQLITE" >&2
	exit 1
fi

for cmd in sqlite3 psql; do
	command -v "$cmd" >/dev/null || { echo "$cmd не найден" >&2; exit 1; }
done

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

log() { echo "[migrate] $*"; }

log "Создание схемы в PostgreSQL (если нет)..."
psql "$PG_URL" -v ON_ERROR_STOP=1 <<'SQL'
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
SQL

copy_table() {
	local table="$1"
	log "Копирование ${table}..."
	sqlite3 -header -csv "$SQLITE" "SELECT * FROM ${table};" > "${WORKDIR}/${table}.csv"
	local count
	count="$(wc -l < "${WORKDIR}/${table}.csv")"
	if [ "$count" -le 1 ]; then
		log "  ${table}: пусто"
		return 0
	fi
	psql "$PG_URL" -v ON_ERROR_STOP=1 -c "TRUNCATE ${table};"
	psql "$PG_URL" -v ON_ERROR_STOP=1 -c "\\copy ${table} FROM '${WORKDIR}/${table}.csv' CSV HEADER"
	log "  ${table}: $((count - 1)) строк"
}

copy_table artifacts
copy_table sources
copy_table scanned_files

log "Готово. Обновите /etc/debuginfod-go/debuginfod-go.env:"
echo "  DEBUGINFOD_DATABASE_URL=${PG_URL}"
echo "  # DEBUGINFOD_DB_PATH=...  (закомментируйте SQLite)"
