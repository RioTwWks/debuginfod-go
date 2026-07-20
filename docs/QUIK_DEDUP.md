# Quik debuginfo storage (xdelta3 + decompress-dwz)

Инкрементальное сжатие `.debug` ELF-файлов Quik/Front: **decompress → dwz → xdelta3** с опциональным **objcopy zstd** на base.

Подробное сравнение стратегий: [DEDUP_STRATEGY_COMPARISON.md](./DEDUP_STRATEGY_COMPARISON.md).

## Layout на диске

Dedup рекурсивно ищет каталоги `build_*` под `DEBUGINFOD_SCAN_PATH` на любой глубине.
Имя «проекта» в UI — относительный путь от scan root до родителя `build_*`.

Пример:

```text
DEBUGINFOD_SCAN_PATH/
  Released/
    QuikServer_16.0_Common_Linux/
      build_482_2025-03-26_…/
        lib.so.19.1.5.2899.debug          ← base (после preprocess)
        lib.so.19.1.5.2900.debug.xdelta   ← delta (оригинал удалён)
```

## Имена файлов

В dedup попадает **любой** файл с суффиксом `.debug` внутри `build_*`.

Для известных Quik-шаблонов (`lib.so.M.m.p.BUILD.debug`, `quik-M.m.p.BUILD.debug`) в БД сохраняются stem/version/build. Для произвольных имён — только имя файла без расширения.

## Pipeline

Группировка: `NormalizeDedupGroupProject(project) + file_stem`, base = min `file_build_num`.

Для каждой группы из 2+ файлов:

1. `objcopy --decompress-debug-sections` + `dwz` на base (in-place).
2. Для каждого target: preprocess → `xdelta3 -e` → verify decode → удалить оригинал.
3. Опционально: `objcopy --compress-debug-sections=zstd` на base (`DEBUGINFOD_DEDUP_COMPRESS_BASE=true`).

Singleton-группы: `storage_kind=full` без изменений.

### Ingest (после scan)

1. Рекурсивно обнаружить каталоги `build_*` под scan path.
2. Зарегистрировать `.debug` в таблице `dedup_files`.
3. Обработать pending-файлы группами (xdelta pipeline).

### Backfill (для старых сборок)

```http
POST /admin/dedup-backfill?project=Released/QuikServer_16.0_Common_Linux&batch=50&dry_run=false
X-Admin-Token: <DEBUGINFOD_ADMIN_KEY>
```

При первом запуске после обновления с zstd CAS однократная миграция сбрасывает `compressed`/`ref`/`base`/`delta` → `pending`.

## Отдача (cache-aside)

При запросе `/buildid/<id>/debuginfo`:

| storage_kind | Действие |
|--------------|----------|
| `full`, `base` | отдать `file_path` (GDB читает zstd-секции нативно) |
| `delta` | decompress base (если zstd) → `xdelta3 -d` → кэш |
| `compressed`/`ref` | legacy zstd CAS: распаковка blob |

## Конфигурация

```bash
DEBUGINFOD_DEDUP_ENABLED=true
DEBUGINFOD_DEDUP_WORKERS=4
DEBUGINFOD_DEDUP_STRATEGY=xdelta-decompress-dwz   # или xdelta (без dwz)
DEBUGINFOD_DEDUP_COMPRESS_BASE=true               # objcopy zstd на base
DEBUGINFOD_XDELTA_PATH=xdelta3
DEBUGINFOD_DWZ_PATH=dwz
DEBUGINFOD_OBJCOPY_PATH=objcopy
```

Группы `(project, file_stem)` обрабатываются параллельно (`DEBUGINFOD_DEDUP_WORKERS`).

Зависимости: `xdelta3`, `dwz`, `binutils` (`objcopy`).

## БД

Таблицы `dedup_projects`, `dedup_build_dirs`, `dedup_files`, `dedup_meta` — см. `internal/storage/dedup.go`.

`storage_kind`: `full` | `base` | `delta` | `compressed` | `ref` (legacy).

## Бенчмарки

```bash
make build-bench-dedup
./scripts/bench-dedup/run-full-matrix.sh
```

См. [scripts/bench-dedup/README.md](../scripts/bench-dedup/README.md).
