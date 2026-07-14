#!/usr/bin/env bash
# Ожидает debuginfod и индексации Go-бинарника, проверяет debuginfod-find, запускает Delve.
set -euo pipefail

DEBUGINFOD_URL="${DEBUGINFOD_URLS:-http://debuginfod:8002}"
BINARY="${1:-/sample-go/hello}"
METADATA_VALUE="${BINARY}"

echo "Waiting for debuginfod at ${DEBUGINFOD_URL}..."
until curl -sf "${DEBUGINFOD_URL}/healthz" >/dev/null; do
	sleep 1
done

echo "Waiting for ${METADATA_VALUE} in metadata index..."
until curl -sf "${DEBUGINFOD_URL}/metadata?key=file&value=${METADATA_VALUE}" | grep -q '"buildid"'; do
	sleep 1
done

export DEBUGINFOD_URLS="${DEBUGINFOD_URL}"

BUILD_ID=""
if readelf -n "${BINARY}" 2>/dev/null | grep -q 'Build ID'; then
	BUILD_ID=$(readelf -n "${BINARY}" | awk '/Build ID/ {print $3; exit}')
	echo "GNU build-id: ${BUILD_ID}"
else
	RAW=$(go tool buildid "${BINARY}")
	BUILD_ID=$(printf '%s' "${RAW}" | sha256sum | awk '{print $1}')
	echo "Go canonical build-id: ${BUILD_ID}"
fi

echo "Verifying debuginfod-find debuginfo ${BUILD_ID}..."
debuginfod-find debuginfo "${BUILD_ID}" >/dev/null
echo "debuginfod-find OK"

echo "=== Delve demo (batch) ==="
printf 'break main.greet\ncontinue\nprint name\nprint answer\nbt\nquit\n' | \
	dlv exec "${BINARY}" debuginfod-user --allow-non-terminal-interactive=true

echo "=== Demo complete ==="
