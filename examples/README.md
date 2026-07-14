# Примеры debuginfod-go

Демонстрация работы сервера с **GDB** и **docker compose**.

Сценарий: собирается stripped `hello` и отдельный `hello.debug` (как в production с `/usr/lib/debug`). GDB загружает символы с `debuginfod-go` по `DEBUGINFOD_URLS`.

## Быстрый старт (Docker)

```bash
cd examples
make demo
```

Команда:

1. Собирает `sample/bin/hello` + `sample/bin/hello.debug`
2. Поднимает `debuginfod` (порт `8002`)
3. Ждёт индексации и запускает GDB в batch-режиме

Проверка API вручную:

```bash
make up
make health
make metadata
curl http://localhost:8002/ui/api/stats
```

Остановка:

```bash
make down
```

## Локально (без Docker)

Терминал 1 — сервер:

```bash
make -C examples/sample
DEBUGINFOD_SCAN_PATH=$(pwd)/examples/sample/bin \
	DEBUGINFOD_PORT=8002 \
	./debuginfod -s examples/sample/bin -p 8002
```

Терминал 2 — GDB:

```bash
export DEBUGINFOD_URLS=http://localhost:8002
gdb -x examples/gdb/debug.gdb examples/sample/bin/hello
```

## Структура

| Путь | Назначение |
|------|------------|
| `sample/hello.c` | Исходник демо-программы |
| `sample/Makefile` | `gcc -g` → `objcopy` → `strip` |
| `gdb/debug.gdb` | GDB init: breakpoint, backtrace, locals |
| `gdb/run-demo.sh` | Ожидание индексации + запуск GDB |
| `docker-compose.yml` | `debuginfod` + опциональный `gdb` client |

## docker-compose

Сервис `debuginfod` собирается из корневого `Dockerfile` и индексирует `./sample/bin`.

Сервис `gdb` (profile `gdb`) — Ubuntu 22.04 с `gdb` и `elfutils`, подключается к `http://debuginfod:8002`.

```bash
docker compose up -d debuginfod          # только сервер
docker compose --profile gdb run --rm gdb  # одноразовый GDB-клиент
```

## debuginfod-find

После `make up`:

```bash
export DEBUGINFOD_URLS=http://localhost:8002
../debuginfod-find debuginfo "$(readelf -n sample/bin/hello | awk '/Build ID/ {print $3}')"
```

Или metadata:

```bash
../debuginfod-find --key file --value /sample/hello
```

## Требования

- Docker и docker compose (для `make demo`)
- `gcc`, `binutils` (`objcopy`, `strip`) — для сборки sample
- `gdb` + поддержка debuginfod в GDB (пакет `elfutils` на Ubuntu/Debian)
