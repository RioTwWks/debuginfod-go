# Примеры debuginfod-go

Демонстрация работы сервера с **GDB**, **Delve** и **docker compose**.

Сценарии:

- **GDB** — stripped C `hello` + отдельный `hello.debug`
- **Delve** — stripped Go `hello` + `hello.debug` (GNU build-id через external linker)

Оба клиента загружают символы с `debuginfod-go` по `DEBUGINFOD_URLS`.

## Быстрый старт (Docker)

### GDB

```bash
cd examples
make demo
```

### Delve

```bash
cd examples
make demo-delve
```

Команды:

1. Собирают sample-бинарник (C или Go)
2. Поднимают `debuginfod` (порт `8002`)
3. Ждут индексации и запускают отладчик в batch-режиме

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
make -C examples/delve/sample
DEBUGINFOD_SCAN_PATH=$(pwd)/examples/sample/bin,$(pwd)/examples/delve/sample/bin \
	./debuginfod -p 8002
```

Терминал 2 — GDB:

```bash
export DEBUGINFOD_URLS=http://localhost:8002
gdb -x examples/gdb/debug.gdb examples/sample/bin/hello
```

Терминал 2 — Delve:

```bash
export DEBUGINFOD_URLS=http://localhost:8002
bash examples/delve/run-demo.sh examples/delve/sample/bin/hello
```

Подробнее о Go build-id и Delve: [docs/GO_ECOSYSTEM.md](../docs/GO_ECOSYSTEM.md).

## Структура

| Путь | Назначение |
|------|------------|
| `sample/hello.c` | Исходник демо-программы |
| `sample/Makefile` | `gcc -g` → `objcopy` → `strip` |
| `gdb/debug.gdb` | GDB init: breakpoint, backtrace, locals |
| `gdb/run-demo.sh` | Ожидание индексации + запуск GDB |
| `delve/sample/main.go` | Go-программа для Delve-демо |
| `delve/run-demo.sh` | Ожидание индексации + batch Delve |
| `docker-compose.yml` | `debuginfod` + опциональные `gdb` / `delve` |

## docker-compose

Сервис `debuginfod` собирается из корневого `Dockerfile` и индексирует `./sample/bin`.

Сервис `gdb` (profile `gdb`) — Ubuntu 22.04 с `gdb` и `elfutils`, подключается к `http://debuginfod:8002`.

```bash
docker compose up -d debuginfod          # только сервер
docker compose --profile gdb run --rm gdb  # одноразовый GDB-клиент
docker compose --profile delve run --rm delve  # Delve + Go sample
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
- `dlv` (Delve) + `debuginfod-find` для `make demo-delve`
