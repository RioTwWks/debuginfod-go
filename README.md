# debuginfod-go

HTTP-сервер [debuginfod](https://sourceware.org/elfutils/Debuginfod.html) на Go. Отдаёт отладочные артефакты (debuginfo, executable, source) по **build-id** — клиентами выступают GDB, LLDB, `debuginfod-find` и другие инструменты из elfutils.

Репозиторий: [github.com/RioTwWks/debuginfod-go](https://github.com/RioTwWks/debuginfod-go)

## Особенности

| Область | Что реализовано |
|---------|-----------------|
| Индексация | ELF на диске, GNU + Go build-id, `.deb`/`.rpm`, DWARF → исходники |
| HTTP API | `/buildid/*`, `/metadata`, `/healthz` |
| Хранение | SQLite, кэш извлечённых файлов из архивов |
| Конфигурация | `.env`, переменные окружения, флаги CLI |
| Эксплуатация | Периодический rescan, graceful shutdown, Docker |
| CI | GitHub Actions: `vet`, `test -race`, `build` |

Подробный план развития — в [TODO.md](./TODO.md).

## Быстрый старт

### Требования

- Go 1.21+
- GCC и `libsqlite3-dev` (CGO для SQLite)
- Для тестов с C-бинарниками: `gcc`
- Для индексации RPM: пакеты в формате `.rpm` в scan path

### Установка

```bash
git clone https://github.com/RioTwWks/debuginfod-go.git
cd debuginfod-go
go mod download
```

### Запуск

```bash
cp .env.example .env   # отредактируйте при необходимости
make run-env
```

Или с явными флагами:

```bash
go run ./cmd/debuginfod -s /usr/lib/debug -p 8002
```

### Docker

```bash
docker compose up --build
```

## Конфигурация

Приоритет: **флаги CLI → переменные окружения → `.env` → значения по умолчанию**.

| Переменная | Флаг | Описание | По умолчанию |
|------------|------|----------|--------------|
| `DEBUGINFOD_DB_PATH` | `-d` | Путь к SQLite | `debuginfod.sqlite` |
| `DEBUGINFOD_SCAN_PATH` | `-s` | Пути сканирования (через запятую) | `.` |
| `DEBUGINFOD_PORT` | `-p` | HTTP-порт | `8002` |
| `DEBUGINFOD_RESCAN_INTERVAL` | `-r` | Интервал переиндексации | `1h` |
| `DEBUGINFOD_METADATA_MAXTIME` | `-metadata-maxtime` | Лимит metadata-запросов | `5s` |
| `DEBUGINFOD_CACHE_DIR` | `-cache` | Кэш ELF из архивов | `.debuginfod-cache` |
| `DEBUGINFOD_LOG_LEVEL` | `-log-level` | Уровень логов (пока не в slog) | `info` |
| `DEBUGINFOD_ENV_FILE` | `-env-file` | Путь к `.env` | `.env` |

## HTTP API

### Артефакты по build-id

```http
GET /buildid/<BUILDID>/debuginfo
GET /buildid/<BUILDID>/executable
GET /buildid/<BUILDID>/source/<абсолютный/путь/к/файлу>
GET /buildid/<BUILDID>/section/<имя_секции>
```

`BUILDID` — lowercase hex. Для Go-бинарников используется SHA-256 от raw build-id (см. [DEVELOPMENT.md](./DEVELOPMENT.md#go-build-id)).

### Metadata

```http
GET /metadata?key=glob&value=/usr/bin/*
GET /metadata?key=file&value=/usr/bin/hello
GET /metadata?key=buildid&value=<hex>
```

Ответ:

```json
{
  "results": [
    {
      "buildid": "…",
      "type": "executable",
      "file": "/usr/bin/hello",
      "archive": "/path/to/pkg.rpm"
    }
  ],
  "complete": true
}
```

### Health

```http
GET /healthz   → 200 ok
```

## Использование с GDB

```bash
export DEBUGINFOD_URLS="http://localhost:8002"
gdb /path/to/binary
```

Для C/C++ бинарников GDB берёт GNU build-id из ELF. Для Go — см. раздел про Go build-id в DEVELOPMENT.md.

## Проверка работы

```bash
make test
make build

curl http://localhost:8002/healthz
readelf -n /bin/ls | grep 'Build ID'
curl 'http://localhost:8002/metadata?key=glob&value=/bin/*'
```

## Архитектура

```
scan paths ──► indexer ──► SQLite ◄── webapi ◄── HTTP clients (GDB, curl)
                  │                        │
                  └── archive (.deb/.rpm)  └── /buildid, /metadata
```

| Пакет | Назначение |
|-------|------------|
| `cmd/debuginfod` | Точка входа, HTTP-сервер, фоновый индексатор |
| `internal/config` | `.env` + флаги |
| `pkg/buildid` | GNU и Go build-id из ELF notes |
| `internal/archive` | ELF внутри `.deb`/`.rpm` |
| `internal/indexer` | Обход FS, DWARF, запись в БД |
| `internal/storage` | SQLite: артефакты, sources, metadata |
| `internal/webapi` | HTTP-обработчики |
| `internal/fnmatch` | Shell-glob с FNM_PATHNAME для metadata |
| `pkg/elfsection` | Извлечение сырых ELF-секций |

## Документация

| Файл | Содержание |
|------|------------|
| [DEVELOPMENT.md](./DEVELOPMENT.md) | Архитектура, dev workflow, тесты |
| [CONTRIBUTING.md](./CONTRIBUTING.md) | Как вносить изменения |
| [TODO.md](./TODO.md) | Roadmap и идеи |
| [.env.example](./.env.example) | Пример конфигурации |

## Лицензия

MIT
