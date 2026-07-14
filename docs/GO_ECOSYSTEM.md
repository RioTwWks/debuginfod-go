# Go-экосистема и debuginfod-go

Руководство по идентификации Go-бинарников, маппингу build-id и отладке через **Delve** / **GDB** с `DEBUGINFOD_URLS`.

## Build-id в Go-бинарниках

Go записывает ELF note `.note.go.buildid` (owner `Go`, type `4`). Сырое значение — строка вида:

```text
<action>/<module>/<checksum>
```

Пример:

```bash
go build -o /tmp/hello .
go tool buildid /tmp/hello
# V3tM5FaSu1fxU2Ua_7Bv/IF91Dv7gL90n1SdU2f6T/-6ccm9oZTO6X6Gb5EMng/-wYdIEKlP6aKUBzRQJ7z
```

Строка содержит `/` и **не подходит** для URL debuginfod (`/buildid/<id>/...`).

### Канонический ID для HTTP API

debuginfod-go (как и upstream debuginfod для Go) использует:

```text
canonical_id = hex(sha256(raw_go_build_id))
```

Реализация: `pkg/buildid.GoCanonicalID`.

### Маппинг: `go tool buildid` → debuginfod

| Поле / инструмент | Значение | Пример |
|-------------------|----------|--------|
| `go tool buildid <bin>` | raw Go build-id | `abc/module/sum` |
| URL `/buildid/<id>/...` | `hex(sha256(raw))` | 64 hex-символа |
| metadata `buildid` | канонический ID (GNU или Go) | `b018b3ae...` или `a1b2c3...` |
| metadata `buildid_kind` | `gnu` или `go` | `go` |
| metadata `raw_buildid` | raw Go note (только для Go) | `abc/module/sum` |

### Поиск по metadata

Запрос по build-id принимает **канонический hex**, **raw Go build-id** или **GNU hex** (см. `MatchBuildIDQuery`):

```bash
RAW=$(go tool buildid ./myapp)
CANON=$(go run -e '...' )  # или см. пример ниже

curl "http://localhost:8002/metadata?key=buildid&value=${CANON}"
curl "http://localhost:8002/metadata?key=buildid&value=${RAW}"
```

Практичный способ получить канонический ID без ручного SHA-256:

```bash
export DEBUGINFOD_URLS=http://localhost:8002
debuginfod-find --key file --value /path/to/myapp
# в JSON: "buildid", "buildid_kind", "raw_buildid"
```

Или одной строкой (требует `go` и `sha256sum`):

```bash
go tool buildid ./myapp | xargs -I{} sh -c 'printf %s "{}" | sha256sum | awk "{print \$1}"'
```

### Приоритет GNU build-id

Если в ELF есть **и** GNU (`.note.gnu.build-id`), **и** Go note — индексатор и HTTP API используют **GNU** (40 hex-символов для SHA-1).

GNU note появляется при сборке с **внешним линкером**:

```bash
go build -ldflags="-linkmode=external" -o myapp .
readelf -n myapp | grep 'Build ID'
```

| Сборка | Go note | GNU note | ID в debuginfod |
|--------|---------|----------|-----------------|
| `go build` (default) | ✅ | ❌ | `hex(sha256(raw))` |
| `go build -buildmode=pie` | ✅ | ❌* | `hex(sha256(raw))` |
| `go build -ldflags="-linkmode=external"` | ✅ | ✅ | GNU hex |
| `go build -buildmode=pie -ldflags="-linkmode=external"` | ✅ | ✅ | GNU hex |

\* На некоторых платформах PIE без external linker GNU note не создаётся.

Production-сборки (hardened PIE + distro GCC) обычно попадают в последнюю строку таблицы.

## Отладка с Delve

[Delve](https://github.com/go-delve/delve) на Linux подтягивает отдельный debuginfo через клиент **elfutils** `debuginfod-find`, который читает `DEBUGINFOD_URLS` (как GDB).

### Быстрый старт

```bash
# Терминал 1 — сервер
DEBUGINFOD_SCAN_PATH=/path/to/binaries ./debuginfod -p 8002

# Терминал 2 — Delve
export DEBUGINFOD_URLS=http://localhost:8002
dlv exec ./myapp
```

Требования на машине отладчика:

- `dlv` (Delve)
- `debuginfod-find` из пакета `elfutils` (или совместимый `debuginfod-find` из этого репозитория)

### Stripped binary + отдельный .debug

Как для C/C++: вынести DWARF в отдельный файл и проиндексировать оба:

```bash
go build -gcflags="all=-N -l" -ldflags="-linkmode=external" -o hello.full .
objcopy --only-keep-debug hello.full hello.debug
objcopy --strip-debug hello.full hello
# Положить hello и hello.debug в DEBUGINFOD_SCAN_PATH
```

Демо: [examples/delve/](../examples/delve/).

### Проверка до запуска Delve

```bash
export DEBUGINFOD_URLS=http://localhost:8002
BUILD_ID=$(readelf -n hello | awk '/Build ID/ {print $3; exit}')
# для чисто Go-бинарника без GNU:
# BUILD_ID=$(go tool buildid hello | xargs -I{} sh -c 'printf %s "{}" | sha256sum | awk "{print \$1}"')

debuginfod-find debuginfo "$BUILD_ID"
debuginfod-find executable "$BUILD_ID"
```

## Отладка с GDB

Go-бинарники с DWARF отлаживаются и через GDB:

```bash
export DEBUGINFOD_URLS=http://localhost:8002
gdb ./myapp
```

Демо на C: [examples/gdb/](../examples/gdb/).

## CI / production рекомендации

1. **Индексировать** каталог с бинарниками и `.debug` (или unstripped копии).
2. Для PIE + external linker ориентироваться на **GNU build-id** в URL и metadata.
3. В Delve/GDB на рабочих станциях задать `DEBUGINFOD_URLS` (или `/etc/debuginfod/*.urls` на дистрибутивах с profile.d).
4. Для поиска артефактов Go-приложений использовать `metadata?key=file&value=<path>` — в ответе будут `buildid`, `buildid_kind`, `raw_buildid`.

## См. также

- [DEVELOPMENT.md](../DEVELOPMENT.md) — внутренности `pkg/buildid`
- [README.md](../README.md) — переменные `DEBUGINFOD_*`
- [examples/README.md](../examples/README.md) — docker-compose демо
