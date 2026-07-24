#!/bin/sh
# docker compose с подхватом корпоративного прокси (как PVS-Studio-Tracker compose.sh).
set -e

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$ROOT"

# shellcheck disable=SC1091
. "$ROOT/deploy/docker/ensure-proxy-env.sh"

if ! docker info >/dev/null 2>&1; then
	echo "Нет доступа к Docker daemon (permission denied)." >&2
	echo "  sudo usermod -aG docker \"\$USER\" && newgrp docker" >&2
	echo "  или: sudo docker compose ..." >&2
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
