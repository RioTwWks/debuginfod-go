# debuginfod-go

HTTP-сервер [debuginfod](https://sourceware.org/elfutils/Debuginfod.html) на Go. Отдаёт отладочные артефакты (debuginfo, executable, source) по **build-id** — клиентами выступают GDB, LLDB, `debuginfod-find` и другие инструменты из elfutils.

Репозиторий: [github.com/RioTwWks/debuginfod-go](https://github.com/RioTwWks/debuginfod-go)

## Особенности

| Область | Что реализовано |
|---------|-----------------|
| Индексация | ELF, GNU + Go build-id, `.deb`/`.rpm` (целевые ОС), tar, SRPM/DSC, lazy extract |
| HTTP API | `/buildid/*`, `/metadata`, `/healthz`, `/zabbix`, `/ui/` |
| Хранение | SQLite или PostgreSQL, LRU-кэш, отложенное извлечение из архивов |
| Эксплуатация | slog, worker pool, федерация, gzip, graceful shutdown, systemd |
| Мониторинг | Zabbix HTTP agent (`/zabbix`), Web UI дашборд (`/ui/`) |
| CI | GitHub Actions: `vet`, `test -race`, `build` |

Подробный план — [TODO.md](./TODO.md). Архитектура — [DEVELOPMENT.md](./DEVELOPMENT.md).

## Целевые ОС развёртывания

Сервис предназначен для эксплуатации **только** на:

| ОС | Семейство | Основные форматы пакетов |
|----|-----------|--------------------------|
| **Astra Linux** | Debian | `.deb`, `.dsc`, plain tar |
| **Ubuntu** | Debian | `.deb`, `.dsc`, plain tar |
| **RedOS** | RHEL | `.rpm`, `.src.rpm`, plain tar |
| **CentOS** | RHEL | `.rpm`, `.src.rpm`, plain tar |

На всех платформах дополнительно индексируются loose ELF и каталоги отладочных символов (`/usr/lib/debug`, plain `.tar.*`).

Форматы **Alpine (`.apk`)** и **Arch (`.pacman`, `.pkg.tar.*`)** **не поддерживаются** — сервис ориентирован только на deb/rpm-стек целевых ОС.

### Типичные пути scan

```bash
# Astra Linux / Ubuntu (deb)
DEBUGINFOD_SCAN_PATH=/usr/lib/debug,/var/cache/apt/archives

# RedOS / CentOS (rpm)
DEBUGINFOD_SCAN_PATH=/usr/lib/debug,/var/cache/dnf,/var/cache/yum
```

## Быстрый старт

### Требования

- Go 1.21+
- GCC и `libsqlite3-dev` (CGO для SQLite)
- Для тестов: `gcc`; для RPM-тестов: `rpmbuild`
- **Целевые ОС:** Astra Linux, Ubuntu, RedOS, CentOS (см. выше)
- Scan path: ELF, `.deb` / `.rpm`, plain tar, `.src.rpm`, `.dsc`

### Установка и запуск

```bash
git clone https://github.com/RioTwWks/debuginfod-go.git
cd debuginfod-go
go mod download
cp .env.example .env
make run-env
```

Или явные флаги:

```bash
go run ./cmd/debuginfod -s /usr/lib/debug -p 8002
```

### Docker

```bash
docker compose up --build
```

## Конфигурация

Приоритет: **флаги CLI → переменные окружения → `.env` → defaults**.

| Переменная | Флаг | Описание | По умолчанию |
|------------|------|----------|--------------|
| `DEBUGINFOD_DB_PATH` | `-d` | SQLite | `debuginfod.sqlite` |
| `DEBUGINFOD_DATABASE_URL` | `-database-url` | PostgreSQL URL | — |
| `DEBUGINFOD_SCAN_PATH` | `-s` | Пути scan (через запятую) | `.` |
| `DEBUGINFOD_PORT` | `-p` | HTTP-порт | `8002` |
| `DEBUGINFOD_RESCAN_INTERVAL` | `-r` | Интервал переиндексации | `1h` |
| `DEBUGINFOD_METADATA_MAXTIME` | `-metadata-maxtime` | Лимит metadata | `5s` |
| `DEBUGINFOD_LOG_LEVEL` | `-log-level` | Уровень slog | `info` |
| `DEBUGINFOD_CACHE_DIR` | `-cache` | Кэш извлечённых файлов | `.debuginfod-cache` |
| `DEBUGINFOD_CACHE_MAX_BYTES` | `-cache-max-bytes` | LRU лимит кэша (0=∞) | `0` |
| `DEBUGINFOD_LAZY_EXTRACT` | `-lazy-extract` | Не кэшировать ELF при scan | `true` |
| `DEBUGINFOD_UI_ENABLED` | `-ui` | Web UI на `/ui/` | `true` |
| `DEBUGINFOD_SCAN_WORKERS` | `-scan-workers` | Параллельные воркеры | `4` |
| `DEBUGINFOD_URLS` | `-upstream` | Upstream для федерации | — |
| `DEBUGINFOD_ZABBIX_KEY` | `-zabbix-key` | Токен `/zabbix` | — |
| `DEBUGINFOD_ENV_FILE` | `-env-file` | Путь к `.env` | `.env` |

Полный пример: [.env.example](./.env.example).

## HTTP API

### Артефакты

```http
GET /buildid/<BUILDID>/debuginfo
GET /buildid/<BUILDID>/executable
GET /buildid/<BUILDID>/source/<абсолютный/путь>
GET /buildid/<BUILDID>/section/<имя_секции>
```

`BUILDID` — lowercase hex. Go: SHA-256 от raw build-id ([DEVELOPMENT.md](./DEVELOPMENT.md#go-build-id)).

### Metadata

```http
GET /metadata?key=glob&value=/usr/bin/*
GET /metadata?key=file&value=/usr/bin/hello
GET /metadata?key=buildid&value=<hex>
```

### Мониторинг и UI

```http
GET /healthz              → 200 ok
GET /zabbix               → JSON-метрики (Zabbix HTTP agent)
GET /ui/                  → Web UI дашборд
GET /ui/api/stats         → счётчики индекса
GET /ui/api/search?q=     → поиск по префиксу build-id
```

Zabbix: [deploy/zabbix/README.md](deploy/zabbix/README.md).

## Использование с GDB

```bash
export DEBUGINFOD_URLS="http://localhost:8002"
gdb /path/to/binary
```

## Проверка

```bash
make test && make build
curl http://localhost:8002/healthz
curl http://localhost:8002/ui/api/stats
readelf -n /bin/ls | grep 'Build ID'
curl 'http://localhost:8002/metadata?key=glob&value=/bin/*'
```

## Архитектура

```
scan paths ──► indexer (workers) ──► SQLite/PostgreSQL ◄── webapi / webui
                    │                      ▲
                    ├── archive (lazy)     │
                    ├── cache (LRU)        └── federation (404 → upstream)
                    └── metrics ──► /zabbix
```

| Пакет | Назначение |
|-------|------------|
| `cmd/debuginfod` | Точка входа |
| `internal/config` | `.env` + флаги |
| `pkg/buildid` | GNU и Go build-id |
| `pkg/elfsection` | ELF-секции |
| `internal/archive` | deb/rpm, tar, SRPM/DSC (целевые ОС) |
| `internal/indexer` | Scan, DWARF, lazy extract |
| `internal/storage` | БД, metadata, stats |
| `internal/webapi` | debuginfod HTTP API |
| `internal/webui` | Дашборд `/ui/` |
| `internal/metrics` | Zabbix JSON |
| `internal/federation` | Upstream proxy |
| `internal/cache` | LRU prune |
| `internal/logging` | slog |
| `internal/fnmatch` | metadata glob |

## Документация

| Файл | Содержание |
|------|------------|
| [DEVELOPMENT.md](./DEVELOPMENT.md) | Архитектура, тесты, сравнение с upstream |
| [CONTRIBUTING.md](./CONTRIBUTING.md) | Процесс PR |
| [TODO.md](./TODO.md) | Roadmap |
| [.cursor/rules.md](./.cursor/rules.md) | Правила для Cursor AI |
| [deploy/zabbix/README.md](./deploy/zabbix/README.md) | Мониторинг Zabbix |

## Лицензия

MIT
