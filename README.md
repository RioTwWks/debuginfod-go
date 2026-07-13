# debuginfod-go

Сервер debuginfod, реализованный на Go. Он предоставляет HTTP-интерфейс для загрузки отладочной информации (debuginfo, исполняемые файлы, исходники) по запросу клиентов (GDB, LLDB, etc.).

## Особенности

- Простая архитектура на чистом Go (stdlib + SQLite).
- Хранение метаданных в SQLite.
- Индексирует ELF-файлы: GNU build-id и Go build-id (`.note.go.buildid`).
- Индексация `.deb` и `.rpm` пакетов.
- Автоматически извлекает пути исходников из DWARF.
- HTTP API: `/buildid/*`, `/metadata`, `/healthz`.
- Конфигурация через `.env` и флаги командной строки.
- Периодическое переиндексирование и graceful shutdown.
- CI: GitHub Actions (тесты, vet, сборка).

## Быстрый старт

### Установка

```bash
git clone https://github.com/your-username/debuginfod-go
cd debuginfod-go
go mod download
```

### Запуск

Скопируйте `.env.example` в `.env` и отредактируйте при необходимости:

```bash
cp .env.example .env
make run-env
```

или вручную:

```bash
go run ./cmd/debuginfod -s /path/to/your/elf/files -p 8002
```

### Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `DEBUGINFOD_DB_PATH` | Путь к SQLite | `debuginfod.sqlite` |
| `DEBUGINFOD_SCAN_PATH` | Пути сканирования (через запятую) | `.` |
| `DEBUGINFOD_PORT` | HTTP-порт | `8002` |
| `DEBUGINFOD_RESCAN_INTERVAL` | Интервал переиндексации | `1h` |
| `DEBUGINFOD_CACHE_DIR` | Кэш извлечённых файлов из архивов | `.debuginfod-cache` |

### Использование с GDB

```bash
export DEBUGINFOD_URLS="http://localhost:8002"
gdb /path/to/your/binary
```

### Проверка работы

```bash
# health-check
curl http://localhost:8002/healthz

# узнать build-id бинарника
readelf -n /bin/ls | grep 'Build ID'

# metadata-поиск (glob)
curl 'http://localhost:8002/metadata?key=glob&value=/usr/bin/*'
```

## Архитектура (кратко)

| Пакет | Назначение |
|-------|------------|
| `cmd/debuginfod` | Точка входа: конфиг, HTTP-сервер, фоновый индексатор |
| `internal/config` | Загрузка `.env` и флагов |
| `pkg/buildid` | Парсинг GNU и Go build-id из ELF |
| `internal/archive` | Чтение ELF из `.deb`/`.rpm` |
| `internal/indexer` | Сканирование директорий, DWARF → исходники |
| `internal/storage` | SQLite: артефакты, исходники, metadata |
| `internal/webapi` | HTTP API протокола debuginfod |

## Документация

Подробнее об архитектуре и разработке смотри в [DEVELOPMENT.md](./DEVELOPMENT.md).

## Лицензия

MIT (или ваша лицензия)
