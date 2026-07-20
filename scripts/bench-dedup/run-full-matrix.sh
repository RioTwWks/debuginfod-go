#!/usr/bin/env bash
# Полная матрица Strategy A/B: все algo × preprocess + objcopy + (опц.) group-by modes.
#
# Использование:
#   export SCAN_PATH=/home/ieme/debug_linux
#   export PROJECT="Released/QuikServer_16.0_Common_Linux"
#   ./scripts/bench-dedup/run-full-matrix.sh
#
# Результат: $WORKDIR/matrix.{json,csv,txt}
# Время: ~30–90 мин (bsdiff медленный; hdiffpatch пропускается если нет в PATH).

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

SCAN_PATH="${SCAN_PATH:?укажите SCAN_PATH}"
PROJECT="${PROJECT:-}"
WORKDIR="${WORKDIR:-/tmp/bench-dedup-matrix-$(date +%Y%m%d-%H%M)}"
EXTENDED="${EXTENDED:-0}"

make -s build-bench-dedup

echo "=== Tools ==="
./bench-dedup check-tools
echo

echo "=== Collect preview ==="
./bench-dedup list-groups --scan-path "$SCAN_PATH" ${PROJECT:+--project "$PROJECT"} --min-files 2
echo

mkdir -p "$WORKDIR"

ARGS=(
  run-matrix
  --scan-path "$SCAN_PATH"
  --workdir "$WORKDIR"
  --output "$WORKDIR/matrix"
)
[[ -n "$PROJECT" ]] && ARGS+=(--project "$PROJECT")
[[ "$EXTENDED" == "1" ]] && ARGS+=(--extended)

echo "=== Full matrix ($(date -Iseconds)) ==="
echo "WORKDIR=$WORKDIR"
echo "EXTENDED=$EXTENDED"
echo

./bench-dedup "${ARGS[@]}"

echo
echo "=== Done ==="
echo "Reports:"
ls -la "$WORKDIR"/matrix.* 2>/dev/null || true
echo
echo "Summary (без jq):"
grep -E 'savings_pct|verify_failures|"id"' "$WORKDIR/matrix.json" | head -40 || true
echo
echo "Python summary:"
python3 - <<'PY' "$WORKDIR/matrix.json"
import json, sys
data = json.load(open(sys.argv[1]))
print(f"{'ID':<40} {'savings%':>8} {'verify':>6} {'errors':>6}  skipped")
print("-" * 72)
for r in sorted(data["rows"], key=lambda x: -x.get("savings_pct", 0)):
    sk = r.get("skipped") or ""
    print(f"{r['id']:<40} {r.get('savings_pct', 0):8.1f} {r.get('verify_failures', 0):6} {r.get('error_count', 0):6}  {sk}")
PY
