#!/usr/bin/env bash
# Упаковывает pool/ в tar.gz для переноса на изолированный хост.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FAMILY="${1:-deb}"
VERSION="${VERSION:-$(cd "$ROOT" && git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo 0.1.0)}"
OUT_DIR="$ROOT/dist/offline"
STAGING="$OUT_DIR/debuginfod-go-offline-${FAMILY}-${VERSION}"

case "$FAMILY" in
deb)
	POOL="$OUT_DIR/deb/pool"
	INSTALLER="install-offline-deb.sh"
	;;
rpm)
	POOL="$OUT_DIR/rpm/pool"
	INSTALLER="install-offline-rpm.sh"
	;;
*)
	echo "Использование: $0 deb|rpm" >&2
	exit 1
	;;
esac

if [ ! -d "$POOL" ] || [ -z "$(ls -A "$POOL" 2>/dev/null)" ]; then
	echo "Пустой pool: $POOL — сначала make offline-download-${FAMILY}" >&2
	exit 1
fi

rm -rf "$STAGING"
mkdir -p "$STAGING/pool"
cp -a "$POOL/." "$STAGING/pool/"
cp "$ROOT/deploy/offline/$INSTALLER" "$STAGING/install-offline.sh"
cp "$ROOT/deploy/offline/README.md" "$STAGING/"

ARCHIVE="$OUT_DIR/debuginfod-go-offline-${FAMILY}-${VERSION}.tar.gz"
tar -C "$OUT_DIR" -czf "$ARCHIVE" "$(basename "$STAGING")"
echo "==> Bundle: $ARCHIVE"
echo "    На целевом хосте:"
echo "    tar xf $(basename "$ARCHIVE") && cd $(basename "$STAGING") && sudo ./install-offline.sh"
