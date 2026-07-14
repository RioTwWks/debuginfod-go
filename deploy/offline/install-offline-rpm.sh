#!/usr/bin/env bash
# Оффлайн-установка debuginfod-go из локального каталога .rpm (без интернета).
# Использование: sudo ./install-offline-rpm.sh [каталог_с_rpm]
set -euo pipefail

POOL="${1:-$(cd "$(dirname "$0")/../../dist/offline/rpm/pool" && pwd)}"

if [ "$(id -u)" -ne 0 ]; then
	echo "Запустите от root: sudo $0" >&2
	exit 1
fi

if ! ls "$POOL"/*.rpm >/dev/null 2>&1; then
	echo "Нет .rpm в $POOL" >&2
	exit 1
fi

PKG_MGR="dnf"
command -v dnf >/dev/null 2>&1 || PKG_MGR="yum"

echo "==> Установка из $POOL (без сетевых репозиториев)"
$PKG_MGR install -y --disablerepo='*' --allowerasing "$POOL"/*.rpm

echo "==> Запуск сервиса"
systemctl enable --now debuginfod-go.service
systemctl status debuginfod-go.service --no-pager || true
echo "==> Готово. Проверка: curl http://localhost:8002/healthz"
