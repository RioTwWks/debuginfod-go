# debuginfod-go

Сервер debuginfod, реализованный на Go. Он предоставляет HTTP-интерфейс для загрузки отладочной информации (debuginfo, исполняемые файлы, исходники) по запросу клиентов (GDB, LLDB, etc.).

## Особенности

- 🚀 Простая архитектура на чистом Go.
- 🗄️ Хранение метаданных в SQLite.
- 🔍 Индексирует ELF-файлы, извлекая build-id.
- 🧩 Поддерживает эндпоинты `/buildid/<id>/debuginfo`, `/buildid/<id>/executable`, `/buildid/<id>/source/*`.

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

## Документация

Подробнее об архитектуре и разработке смотри в [DEVELOPMENT.md](./DEVELOPMENT.md).

## Лицензия

MIT (или ваша лицензия)
