# Тестирование debuginfod-go (ELF, HTTP, GDB)

Проверка сервиса **как у стандартного [debuginfod](https://sourceware.org/elfutils/Debuginfod.html)**: сначала готовность HTTP API и протокол `/buildid/*`, затем клиенты GDB и `debuginfod-find`.

См. также: [examples/README.md](../examples/README.md) (Docker-демо), [docs/GO_ECOSYSTEM.md](./GO_ECOSYSTEM.md) (Go / Delve).

## Содержание

1. [Предварительные условия](#предварительные-условия)
2. [Готовность сервера](#готовность-сервера)
3. [Тест на уровне ELF и HTTP](#тест-на-уровне-elf-и-http)
4. [Клиент debuginfod-find](#клиент-debuginfod-find)
5. [Тест через GDB](#тест-через-gdb)
6. [Quik / dedup](#quik--dedup)
7. [Демо из репозитория](#демо-из-репозитория)
8. [Устранение неполадок](#устранение-неполадок)

---

## Предварительные условия

| Компонент | Назначение |
|-----------|------------|
| `debuginfod-go` | Сервер, порт по умолчанию `8002` |
| `readelf` | Build ID из ELF (`binutils`) |
| `curl` | Проверка HTTP API |
| `debuginfod-find` | CLI-клиент (elfutils или `make build-find`) |
| `gdb` | Отладчик с поддержкой debuginfod (`gdb` + `debuginfod` / `elfutils`) |

На **Astra / Ubuntu**:

```bash
sudo apt install binutils curl gdb debuginfod
```

Сборка клиента из репозитория (опционально):

```bash
make build-find
export PATH="$PWD:$PATH"
```

---

## Готовность сервера

```bash
export DEBUGINFOD_URL=http://127.0.0.1:8002

# Liveness — процесс жив
curl -sf "${DEBUGINFOD_URL}/healthz"

# Readiness — первый scan завершён (важно для GDB)
curl -sf "${DEBUGINFOD_URL}/readyz"

# Сводка индекса
curl -s "${DEBUGINFOD_URL}/ui/api/stats" | jq .
```

| Endpoint | Ожидание |
|----------|----------|
| `/healthz` | `200`, тело `ok` |
| `/readyz` | `200` после первого scan; `503` пока индексация идёт |
| `/ui/api/stats` | `artifacts_total > 0`, `scan_running: false` |

Пока `readyz` возвращает **503**, клиенты могут получать **404** на артефакты, которых ещё нет в индексе.

---

## Тест на уровне ELF и HTTP

Это тот же протокол, что использует `debuginfod-find` и libdebuginfod в GDB.

### 1. Build ID из бинарника

```bash
BIN=/path/to/binary    # stripped executable или .debug

readelf -n "$BIN" | grep -A1 'Build ID'

BUILDID=$(readelf -n "$BIN" | awk '/Build ID/ {print $3}')
echo "$BUILDID"
```

Для Go с external linker см. [GO_ECOSYSTEM.md](./GO_ECOSYSTEM.md) — ID в URL может отличаться от `go tool buildid`.

### 2. Metadata (поиск в индексе)

```bash
# По имени файла
curl -s "${DEBUGINFOD_URL}/metadata?key=file&value=$(basename "$BIN")"

# По полному пути (как в SCAN_PATH)
curl -s "${DEBUGINFOD_URL}/metadata?key=file&value=${BIN}"

# Glob (fnmatch)
curl -s "${DEBUGINFOD_URL}/metadata?key=glob&value=/usr/bin/*"

# По префиксу build-id
curl -s "${DEBUGINFOD_URL}/metadata?key=buildid&value=${BUILDID:0:8}"
```

Успех: JSON с полями `buildid`, `type` (`executable` или `debuginfo`).

### 3. Скачивание артефактов (`/buildid/*`)

```bash
# Executable
curl -f -o /tmp/from-server.exec \
  "${DEBUGINFOD_URL}/buildid/${BUILDID}/executable"

# Debuginfo (отдельный .debug с тем же build-id)
curl -f -o /tmp/from-server.debug \
  "${DEBUGINFOD_URL}/buildid/${BUILDID}/debuginfo"

# ELF-секция
curl -f -o /tmp/build-id.note \
  "${DEBUGINFOD_URL}/buildid/${BUILDID}/section/.note.gnu.build-id"
```

Проверка скачанного файла:

```bash
readelf -n /tmp/from-server.exec | grep 'Build ID'
file /tmp/from-server.debug
```

### 4. Заголовки и коды ответа

```bash
curl -sI "${DEBUGINFOD_URL}/buildid/${BUILDID}/debuginfo"
# Ожидание: HTTP/1.1 200, Content-Type: application/octet-stream
```

| Код | Значение |
|-----|----------|
| `200` | Артефакт найден |
| `404` | Build-id нет в индексе или тип не найден |
| `503` | Сервис не ready (`/readyz`) |

---

## Клиент debuginfod-find

Переменная окружения — **`DEBUGINFOD_URLS`** (как у upstream elfutils):

```bash
export DEBUGINFOD_URLS=http://127.0.0.1:8002

# Скачать debuginfo / executable
debuginfod-find debuginfo "$BUILDID" -o /tmp/out.debug
debuginfod-find executable "$BUILDID" -o /tmp/out.exec

# Исходник и секция
debuginfod-find source "$BUILDID" /usr/src/hello.c
debuginfod-find section "$BUILDID" .note.gnu.build-id

# Metadata
debuginfod-find --key file --value "$BIN"
debuginfod-find --key glob --value '/bin/*'
```

Флаг `--url` переопределяет `DEBUGINFOD_URLS` для одного вызова.

Совместимый CLI из этого репозитория: `make build-find` → `./debuginfod-find`.

---

## Тест через GDB

GDB (с libdebuginfod) автоматически запрашивает debuginfo по build-id stripped-бинарника.

### Минимальный сценарий

```bash
export DEBUGINFOD_URLS=http://127.0.0.1:8002

gdb /path/to/binary
```

В сессии GDB:

```gdb
(gdb) show debuginfod urls
(gdb) info files
(gdb) break main
(gdb) run
(gdb) backtrace
(gdb) info locals
(gdb) info functions
```

Если бинарник **stripped**, но debuginfo проиндексирован на сервере — в backtrace и `info functions` должны быть **имена символов**, а не только адреса.

### URL без переменной окружения

```gdb
set debuginfod urls http://127.0.0.1:8002
set debuginfod enabled on
```

На дистрибутивах с `/etc/debuginfod/*.urls` можно прописать URL в файл вместо `export`.

### Отладка загрузки (GDB ≥ 12)

```gdb
set debuginfod debug 1
```

### Критерии успеха

| Проверка | Ожидание |
|----------|----------|
| `show debuginfod urls` | URL вашего сервера |
| `info functions` | Имена функций (не пусто для stripped + debuginfo) |
| `backtrace` | Имена в стеке, не `??` |
| `curl .../buildid/.../debuginfo` | `200` до запуска GDB |

---

## Quik / dedup

Типичная раскладка под `DEBUGINFOD_SCAN_PATH`:

```text
Released/QuikServer_.../build_N_.../bin/quik          → executable (build-id)
Released/QuikServer_.../build_N_.../libfoo.so.*.debug → debuginfo
```

Цепочка проверки:

```bash
QUik=/home/ieme/debug_linux-go/Released/.../bin/quik

# 1. В индексе?
curl -s "${DEBUGINFOD_URL}/metadata?key=file&value=$(basename "$QUik")" | jq .

# 2. Build ID
BUILDID=$(readelf -n "$QUik" | awk '/Build ID/ {print $3}')

# 3. Debuginfo по HTTP
curl -f -I "${DEBUGINFOD_URL}/buildid/${BUILDID}/debuginfo"

# 4. GDB
export DEBUGINFOD_URLS=http://127.0.0.1:8002
gdb "$QUik"
```

После **dedup** (xdelta) сервер восстанавливает файл в локальный cache при запросе — GDB получает обычный ELF, клиент dedup не видит.

**Важно:** файлы Qt в `qt_debug/` **без GNU build-id** в индекс не попадают (`skip elf without build-id` в логе). Для GDB нужны бинарники Quik с build-id и соответствующие `.debug`.

Подробнее о хранении: [QUIK_DEDUP.md](./QUIK_DEDUP.md).

---

## Демо из репозитория

### Docker (GDB)

```bash
cd examples
make demo
```

### Локально (без Docker)

Терминал 1 — сервер:

```bash
make -C examples/sample
DEBUGINFOD_SCAN_PATH=$(pwd)/examples/sample/bin ./debuginfod -p 8002
```

Терминал 2 — GDB:

```bash
export DEBUGINFOD_URLS=http://localhost:8002
gdb -x examples/gdb/debug.gdb examples/sample/bin/hello
```

Скрипт `examples/gdb/debug.gdb` ставит breakpoint в `greet()`, выводит backtrace и locals — наглядная проверка загрузки символов через debuginfod.

### Delve (Go)

```bash
cd examples
make demo-delve
```

См. [GO_ECOSYSTEM.md](./GO_ECOSYSTEM.md).

---

## Устранение неполадок

| Симптом | Что проверить |
|---------|----------------|
| `404` на `/buildid/.../debuginfo` | Scan завершён? `curl readyz`. Есть `.debug` с тем же build-id? Путь в `DEBUGINFOD_SCAN_PATH`? |
| `readyz` → 503 | Дождаться `scan complete` в логе или Web UI → Сканирования |
| GDB не обращается к серверу | `echo $DEBUGINFOD_URLS`; `show debuginfod urls`; GDB собран с debuginfod (`apt install debuginfod`) |
| Символы `??` в backtrace | Неверный build-id; debuginfo не проиндексирован; кэш клиента — `rm -rf ~/.cache/debuginfod` |
| `metadata` пустой | Имя/путь не совпадает с индексом; попробовать `key=glob` или префикс build-id |
| Медленный первый запрос | Dedup restore / lazy extract из архива — нормально для больших `.debug` |
| Basic Auth / TLS | URL должен включать `https://` и credentials; `/healthz` без auth, API — с auth (см. README) |

Логи сервера:

```bash
journalctl -u debuginfod-go -f
# или stdout при ручном запуске ./debuginfod
```

Счётчик HTTP-запросов растёт при обращениях GDB:

```bash
curl -s http://127.0.0.1:8002/ui/api/stats | jq .http_requests_total
```

---

## Сравнение с upstream debuginfod

Порядок тестирования **тот же**:

1. `readelf -n` → build-id  
2. `curl` или `debuginfod-find` → `/buildid/<id>/debuginfo`  
3. `export DEBUGINFOD_URLS=...` → `gdb`

Отличия debuginfod-go: Web UI (`/ui/`), Zabbix (`/zabbix`), dedup Quik — на протокол GDB не влияют.

Полная таблица совместимости API: [DEVELOPMENT.md](../DEVELOPMENT.md#сравнение-с-upstream-debuginfod-elfutils).
