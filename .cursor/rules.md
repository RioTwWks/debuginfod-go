# Правила Cursor для debuginfod-go

Контекст для AI-ассистента при работе с репозиторием.

## Проект

- **Язык:** Go 1.21+
- **Тип:** HTTP-сервер debuginfod + SQLite + индексация ELF/архивов
- **Модуль:** `github.com/your-username/debuginfod-go`
- **Репозиторий:** https://github.com/RioTwWks/debuginfod-go

## Структура пакетов

```
cmd/debuginfod/          # main, только сборка и wiring
internal/config/         # .env + флаги
internal/indexer/        # scan FS, DWARF sources
internal/storage/        # SQLite
internal/webapi/         # HTTP handlers
internal/archive/        # .deb, .rpm
pkg/buildid/             # публичный парсер ELF build-id
```

## Принципы

1. **Минимальный diff** — не рефакторить несвязанный код.
2. **stdlib first** — `net/http`, `debug/elf`, `debug/dwarf`, `database/sql`.
3. **CGO** — нужен для `github.com/mattn/go-sqlite3`; в CI ставится `libsqlite3-dev`.
4. **Без Docker в коде** — Dockerfile допустим, но логика не должна зависеть от контейнера.
5. **Ошибки** — возвращать `error`, логировать в `main`/handlers через `log` (позже `slog`).
6. **Тесты** — `testing` + table-driven; `t.Skip` если нет `gcc`/`rpmbuild`.

## HTTP API (реализовано)

| Маршрут | Назначение |
|---------|------------|
| `GET /buildid/<id>/debuginfo` | Отдать debuginfo |
| `GET /buildid/<id>/executable` | Отдать executable |
| `GET /buildid/<id>/source/<path>` | Отдать исходник |
| `GET /buildid/<id>/section/<name>` | Сырое содержимое ELF-секции |
| `GET /healthz` | Liveness |

**Не реализовано** (см. TODO.md): федерация, IMA, `/metrics`.

## Build-id

- **GNU:** hex из `NT_GNU_BUILD_ID` (owner `GNU`, type 3).
- **Go:** `hex(sha256(raw))` из `NT_GO_BUILD_ID` (owner `Go`, type 4); raw хранится в `raw_build_id`.
- Приоритет: GNU > Go.

## Конфигурация

Переменные `DEBUGINFOD_*` — см. `.env.example`. Загрузка в `internal/config.Load()`.

## При добавлении фич

1. Проверить [TODO.md](../TODO.md).
2. Написать тест.
3. Обновить README / DEVELOPMENT при смене API.
4. `go fmt`, `go vet`, `go test ./...`.

## Запрещено

- Хардкодить пути и секреты.
- `panic` в `internal/*` и `pkg/*` (кроме init при фатальных misconfig).
- Коммитить `debuginfod` бинарник, `*.sqlite`, `.debuginfod-cache/`.
- Ломать совместимость metadata JSON без обновления документации.

## MCP

Конфиг: [mcp.json](./mcp.json).

| Сервер | Назначение |
|--------|------------|
| `go-doc` | Документация Go stdlib |
| `sqlite` | Инспекция `debuginfod.sqlite` |
| `go-quality-local` | Линтинг и качество Go-кода |
| `test-runner` | Запуск `go test ./...` |

Дополнительно в `mcp.json`: `projectScripts` (make test, curl healthz) и `hints` для контекста.

## Roadmap

Актуальный список задач: [TODO.md](../TODO.md)
