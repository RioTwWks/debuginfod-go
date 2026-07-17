# Quik debuginfo storage (zstd + CAS dedup)

Гибридное сжатие `.debug` ELF-файлов Quik/Front: **per-file zstd** + **SHA256 content-addressable dedup**.

xdelta **не используется** — инкрементальные патчи между сборками дают низкую экономию на DWARF.

## Layout на диске

Dedup рекурсивно ищет каталоги `build_*` под `DEBUGINFOD_SCAN_PATH` на любой глубине.
Имя «проекта» в UI — относительный путь от scan root до родителя `build_*`.

Пример:

```text
DEBUGINFOD_SCAN_PATH/
  Released/
    QuikServer_16.0_Common_Linux/
      build_482_2025-03-26_…/
        lib.so.19.1.5.2899.debug
```

Сжатые blob хранятся в CAS-каталоге (по умолчанию `{DEBUGINFOD_CACHE_DIR}/dedup-blobs/`):

```text
dedup-blobs/
  ab/
    abcdef…<sha256>.zst
```

Оригинальные `.debug` удаляются после успешного сжатия и проверки SHA256.

## Имена файлов

`lib.so.19.1.5.2899.debug`:

| Поле | Значение |
|------|----------|
| stem | `lib.so` |
| version | `19.1.5` |
| build_num | `2899` |

## Pipeline

Для каждого pending `.debug`:

1. Вычислить SHA256 содержимого.
2. Если blob с таким SHA уже есть → **CAS ref** (удалить оригинал, ссылка на существующий blob).
3. Иначе → **zstd-сжатие** в CAS, проверка распаковки, удаление оригинала.

Группировка по `(project, file_stem)` **не требуется** — каждый файл обрабатывается независимо.

### Ingest (после scan)

1. Рекурсивно обнаружить каталоги `build_*` под scan path.
2. Зарегистрировать `.debug` в таблице `dedup_files`.
3. Сжать все pending-файлы (zstd + CAS).

### Backfill (для старых сборок)

```http
POST /admin/dedup-backfill?project=Released/QuikServer_16.0_Common_Linux&batch=50&dry_run=false
X-Admin-Token: <DEBUGINFOD_ADMIN_KEY>
```

Параметр `project` — полный путь проекта как в UI. Без `project` — все обнаруженные папки.

При обновлении с xdelta legacy-записи (`storage_kind=base|delta`) автоматически сбрасываются в `pending` для повторной обработки.

## Отдача (cache-aside)

При запросе `/buildid/<id>/debuginfo`:

1. Найти `file_path` в `artifacts`.
2. Проверить `dedup_files`: если `storage_kind=compressed|ref`, распаковать blob в `DEBUGINFOD_CACHE_DIR/dedup-restored/`.
3. Отдать распакованный файл (`http.ServeFile`).

Повторные запросы используют кэш (проверка size + SHA256).

## БД

Таблицы `dedup_projects`, `dedup_build_dirs`, `dedup_files` — см. `internal/storage/dedup.go`.

`storage_kind`: `full` | `compressed` | `ref`.

Колонка `delta_path` хранит путь к zstd-blob (историческое имя).

## Конфигурация

```env
DEBUGINFOD_DEDUP_ENABLED=true
# Пусто = все найденные папки; иначе фильтр по полному пути проекта (через запятую):
# DEBUGINFOD_DEDUP_PROJECTS=Released/QuikServer_16.0_Common_Linux
DEBUGINFOD_DEDUP_WORKERS=4
# Каталог CAS-blob (по умолчанию: {DEBUGINFOD_CACHE_DIR}/dedup-blobs)
# DEBUGINFOD_DEDUP_BLOB_DIR=/var/lib/debuginfod-go/blobs
```

## Ограничения

- Только plain `.debug` на диске (не архивы).
- Внешние зависимости не требуются — zstd встроен в бинарник (klauspost/compress).
- Экономия CAS заметна только при **байт-идентичных** файлах между сборками; zstd даёт выигрыш на каждом файле.

## Отложено (build-time)

См. [TODO.md](../TODO.md) — оптимизации на стороне CI: `dwz`, `-gz`/`zlib-gnu`, split-dwarf.
