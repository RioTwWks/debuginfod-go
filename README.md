# debuginfod-go

Сервер debuginfod, реализованный на Go. Он предоставляет HTTP-интерфейс для загрузки отладочной информации (debuginfo, исполняемые файлы, исходники) по запросу клиентов (GDB, LLDB, etc.).

## Особенности

- Простая архитектура на чистом Go (stdlib + SQLite).
- Хранение метаданных в SQLite.
- Индексирует ELF-файлы, извлекая GNU build-id.
- Автоматически извлекает пути исходников из DWARF.
- Поддерживает эндпоинты `/buildid/<id>/debuginfo`, `/buildid/<id>/executable`, `/buildid/<id>/source/*`.
- Периодическое переиндексирование и graceful shutdown.

## Быстрый старт

### Установка

```bash
git clone https://github.com/your-username/debuginfod-go
cd debuginfod-go
go mod download
```

### Запуск

```bash
make run
```

или вручную:

```bash
go run ./cmd/debuginfod -s /path/to/your/elf/files -p 8002
```

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

# скачать debuginfo (если проиндексирован)
curl -O http://localhost:8002/buildid/<BUILDID>/debuginfo
```

## Архитектура (кратко)

| Пакет | Назначение |
|-------|------------|
| `cmd/debuginfod` | Точка входа: флаги, HTTP-сервер, фоновый индексатор |
| `pkg/buildid` | Парсинг GNU build-id из ELF |
| `internal/indexer` | Сканирование директорий, DWARF → исходники |
| `internal/storage` | SQLite: артефакты и исходники |
| `internal/webapi` | HTTP API протокола debuginfod |

## Документация

Подробнее об архитектуре и разработке смотри в [DEVELOPMENT.md](./DEVELOPMENT.md).

## Лицензия

MIT (или ваша лицензия)
