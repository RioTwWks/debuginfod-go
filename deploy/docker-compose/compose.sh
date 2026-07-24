#!/bin/sh
# Wrapper: docker compose (v2) или docker-compose (v1). Запускать из этой папки.
#   cd deploy/docker-compose
#   cp .env.example .env
#   ./compose.sh -f docker-compose.postgres.yml up -d
set -e

DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
cd "$DIR"

if [ -f .env ]; then
	set -a
	# shellcheck disable=SC1091
	. ./.env
	set +a
fi

if ! docker info >/dev/null 2>&1; then
	echo "Нет доступа к Docker daemon." >&2
	echo "  sudo usermod -aG docker \"\$USER\" && newgrp docker" >&2
	exit 1
fi

if docker compose version >/dev/null 2>&1; then
	exec docker compose "$@"
fi

if command -v docker-compose >/dev/null 2>&1; then
	exec docker-compose "$@"
fi

echo "Docker Compose not found." >&2
exit 1
