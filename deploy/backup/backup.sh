#!/usr/bin/env bash
# Резервное копирование debuginfod-go: БД, конфиг, опционально cache.
# Использование: sudo deploy/backup/backup.sh
set -euo pipefail

ENV_FILE="${DEBUGINFOD_ENV_FILE:-/etc/debuginfod-go/debuginfod-go.env}"
BACKUP_ROOT="${DEBUGINFOD_BACKUP_DIR:-/var/backups/debuginfod-go}"
KEEP_DAYS="${DEBUGINFOD_BACKUP_KEEP_DAYS:-14}"
BACKUP_CACHE="${DEBUGINFOD_BACKUP_CACHE:-false}"
RESTIC_REPO="${RESTIC_REPOSITORY:-}"
RESTIC_PASSWORD="${RESTIC_PASSWORD:-}"

if [ "$(id -u)" -ne 0 ]; then
	echo "Запустите от root: sudo $0" >&2
	exit 1
fi

if [ -f "$ENV_FILE" ]; then
	set -a
	# shellcheck disable=SC1090
	. "$ENV_FILE"
	set +a
fi

DB_PATH="${DEBUGINFOD_DB_PATH:-/var/lib/debuginfod-go/debuginfod.sqlite}"
DATABASE_URL="${DEBUGINFOD_DATABASE_URL:-}"
CACHE_DIR="${DEBUGINFOD_CACHE_DIR:-/var/cache/debuginfod-go}"
TS="$(date -u +%Y%m%d-%H%M%S)"
DEST="${BACKUP_ROOT}/${TS}"
mkdir -p "$DEST"

log() { echo "[backup] $*"; }

backup_sqlite() {
	local src="$1"
	if [ ! -f "$src" ]; then
		log "SQLite не найден: $src (пропуск)"
		return 0
	fi
	if command -v sqlite3 >/dev/null 2>&1; then
		sqlite3 "$src" ".backup '${DEST}/debuginfod.sqlite'"
		log "SQLite backup: ${DEST}/debuginfod.sqlite"
	else
		cp -a "$src" "${DEST}/debuginfod.sqlite"
		log "SQLite copy (sqlite3 недоступен): ${DEST}/debuginfod.sqlite"
	fi
}

backup_postgres() {
	local url="$1"
	if ! command -v pg_dump >/dev/null 2>&1; then
		echo "pg_dump не найден" >&2
		exit 1
	fi
	pg_dump --format=custom --file="${DEST}/debuginfod.pgdump" "$url"
	log "PostgreSQL dump: ${DEST}/debuginfod.pgdump"
}

backup_config() {
	if [ -f "$ENV_FILE" ]; then
		install -m 0640 -o root -g debuginfod "$ENV_FILE" "${DEST}/debuginfod-go.env"
		log "Config: ${DEST}/debuginfod-go.env"
	fi
}

backup_cache() {
	if [ "$BACKUP_CACHE" != "true" ]; then
		return 0
	fi
	if [ -d "$CACHE_DIR" ]; then
		tar -C "$(dirname "$CACHE_DIR")" -czf "${DEST}/cache.tar.gz" "$(basename "$CACHE_DIR")"
		log "Cache archive: ${DEST}/cache.tar.gz"
	fi
}

write_manifest() {
	cat > "${DEST}/manifest.txt" <<EOF
timestamp=${TS}
hostname=$(hostname -f 2>/dev/null || hostname)
env_file=${ENV_FILE}
db_backend=$([ -n "$DATABASE_URL" ] && echo postgres || echo sqlite)
db_path=${DB_PATH}
cache_dir=${CACHE_DIR}
backup_cache=${BACKUP_CACHE}
EOF
}

prune_old() {
	find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -mtime "+${KEEP_DAYS}" -exec rm -rf {} + 2>/dev/null || true
	log "Удалены каталоги старше ${KEEP_DAYS} дней в ${BACKUP_ROOT}"
}

restic_upload() {
	if [ -z "$RESTIC_REPO" ]; then
		return 0
	fi
	if ! command -v restic >/dev/null 2>&1; then
		log "restic не установлен (пропуск offsite)"
		return 0
	fi
	export RESTIC_REPOSITORY="$RESTIC_REPO"
	[ -n "$RESTIC_PASSWORD" ] && export RESTIC_PASSWORD
	restic backup "$DEST" --tag debuginfod-go --host "$(hostname -s)"
	log "Restic: загружен ${DEST}"
}

log "Начало backup → ${DEST}"

if [ -n "$DATABASE_URL" ]; then
	backup_postgres "$DATABASE_URL"
else
	backup_sqlite "$DB_PATH"
fi

backup_config
backup_cache
write_manifest
restic_upload
prune_old

log "Готово: ${DEST}"
