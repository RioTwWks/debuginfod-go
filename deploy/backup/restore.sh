#!/usr/bin/env bash
# Восстановление debuginfod-go из каталога backup.
# Использование: sudo deploy/backup/restore.sh /var/backups/debuginfod-go/20260714-120000
set -euo pipefail

SRC="${1:-}"
ENV_FILE="${DEBUGINFOD_ENV_FILE:-/etc/debuginfod-go/debuginfod-go.env}"

if [ "$(id -u)" -ne 0 ]; then
	echo "Запустите от root: sudo $0 <backup-dir>" >&2
	exit 1
fi

if [ -z "$SRC" ] || [ ! -d "$SRC" ]; then
	echo "Укажите каталог backup: sudo $0 /var/backups/debuginfod-go/YYYYMMDD-HHMMSS" >&2
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

log() { echo "[restore] $*"; }

systemctl stop debuginfod-go.service 2>/dev/null || true

if [ -f "${SRC}/debuginfod.pgdump" ]; then
	if [ -z "$DATABASE_URL" ]; then
		echo "В backup PostgreSQL dump, но DEBUGINFOD_DATABASE_URL не задан в ${ENV_FILE}" >&2
		exit 1
	fi
	if ! command -v pg_restore >/dev/null 2>&1; then
		echo "pg_restore не найден" >&2
		exit 1
	fi
	log "Восстановление PostgreSQL (clean)..."
	pg_restore --clean --if-exists --dbname="$DATABASE_URL" "${SRC}/debuginfod.pgdump" || true
elif [ -f "${SRC}/debuginfod.sqlite" ]; then
	mkdir -p "$(dirname "$DB_PATH")"
	install -m 0644 -o debuginfod -g debuginfod "${SRC}/debuginfod.sqlite" "$DB_PATH"
	log "Восстановлен SQLite: $DB_PATH"
else
	log "Файл БД в backup не найден"
fi

if [ -f "${SRC}/debuginfod-go.env" ]; then
	install -m 0640 -o root -g debuginfod "${SRC}/debuginfod-go.env" "$ENV_FILE"
	log "Восстановлен конфиг: $ENV_FILE"
fi

if [ -f "${SRC}/cache.tar.gz" ]; then
	mkdir -p "$(dirname "$CACHE_DIR")"
	tar -C "$(dirname "$CACHE_DIR")" -xzf "${SRC}/cache.tar.gz"
	chown -R debuginfod:debuginfod "$CACHE_DIR" 2>/dev/null || true
	log "Восстановлен cache: $CACHE_DIR"
fi

systemctl start debuginfod-go.service 2>/dev/null || true
log "Готово. Проверка: curl http://127.0.0.1:${DEBUGINFOD_PORT:-8002}/healthz"
