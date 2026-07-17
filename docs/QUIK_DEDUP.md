# Quik debuginfo dedup (xdelta3)

Сжатие дублирующихся `.debug` ELF-файлов Quik/Front через xdelta3 с группировкой по commit tag из секции `.comment`.

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

В Web UI такой проект отобразится как `Released/QuikServer_16.0_Common_Linux`.

Классический layout тоже поддерживается:

```text
DEBUGINFOD_SCAN_PATH/
  QuikServer/
    build_482_2025-03-26_…/
      lib.so.19.1.5.2899.debug
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

Ключ группы: `(project, file_stem, commit_tag_id)`.

`version` из имени файла **не** входит в ключ: между `build_1` и `build_2` версия
в имени меняется (`16.0.0` → `16.0.1`), но delta строится для одной и той же библиотеки.

`commit_tag_id` извлекается из ELF-секции `.comment` (например `DEVOPS-110`).
Если JIRA-тега нет (типично для Quik: только `GCC:` / `ARQA` / `QUIKDB Library`),
группировка выполняется по `(project, file_stem)` с пустым тегом.

Внутри группы **base** — файл с минимальным `build_num` в имени. Остальные кодируются как delta относительно base.

## Pipeline

### Ingest (после scan)

1. Рекурсивно обнаружить каталоги `build_*` под scan path.
2. Зарегистрировать `.debug` в таблице `dedup_files`.
3. Сгруппировать pending-файлы и выполнить xdelta.

### Backfill (обязателен для старых сборок)

```http
POST /admin/dedup-backfill?project=Released/QuikServer_16.0_Common_Linux&batch=50&dry_run=false
X-Admin-Token: <DEBUGINFOD_ADMIN_KEY>
```

Параметр `project` — полный путь проекта как в UI. Без `project` — все обнаруженные папки.

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
# Пусто = все найденные папки; иначе фильтр по полному пути проекта (через запятую):
# DEBUGINFOD_DEDUP_PROJECTS=Released/QuikServer_16.0_Common_Linux
DEBUGINFOD_DEDUP_WORKERS=4
DEBUGINFOD_XDELTA_PATH=xdelta3
```

## Ограничения

- Только plain `.debug` на диске (не архивы).
- xdelta эффективен для Quik при группировке по одному commit tag; для других проектов dedup можно отключить.
- Требуется `xdelta3` в PATH (пакет ОС: Astra/RedOS).
