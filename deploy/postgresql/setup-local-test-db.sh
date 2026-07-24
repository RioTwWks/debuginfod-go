#!/usr/bin/env bash
# Создаёт локального пользователя/БД debuginfod для тестов без Docker.
# Запуск: sudo deploy/postgresql/setup-local-test-db.sh
set -euo pipefail

DB_USER="${POSTGRES_TEST_USER:-debuginfod}"
DB_PASS="${POSTGRES_TEST_PASSWORD:-debuginfod}"
DB_NAME="${POSTGRES_TEST_DB:-debuginfod}"

if [ "$(id -u)" -ne 0 ]; then
	echo "Run as root: sudo $0" >&2
	exit 1
fi

if ! command -v psql >/dev/null 2>&1; then
	echo "Install PostgreSQL client/server first, e.g.: apt install postgresql" >&2
	exit 1
fi

run_psql() {
	if id postgres >/dev/null 2>&1; then
		su - postgres -c "psql -v ON_ERROR_STOP=1 -c \"$1\""
	else
		psql -v ON_ERROR_STOP=1 -c "$1"
	fi
}

if ! run_psql "SELECT 1" >/dev/null 2>&1; then
	echo "PostgreSQL is not running. Start it: systemctl start postgresql" >&2
	exit 1
fi

run_psql "DO \$\$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '$DB_USER') THEN
    CREATE ROLE $DB_USER LOGIN PASSWORD '$DB_PASS';
  ELSE
    ALTER ROLE $DB_USER WITH LOGIN PASSWORD '$DB_PASS';
  END IF;
END \$\$;"

run_psql "SELECT 1 FROM pg_database WHERE datname = '$DB_NAME'" | grep -q 1 \
	|| run_psql "CREATE DATABASE $DB_NAME OWNER $DB_USER"

run_psql "GRANT ALL PRIVILEGES ON DATABASE $DB_NAME TO $DB_USER"

cat <<EOF
Ready.

DEBUGINFOD_TEST_DATABASE_URL=postgres://${DB_USER}:${DB_PASS}@127.0.0.1:5432/${DB_NAME}?sslmode=disable
DEBUGINFOD_DATABASE_URL=postgres://${DB_USER}:${DB_PASS}@127.0.0.1:5432/${DB_NAME}?sslmode=disable

Run tests:
  DEBUGINFOD_TEST_DATABASE_URL=postgres://${DB_USER}:${DB_PASS}@127.0.0.1:5432/${DB_NAME}?sslmode=disable \\
    go test -tags=integration -v ./internal/storage -run Postgres
EOF
