#!/usr/bin/env bash
# Поднимает PostgreSQL для тестов (docker-compose.postgres.yml).
# Учитывает корпоративный прокси и внутренние зеркала образов.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

# shellcheck disable=SC1091
source "$ROOT/deploy/docker/ensure-proxy-env.sh"

COMPOSE_FILE="${POSTGRES_COMPOSE_FILE:-docker-compose.postgres.yml}"
IMAGE="${POSTGRES_IMAGE:-postgres:16-alpine}"

export POSTGRES_IMAGE="$IMAGE"

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
	echo "PostgreSQL image not found locally: $IMAGE"
	if [ -n "${HTTP_PROXY:-}" ]; then
		echo "Note: docker pull uses the Docker daemon proxy, not shell HTTP_PROXY."
		echo "Configure /etc/systemd/system/docker.service.d/http-proxy.conf — see deploy/docker/README.md"
	fi
	echo "Pulling (requires registry access or daemon proxy)..."
	if ! docker pull "$IMAGE"; then
		cat <<EOF

Failed to pull $IMAGE.

Options:
  1) Docker daemon proxy (recommended for corporate networks):
     sudo mkdir -p /etc/systemd/system/docker.service.d
     sudo tee /etc/systemd/system/docker.service.d/http-proxy.conf <<'PROXY'
[Service]
Environment="HTTP_PROXY=http://proxy.corp:3128"
Environment="HTTPS_PROXY=http://proxy.corp:3128"
Environment="NO_PROXY=localhost,127.0.0.1"
PROXY
     sudo systemctl daemon-reload && sudo systemctl restart docker

  2) Internal registry mirror:
     export POSTGRES_IMAGE=registry.corp.local/library/postgres:16-alpine
     deploy/postgresql/docker-up.sh

  3) Offline (on a host with internet):
     docker pull postgres:16-alpine && docker save postgres:16-alpine -o postgres-16-alpine.tar
     # on Astra: docker load -i postgres-16-alpine.tar

  4) Skip Docker — use system PostgreSQL:
     sudo apt install postgresql
     deploy/postgresql/setup-local-test-db.sh
     export DEBUGINFOD_TEST_DATABASE_URL=postgres://debuginfod:debuginfod@127.0.0.1:5432/debuginfod?sslmode=disable
     go test -tags=integration -v ./internal/storage -run Postgres

See deploy/postgresql/README.md
EOF
		exit 1
	fi
fi

exec docker compose -f "$COMPOSE_FILE" up -d --wait "$@"
