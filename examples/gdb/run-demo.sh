#!/usr/bin/env bash
# Ожидает готовности debuginfod и индексации бинарника, затем запускает GDB.
set -euo pipefail

DEBUGINFOD_URL="${DEBUGINFOD_URLS:-http://debuginfod:8002}"
BINARY="${1:-/sample/hello}"
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
exec gdb -batch -x /gdb/debug.gdb "${BINARY}"
