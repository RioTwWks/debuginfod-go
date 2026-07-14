# Правила Cursor для debuginfod-go

Контекст для AI-ассистента при работе с репозиторием.

## Проект

- **Язык:** Go 1.21+
- **Тип:** HTTP-сервер debuginfod + SQLite/PostgreSQL + индексация ELF/архивов
- **Модуль:** `github.com/your-username/debuginfod-go`
- **Репозиторий:** https://github.com/RioTwWks/debuginfod-go
- **Целевые ОС:** Astra Linux, Ubuntu, RedOS, CentOS (deb/rpm-стек). Arch/Alpine — вне scope развёртывания.

## Структура пакетов

```
cmd/debuginfod/          # main: wiring HTTP, indexer, graceful shutdown
internal/config/         # .env + флаги CLI
internal/indexer/        # scan FS (worker pool), DWARF sources, lazy extract
internal/storage/        # SQLite / PostgreSQL: artifacts, sources, metadata
internal/webapi/         # /buildid, /metadata, /healthz, /zabbix, gzip, federation
internal/webui/          # /ui/ дашборд (embed static)
internal/archive/        # .deb, .rpm (целевые ОС), tar, SRPM, DSC; apk/pacman — опционально
internal/metrics/        # runtime counters + /zabbix JSON
internal/federation/     # upstream proxy при 404
internal/cache/          # LRU prune кэша
internal/logging/        # log/slog JSON
internal/fnmatch/        # FNM_PATHNAME для metadata glob
pkg/buildid/             # GNU + Go build-id из ELF notes
pkg/elfsection/          # сырые ELF-секции для /section
deploy/                  # systemd unit, Zabbix docs
```

## Принципы

1. **Минимальный diff** — не рефакторить несвязанный код.
2. **stdlib first** — `net/http`, `debug/elf`, `debug/dwarf`, `database/sql`, `log/slog`.
3. **CGO** — нужен для `github.com/mattn/go-sqlite3`; PostgreSQL через `pgx` без CGO.
4. **Без Docker в логике** — Dockerfile допустим, бизнес-код не зависит от контейнера.
5. **Ошибки** — возвращать `error`; логировать через `slog` в handlers/main.
6. **Тесты** — `testing` + table-driven; `t.Skip` если нет `gcc`/`rpmbuild`.

## HTTP API (реализовано)

| Маршрут | Назначение |
|---------|------------|
| `GET /buildid/<id>/debuginfo` | Отдать debuginfo (stream из архива при lazy) |
| `GET /buildid/<id>/executable` | Отдать executable |
| `GET /buildid/<id>/source/<path>` | Отдать исходник |
| `GET /buildid/<id>/section/<name>` | Сырое содержимое ELF-секции |
| `GET /metadata?key=glob\|file\|buildid&value=...` | Поиск артефактов (fnmatch, timeout) |
| `GET /healthz` | Liveness |
| `GET /zabbix` | JSON-метрики для Zabbix HTTP agent |
| `GET /ui/` | Web UI: статистика + поиск по build-id |
| `GET /ui/api/stats` | JSON счётчиков для UI |
| `GET /ui/api/search?q=` | Поиск по префиксу build-id |

**Middleware:** gzip, HTTP metrics (2xx/4xx/5xx), federation fallback на 404.

## Индексация

### Форматы архивов

**Целевые ОС (Astra Linux, Ubuntu, RedOS, CentOS):**

| Формат | Расширения | ОС |
|--------|-----------|-----|
| Debian | `.deb` | Astra, Ubuntu |
| RPM | `.rpm` | RedOS, CentOS |
| Plain tar | `.tar`, `.tar.gz`, `.tar.xz`, `.tar.zst` | все |
| SRPM | `.src.rpm` | RedOS, CentOS |
| DSC | `.dsc` | Astra, Ubuntu |

**Вне целевых ОС** (код есть, не приоритет для развёртывания/тестов):

