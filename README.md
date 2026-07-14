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

Подробный план — [TODO.md](./TODO.md). Архитектура — [DEVELOPMENT.md](./DEVELOPMENT.md). Эксплуатация — [deploy/OPERATIONS.md](./deploy/OPERATIONS.md).

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

### Пакеты и оффлайн-установка (production)

На целевых ОС (Astra/Ubuntu/RedOS/CentOS) — нативные пакеты и установка **без интернета**:

```bash
make package                    # .deb + .rpm в dist/
make offline-bundle-deb         # bundle для переноса (online build-хост)
# на оффлайн-хосте: tar xf … && sudo ./install-offline.sh
```

Подробно: [deploy/README.md](deploy/README.md), [deploy/offline/README.md](deploy/offline/README.md).

Примеры с GDB и демо stripped binary: [examples/](./examples/).

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
| `DEBUGINFOD_CORS_ORIGINS` | `-cors-origins` | CORS origins (`*`=все) | — |
| `DEBUGINFOD_RATE_LIMIT` | `-rate-limit` | Лимит запросов/с на IP (0=выкл) | `0` |
| `DEBUGINFOD_BASIC_AUTH_USER` | `-basic-auth-user` | Basic Auth пользователь | — |
| `DEBUGINFOD_BASIC_AUTH_PASSWORD` | `-basic-auth-password` | Basic Auth пароль | — |
| `DEBUGINFOD_TLS_CERT` | `-tls-cert` | TLS сертификат | — |
| `DEBUGINFOD_TLS_KEY` | `-tls-key` | TLS ключ | — |
| `DEBUGINFOD_TLS_CLIENT_CA` | `-tls-client-ca` | CA для mTLS клиентов | — |
| `DEBUGINFOD_METADATA_PAGE_SIZE` | `-metadata-page-size` | Размер страницы metadata | `100` |
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
GET /metadata?key=glob&value=/bin/*&offset=0&limit=100
```

Ответ — JSON с полями `artifacts` и `next_offset` (если есть ещё страницы). Параметры `offset`/`limit` опциональны; по умолчанию `limit` берётся из `DEBUGINFOD_METADATA_PAGE_SIZE`.

### OpenAPI и безопасность

```http
GET /openapi.yaml          → OpenAPI 3.0 спецификация
```

Опционально: CORS (`DEBUGINFOD_CORS_ORIGINS`), rate limiting (`DEBUGINFOD_RATE_LIMIT`), Basic Auth (`DEBUGINFOD_BASIC_AUTH_*`), TLS/mTLS (`DEBUGINFOD_TLS_*`). `/healthz` доступен без Basic Auth.

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

Демо: [examples/gdb/](examples/gdb/).

## Использование с Delve (Go)

Delve на Linux вызывает `debuginfod-find`, который читает `DEBUGINFOD_URLS`:

```bash
export DEBUGINFOD_URLS="http://localhost:8002"
dlv exec ./myapp
```

Маппинг Go build-id (`go tool buildid` → URL), PIE и external linker: [docs/GO_ECOSYSTEM.md](docs/GO_ECOSYSTEM.md).  
Демо: [examples/delve/](examples/delve/) (`make -C examples demo-delve`).

## CLI `debuginfod-find`

Совместимая обёртка над HTTP API (сборка: `make build-find`):

```bash
export DEBUGINFOD_URLS="http://localhost:8002"

# Скачать debuginfo / executable
debuginfod-find debuginfo <BUILDID> -o /tmp/out.debug
debuginfod-find executable <BUILDID>

# Исходник и ELF-секция
debuginfod-find source <BUILDID> /usr/src/hello.c
debuginfod-find section <BUILDID> .note.gnu.build-id

# Metadata
debuginfod-find --key glob --value '/bin/*'
```

Флаг `--url` переопределяет `DEBUGINFOD_URLS`.

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
| `cmd/debuginfod-find` | CLI-клиент HTTP API |
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
| [examples/](./examples/) | Docker-compose и GDB-скрипт |
| [.cursor/rules.md](./.cursor/rules.md) | Правила для Cursor AI |
| [deploy/README.md](./deploy/README.md) | Пакеты, Ansible, nginx, Zabbix, продакшн-схема |
| [deploy/ansible/README.md](./deploy/ansible/README.md) | Ansible playbook |
| [deploy/nginx/README.md](./deploy/nginx/README.md) | Reverse proxy |
| [deploy/offline/README.md](./deploy/offline/README.md) | Оффлайн bundle `.deb`/`.rpm` |
| [deploy/backup/README.md](./deploy/backup/README.md) | Backup и restore |
| [deploy/postgresql/README.md](./deploy/postgresql/README.md) | PostgreSQL в проде |
| [deploy/OPERATIONS.md](./deploy/OPERATIONS.md) | Руководство по эксплуатации |
| [deploy/security/README.md](./deploy/security/README.md) | Path traversal, IMA, systemd hardening |

## Лицензия

MIT
