# Симуляция: разработчик и сломанная сборка Quik

Сценарий: на машине разработчика есть только **stripped** runtime-библиотека из проблемной сборки (как в дистрибутиве Quik). Отладочные `.debug` на диске нет. Символы берутся с **debuginfod-go** по build-id.

Реальные Quik `.debug` часто содержат **zlib-сжатые** DWARF-секции; GDB читает их только после распаковки. Сервер отдаёт файл как на `SCAN_PATH`; клиент при необходимости делает `objcopy --decompress-debug-sections` (скрипт делает это в кэше).

## Что делает скрипт

1. Скачивает `debuginfo` с сервера по build-id (или берёт локальный `.debug`).
2. Создаёт копию «как у разработчика» — `strip --strip-debug`, build-id сохраняется.
3. Берёт адрес функции из полного debuginfo (как «PC из лога краша»).
4. Показывает GDB **без** и **с** `DEBUGINFOD_URLS`: разрешение адреса и список символов `quik::`.

## Быстрый запуск (ваш libcore 16.0.0.1)

```bash
# Сервер уже сканирует /home/ieme/debug_linux-go и отвечает readyz
export DEBUGINFOD_URLS=http://127.0.0.1:8002
export BUILDID=296f131912a8e43b35aa294f7d08b91c1b76014a

./examples/quik-broken-build/simulate.sh
```

Другой артефакт из индекса:

```bash
export BUILDID=$(curl -s "$DEBUGINFOD_URLS/metadata?key=glob&value=*/libcore.so.16.0.0.10.debug" \
  | jq -r '.results[0].buildid')
./examples/quik-broken-build/simulate.sh
```

## Ручной сценарий (без скрипта)

```bash
export DEBUGINFOD_URLS=http://127.0.0.1:8002
BUILDID=296f131912a8e43b35aa294f7d08b91c1b76014a
WORKDIR=/tmp/quik-dev-$(id -u)
mkdir -p "$WORKDIR"

# 1. «Получили» только stripped .so (симуляция: strip из эталонного debuginfo)
curl -sf "$DEBUGINFOD_URLS/buildid/$BUILDID/debuginfo" -o "$WORKDIR/libcore.full"
objcopy --decompress-debug-sections "$WORKDIR/libcore.full"
cp "$WORKDIR/libcore.full" "$WORKDIR/libcore.so"
strip --strip-debug "$WORKDIR/libcore.so"

readelf --notes "$WORKDIR/libcore.so" | grep -E 'Build ID|ID сборки'

# 2. Адрес из «лога» (взяли из полного debuginfo)
ADDR=$(gdb -batch -nx \
  -ex "file $WORKDIR/libcore.full" \
  -ex "info address quik::Class::GetCode" \
  2>/dev/null | awk '/is at/ {print $2; exit}')
echo "Симулированный PC из лога: $ADDR"

# 3. Разработчик БЕЗ debuginfod — символ неизвестен
gdb -batch -nx -ex "file $WORKDIR/libcore.so" -ex "info symbol $ADDR"

# 4. Разработчик С debuginfod-go
export DEBUGINFOD_URLS
gdb -batch -nx \
  -ex "set debuginfod enabled on" \
  -ex "file $WORKDIR/libcore.so" \
  -ex "info symbol $ADDR" \
  -ex "info functions quik::Class::"
```

Ожидание: в шаге 3 — `no symbol` или только offset; в шаге 4 — имя `quik::Class::GetCode` и список методов.

## Ограничения этой VM

- В `debug_linux-go` нет runtime `libcore.so` без `.debug` — скрипт **синтезирует** stripped копию.
- На сервере Quik stripped `.so` уже есть в установке; там не нужен `strip`, только `DEBUGINFOD_URLS` и GDB attach / анализ core.
- Запуск `quik` или `dlopen(libcore)` здесь обычно невозможен без полного окружения Quik — сценарий фокусируется на **символах и адресах из лога**, как при разборе инцидента.

## См. также

- [docs/TESTING.md](../../docs/TESTING.md) — общая проверка ELF/HTTP/GDB
- [examples/gdb/debug.gdb](../gdb/debug.gdb) — демо с `hello` (запуск + backtrace)
