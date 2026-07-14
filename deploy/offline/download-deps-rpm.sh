#!/usr/bin/env bash
# Скачивает .rpm пакет debuginfod-go и все runtime-зависимости для оффлайн-установки.
# Запускать на машине с интернетом (RedOS / CentOS).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
POOL="${POOL:-$ROOT/dist/offline/rpm/pool}"
MANIFEST="$ROOT/deploy/offline/deps-rpm.txt"
VERSION="${VERSION:-$(cd "$ROOT" && git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo 0.1.0)}"

cd "$ROOT"

if ! command -v dnf >/dev/null 2>&1 && ! command -v yum >/dev/null 2>&1; then
	echo "dnf/yum не найден — скрипт для RedOS/CentOS" >&2
	exit 1
fi

PKG_MGR="dnf"
command -v dnf >/dev/null 2>&1 || PKG_MGR="yum"

echo "==> Сборка пакета debuginfod-go ${VERSION}"
make package-rpm VERSION="$VERSION"

mkdir -p "$POOL"
rm -f "$POOL"/*.rpm

echo "==> Копирование пакета debuginfod-go"
cp "$ROOT"/dist/debuginfod-go-"${VERSION}"-1.x86_64.rpm "$POOL/" 2>/dev/null || \
	cp "$ROOT"/dist/debuginfod-go-*-1.x86_64.rpm "$POOL/"

echo "==> Скачивание зависимостей"
mapfile -t SEEDS < <(grep -v '^\s*#' "$MANIFEST" | grep -v '^\s*$')
for pkg in "${SEEDS[@]}"; do
	$PKG_MGR download --destdir="$POOL" "$pkg" || echo "  пропуск: $pkg" >&2
done

# Транзитивные зависимости для скачанных RPM
if command -v dnf >/dev/null 2>&1; then
	dnf repoquery --requires --resolve --arch x86_64 --destdir="$POOL" \
		$(grep -v '^\s*#' "$MANIFEST" | grep -v '^\s*$' | tr '\n' ' ') 2>/dev/null || true
fi

COUNT="$(find "$POOL" -name '*.rpm' | wc -l)"
echo "==> Готово: $COUNT .rpm в $POOL"
echo "    Создайте bundle: make offline-bundle-rpm VERSION=$VERSION"
