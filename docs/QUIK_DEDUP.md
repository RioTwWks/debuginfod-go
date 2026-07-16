# Quik debuginfo dedup (xdelta3)

Сжатие дублирующихся `.debug` ELF-файлов Quik/Front через xdelta3 с группировкой по commit tag из секции `.comment`.

## Layout на диске

```text
DEBUGINFOD_SCAN_PATH/
  QuikServer/
    build_482_2025-03-26_…/
      lib.so.19.1.5.2899.debug
  Front/
    build_*/*.debug
```

Файлы уже приходят как `*.debug` (без `.7zip.debug`).

## Имена файлов

`lib.so.19.1.5.2899.debug`:

| Поле | Значение |
|------|----------|
| stem | `lib.so` |
| version | `19.1.5` |
| build_num | `2899` |

Каталог сборки: `build_<num>_YYYY-MM-DD_…` — `num` используется только для идентификации каталога.

## Группировка

Ключ группы: `(project, file_stem, version, commit_tag_id)`.

`commit_tag_id` извлекается из ELF-секции `.comment` (например `DEVOPS-110`).

Внутри группы **base** — файл с минимальным `build_num` в имени. Остальные кодируются как delta относительно base.

## Pipeline

### Ingest (после scan)

1. Обнаружить новые каталоги `build_*` в проектах из `DEBUGINFOD_DEDUP_PROJECTS`.
2. Зарегистрировать `.debug` в таблице `dedup_files`.
3. Сгруппировать pending-файлы и выполнить xdelta.

### Backfill (обязателен для старых сборок)

```http
POST /admin/dedup-backfill?project=QuikServer&batch=50&dry_run=false
X-Admin-Token: <DEBUGINFOD_ADMIN_KEY>
```

Обрабатывает каталоги `build_*` со статусом `pending` порциями.

### xdelta3

```bash
xdelta3 -e -s base.debug target.debug target.debug.xdelta
xdelta3 -d -s base.debug target.debug.xdelta restored.debug
```

После encode: SHA256(restored) == SHA256(original) → удалить original, сохранить `.xdelta`.

## Отдача (cache-aside)

При запросе `/buildid/<id>/debuginfo`:

1. Найти `file_path` в `artifacts`.
2. Проверить `dedup_files`: если `storage_kind=delta`, восстановить `base + delta` в `DEBUGINFOD_CACHE_DIR/dedup-restored/`.
3. Отдать восстановленный файл (`http.ServeFile`).

Повторные запросы используют кэш (проверка mtime/size delta и base).

## БД

Таблицы `dedup_projects`, `dedup_build_dirs`, `dedup_files` — см. `internal/storage/dedup.go`.

`storage_kind`: `full` | `base` | `delta`.

## Конфигурация

```env
DEBUGINFOD_DEDUP_ENABLED=true
DEBUGINFOD_DEDUP_PROJECTS=QuikServer,Front
DEBUGINFOD_DEDUP_WORKERS=4
DEBUGINFOD_XDELTA_PATH=xdelta3
```

## Ограничения

- Только plain `.debug` на диске (не архивы).
- xdelta эффективен для Quik при группировке по одному commit tag; для других проектов dedup можно отключить.
- Требуется `xdelta3` в PATH (пакет ОС: Astra/RedOS).
