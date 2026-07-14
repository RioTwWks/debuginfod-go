#!/usr/bin/env bash
# Скачивает .deb пакет debuginfod-go и все runtime-зависимости для оффлайн-установки.
# Запускать на машине с интернетом (Ubuntu / Astra Linux).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
POOL="${POOL:-$ROOT/dist/offline/deb/pool}"
MANIFEST="$ROOT/deploy/offline/deps-deb.txt"
VERSION="${VERSION:-$(cd "$ROOT" && git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo 0.1.0)}"

cd "$ROOT"

if ! command -v apt-get >/dev/null 2>&1; then
	echo "apt-get не найден — скрипт для Debian/Ubuntu/Astra" >&2
	exit 1
fi

echo "==> Сборка пакета debuginfod-go ${VERSION}"
make package-deb VERSION="$VERSION"

mkdir -p "$POOL"
shopt -s nullglob
rm -f "$POOL"/*.deb

echo "==> Копирование пакета debuginfod-go"
cp "$ROOT"/dist/debuginfod-go_"${VERSION}"_amd64.deb "$POOL/" 2>/dev/null || \
	cp "$ROOT"/dist/debuginfod-go_*_amd64.deb "$POOL/"

echo "==> Скачивание зависимостей"
if [ "$(id -u)" -eq 0 ]; then
	apt-get update -qq 2>/dev/null || true
fi
mapfile -t SEEDS < <(grep -v '^\s*#' "$MANIFEST" | grep -v '^\s*$')
DEPS="$(apt-cache depends --recurse --no-recommends --no-suggests \
	--no-conflicts --no-breaks --no-replaces --no-enhances \
	"${SEEDS[@]}" 2>/dev/null | awk '/^[[:alnum:]]/ {print $1}' | sort -u)"

(
	cd "$POOL"
	for pkg in $DEPS; do
		apt-get download "$pkg" 2>/dev/null || echo "  пропуск: $pkg (нет в репозитории)" >&2
	done
)

COUNT="$(find "$POOL" -name '*.deb' | wc -l)"
echo "==> Готово: $COUNT .deb в $POOL"
echo "    Создайте bundle: make offline-bundle-deb VERSION=$VERSION"
