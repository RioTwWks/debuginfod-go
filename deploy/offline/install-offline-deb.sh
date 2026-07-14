#!/usr/bin/env bash
# Оффлайн-установка debuginfod-go из локального каталога .deb (без интернета).
# Использование: sudo ./install-offline-deb.sh [каталог_с_deb]
set -euo pipefail

POOL="${1:-$(cd "$(dirname "$0")/../../dist/offline/deb/pool" && pwd)}"

if [ "$(id -u)" -ne 0 ]; then
	echo "Запустите от root: sudo $0" >&2
	exit 1
fi

if ! ls "$POOL"/*.deb >/dev/null 2>&1; then
	echo "Нет .deb в $POOL" >&2
	exit 1
fi

echo "==> Локальный apt-репозиторий: $POOL"
cd "$POOL"
dpkg-scanpackages . /dev/null 2>/dev/null | gzip -9c > Packages.gz

LIST="/etc/apt/sources.list.d/debuginfod-go-offline.list"
echo "deb [trusted=yes] file:$POOL ./" > "$LIST"

echo "==> Установка (без сетевых репозиториев)"
apt-get update -o Dir::Etc::sourcelist="sources.list.d/debuginfod-go-offline.list" \
	-o Dir::Etc::sourceparts="-" -o APT::Get::List-Cleanup="0"
DEBIAN_FRONTEND=noninteractive apt-get install -y \
	-o Dir::Etc::sourcelist="sources.list.d/debuginfod-go-offline.list" \
	-o Dir::Etc::sourceparts="-" \
	-o APT::Get::AllowUnauthenticated="true" \
	debuginfod-go

echo "==> Запуск сервиса"
systemctl enable --now debuginfod-go.service
systemctl status debuginfod-go.service --no-pager || true
echo "==> Готово. Проверка: curl http://localhost:8002/healthz"
