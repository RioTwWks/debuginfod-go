#!/bin/sh
# Инициализация и запуск PostgreSQL (пакет Debian/Astra, без образа postgres:* с Hub).
set -e

: "${POSTGRES_USER:=debuginfod}"
: "${POSTGRES_PASSWORD:=debuginfod}"
: "${POSTGRES_DB:=debuginfod}"
: "${PGDATA:=/var/lib/postgresql/data}"

pg_path() {
	_bin=$(command -v initdb 2>/dev/null) && dirname "$_bin" && return 0
	_dir=$(find /usr/lib/postgresql -maxdepth 2 -type d -name bin 2>/dev/null | head -1)
	[ -n "$_dir" ] && printf '%s' "$_dir" && return 0
	return 1
}

run_as_postgres() {
	su - postgres -s /bin/sh -c "$1"
}

if [ "$1" = 'postgres' ] || [ $# -eq 0 ]; then
	PGBIN=$(pg_path) || {
		echo "postgresql binaries not found" >&2
		exit 1
	}
	export PATH="$PGBIN:$PATH"

	if [ ! -s "$PGDATA/PG_VERSION" ]; then
		echo "Initializing PostgreSQL in $PGDATA"
		mkdir -p "$PGDATA"
		chown -R postgres:postgres "$(dirname "$PGDATA")"
		chown postgres:postgres "$PGDATA"

		run_as_postgres "initdb -D '$PGDATA' -E UTF8 --locale=C"
		echo "listen_addresses = '*'" >>"$PGDATA/postgresql.conf"
		echo "host all all all md5" >>"$PGDATA/pg_hba.conf"

		run_as_postgres "pg_ctl -D '$PGDATA' -w start"
		run_as_postgres "psql -v ON_ERROR_STOP=1 --username postgres -c \"CREATE USER ${POSTGRES_USER} WITH PASSWORD '${POSTGRES_PASSWORD}' CREATEDB;\""
		run_as_postgres "psql -v ON_ERROR_STOP=1 --username postgres -c \"CREATE DATABASE ${POSTGRES_DB} OWNER ${POSTGRES_USER};\""
		run_as_postgres "pg_ctl -D '$PGDATA' -m fast -w stop"
	fi

	exec su - postgres -s /bin/sh -c "exec postgres -D '$PGDATA'"
fi

exec "$@"