| Формат | Расширения | Примечание |
|--------|-----------|------------|
| Alpine | `.apk` | не развёртывается |
| Arch | `.pacman`, `.pkg.tar.*` | не развёртывается |

### Отложенное извлечение

`DEBUGINFOD_LAZY_EXTRACT=true` (по умолчанию): при индексации в БД пишутся `archive_path` + `member_path`, ELF извлекается по HTTP-запросу (`OpenMemberReader` / `ExtractMember`).

## Build-id

- **GNU:** hex из `NT_GNU_BUILD_ID` (owner `GNU`, type 3).
- **Go:** `hex(sha256(raw))` из `NT_GO_BUILD_ID` (owner `Go`, type 4); raw в `raw_build_id`.
- Приоритет: GNU > Go.
- Нормализация: lowercase hex, без `0x` и дефисов.

## Конфигурация

Переменные `DEBUGINFOD_*` — см. `.env.example`. Загрузка: `internal/config.Load()`.

| Переменная | По умолчанию | Назначение |
|------------|--------------|------------|
| `DEBUGINFOD_DB_PATH` | `debuginfod.sqlite` | SQLite |
| `DEBUGINFOD_DATABASE_URL` | — | PostgreSQL (альтернатива SQLite) |
| `DEBUGINFOD_SCAN_PATH` | `.` | Пути scan (через запятую) |
| `DEBUGINFOD_PORT` | `8002` | HTTP-порт |
| `DEBUGINFOD_RESCAN_INTERVAL` | `1h` | Периодический rescan |
| `DEBUGINFOD_METADATA_MAXTIME` | `5s` | Таймаут metadata |
| `DEBUGINFOD_LOG_LEVEL` | `info` | slog level |
| `DEBUGINFOD_CACHE_DIR` | `.debuginfod-cache` | Кэш извлечённых файлов |
| `DEBUGINFOD_CACHE_MAX_BYTES` | `0` | LRU лимит кэша (0=∞) |
| `DEBUGINFOD_LAZY_EXTRACT` | `true` | Отложенное извлечение |
| `DEBUGINFOD_UI_ENABLED` | `true` | Web UI на `/ui/` |
| `DEBUGINFOD_SCAN_WORKERS` | `4` | Параллельные воркеры |
| `DEBUGINFOD_URLS` | — | Upstream для федерации |
| `DEBUGINFOD_ZABBIX_KEY` | — | Токен `/zabbix` |

## База данных

Таблицы: `artifacts`, `sources`, `scanned_files`. Схема и миграции: `internal/storage/sqlite.go`, `postgres.go`.

Ключевые поля `artifacts`: `build_id`, `type`, `file_path`, `archive_path`, `member_path`, `build_id_kind`, `raw_build_id`.

## При добавлении фич

1. Проверить [TODO.md](../TODO.md).
2. Написать тест.
3. Обновить README / DEVELOPMENT / `.env.example` при смене API или конфига.
4. `go fmt`, `go vet`, `go test ./...`.

## Запрещено

- Хардкодить пути и секреты.
- `panic` в `internal/*` и `pkg/*` (кроме init при фатальных misconfig).
- `print()` для логирования — только `slog`.
- Коммитить бинарник `debuginfod`, `*.sqlite`, `.debuginfod-cache/`.
- Ломать совместимость metadata JSON без обновления документации.
- Менять статус артефактов предыдущего run при diff (создавать записи в текущем run).

## MCP

Конфиг: [mcp.json](./mcp.json).

| Сервер | Назначение |
|--------|------------|
| `go-doc` | Документация Go stdlib |
| `sqlite` | Инспекция `debuginfod.sqlite` |
| `go-quality-local` | Линтинг Go-кода |
| `test-runner` | `go test ./...` |

`projectScripts`: make test/build/run, curl healthz/metadata/ui/zabbix.

## Roadmap

Актуальный список: [TODO.md](../TODO.md). Выполнено: elfutils-совместимость, Zabbix, архивы, Web UI.
