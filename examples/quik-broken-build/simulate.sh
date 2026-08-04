#!/usr/bin/env bash
# Симуляция: разработчик получил stripped libcore из сломанной сборки Quik
# и разбирает адрес из лога через debuginfod-go (без локальных .debug).
set -euo pipefail

DEBUGINFOD_URL="${DEBUGINFOD_URLS:-http://127.0.0.1:8002}"
BUILDID="${BUILDID:-296f131912a8e43b35aa294f7d08b91c1b76014a}"
WORKDIR="${WORKDIR:-/tmp/quik-broken-build-$$}"
GDB="${GDB:-gdb}"

need() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "не найдено в PATH: $1" >&2
		exit 1
	}
}

need curl
need objcopy
need strip
need readelf
need "$GDB"

mkdir -p "$WORKDIR"
FULL="$WORKDIR/libcore.full.debug"
STRIPPED="$WORKDIR/libcore.so"
UNPACKED="$WORKDIR/libcore.unpacked.debug"

echo "=== Симуляция: сломанная сборка Quik + debuginfod-go ==="
echo "DEBUGINFOD_URLS=$DEBUGINFOD_URL"
echo "BUILDID=$BUILDID"
echo "WORKDIR=$WORKDIR"
echo

echo "--- 1. Проверка сервера ---"
curl -sf "${DEBUGINFOD_URL}/healthz" >/dev/null || {
	echo "healthz failed — сервер не доступен на $DEBUGINFOD_URL" >&2
	exit 1
}
code=$(curl -s -o /dev/null -w '%{http_code}' "${DEBUGINFOD_URL}/readyz")
echo "readyz: HTTP $code (200 = индекс готов)"

echo
echo "--- 2. Скачивание debuginfo (как делает libdebuginfod) ---"
curl -sf "${DEBUGINFOD_URL}/buildid/${BUILDID}/debuginfo" -o "$FULL"
echo "получено: $(wc -c <"$FULL") байт → $FULL"

objcopy --decompress-debug-sections "$FULL"
cp "$FULL" "$UNPACKED"
echo "распакованы DWARF-секции → $UNPACKED"

echo
echo "--- 3. «Сломанная сборка»: только stripped runtime на диске разработчика ---"
cp "$UNPACKED" "$STRIPPED"
strip --strip-debug "$STRIPPED"
echo "stripped: $STRIPPED ($(wc -c <"$STRIPPED") байт)"

bid=$(readelf --notes "$STRIPPED" | awk '/Build ID|ID сборки/ {print $NF; exit}')
if [[ "$bid" != "$BUILDID" ]]; then
	echo "WARN: build-id stripped=$bid ожидали $BUILDID" >&2
else
	echo "build-id совпадает: $bid"
fi

echo
echo "--- 4. «Лог краша»: адрес PC (взяли из полного debuginfo) ---"
ADDR=$("$GDB" -batch -nx \
	-ex "file $UNPACKED" \
	-ex "info address quik::Class::GetCode" \
	2>/dev/null | awk '/is at/ {print $2; exit}')
if [[ -z "$ADDR" ]]; then
	ADDR=$("$GDB" -batch -nx \
		-ex "file $UNPACKED" \
		-ex "info address quik::Exception::what" \
		2>/dev/null | awk '/is at/ {print $2; exit}')
fi
if [[ -z "$ADDR" ]]; then
	echo "не удалось получить адрес quik::Class::GetCode — проверьте символы в $UNPACKED" >&2
	exit 1
fi
echo "Симулированный PC: $ADDR"

echo
echo "--- 5. БЕЗ debuginfod: разработчик не видит символ ---"
"$GDB" -batch -nx \
	-ex "file $STRIPPED" \
	-ex "info symbol $ADDR" \
	2>/dev/null | sed '/^$/d' || true

echo
echo "--- 6. С DEBUGINFOD_URLS: запрос символов с сервера ---"
export DEBUGINFOD_URLS="$DEBUGINFOD_URL"
"$GDB" -batch -nx \
	-ex "set debuginfod enabled on" \
	-ex "file $STRIPPED" \
	-ex "info symbol $ADDR" \
	-ex "info functions quik::Class::" \
	2>/dev/null | sed '/^$/d' | head -40

echo
echo "--- 7. debuginfod-find (опционально) ---"
if command -v debuginfod-find >/dev/null 2>&1; then
	out="$WORKDIR/from-find.debug"
	DEBUGINFOD_URLS="$DEBUGINFOD_URL" debuginfod-find debuginfo "$BUILDID" -o "$out"
	objcopy --decompress-debug-sections "$out" 2>/dev/null || true
	echo "debuginfod-find → $out ($(wc -c <"$out") байт)"
else
	echo "debuginfod-find не в PATH (make build-find); шаг пропущен"
fi

echo
echo "=== Готово ==="
echo "Рабочая директория: $WORKDIR"
echo "Для ручного GDB: export DEBUGINFOD_URLS=$DEBUGINFOD_URL"
echo "  $GDB $STRIPPED"
