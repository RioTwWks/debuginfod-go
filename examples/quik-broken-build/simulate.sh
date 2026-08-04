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

# Адрес символа из ELF (локаль GDB/Russian не важны — ищем 0x…).
symbol_addr() {
	local elf=$1 sym=$2
	local addr
	addr=$("$GDB" -batch -nx \
		-ex "file $elf" \
		-ex "info address $sym" \
		2>/dev/null | grep -oE '0x[0-9a-f]+' | head -1)
	if [[ -n "$addr" ]]; then
		echo "$addr"
		return 0
	fi
	if command -v nm >/dev/null 2>&1; then
		addr=$(nm -D "$elf" 2>/dev/null | awk '/GetCode/ {printf "0x%s\n", $1; exit}')
		if [[ -n "$addr" ]]; then
			echo "$addr"
			return 0
		fi
	fi
	return 1
}

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
echo "--- 2. Скачивание debuginfo (curl / debuginfod-find) ---"
curl -sf "${DEBUGINFOD_URL}/buildid/${BUILDID}/debuginfo" -o "$FULL"
echo "получено: $(wc -c <"$FULL") байт → $FULL"

objcopy --decompress-debug-sections "$FULL"
cp "$FULL" "$UNPACKED"
echo "objcopy --decompress-debug-sections (обязательно для Quik zlib DWARF)"

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
echo "--- 4. «Лог краша»: адрес PC ---"
ADDR=$(symbol_addr "$UNPACKED" 'quik::Class::GetCode' || true)
if [[ -z "$ADDR" ]]; then
	ADDR=$(symbol_addr "$UNPACKED" 'quik::Exception::what' || true)
fi
if [[ -z "$ADDR" ]]; then
	echo "не удалось получить адрес — проверьте $UNPACKED" >&2
	exit 1
fi
echo "Симулированный PC: $ADDR"

echo
echo "--- 5. БЕЗ символов: только stripped .so ---"
"$GDB" -batch -nx \
	-ex "file $STRIPPED" \
	-ex "info symbol $ADDR" \
	2>/dev/null | sed '/^$/d' || true

echo
echo "--- 6. С debuginfo с сервера (распакованный, как после curl+objcopy) ---"
echo "GDB 10.1 не читает zlib DWARF из ~/.cache/debuginfod_client без objcopy."
"$GDB" -batch -nx \
	-ex "file $STRIPPED" \
	-ex "symbol-file $UNPACKED" \
	-ex "info symbol $ADDR" \
	-ex "info functions quik::Class::" \
	2>/dev/null | sed '/^$/d' | head -40

echo
echo "--- 7. libdebuginfod: кэш + objcopy на клиенте ---"
CACHE="${HOME}/.cache/debuginfod_client/${BUILDID}/debuginfo"
rm -rf "${HOME}/.cache/debuginfod_client/${BUILDID}"
export DEBUGINFOD_URLS="$DEBUGINFOD_URL"
"$GDB" -batch -nx \
	-ex "set debuginfod enabled on" \
	-ex "file $STRIPPED" \
	2>/dev/null | sed '/^$/d' || true
if [[ -f "$CACHE" ]]; then
	objcopy --decompress-debug-sections "$CACHE"
	echo "кэш распакован: $CACHE"
	"$GDB" -batch -nx \
		-ex "set debuginfod enabled on" \
		-ex "file $STRIPPED" \
		-ex "info symbol $ADDR" \
		2>/dev/null | sed '/^$/d' || true
else
	echo "кэш не создан (libdebuginfod не скачал debuginfo)"
fi

echo
echo "--- 8. debuginfod-find (опционально) ---"
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
echo "Ручной GDB (надёжный путь для Quik на GDB 10.1):"
echo "  $GDB $STRIPPED -ex \"symbol-file $UNPACKED\""
