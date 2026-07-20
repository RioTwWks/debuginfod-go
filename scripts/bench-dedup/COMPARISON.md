# Strategy A — сводка сравнения подходов (Quik .debug)

Данные: `Released/QuikServer_16.0_Common_Linux`, 72 файла, 9 библиотек × 8 сборок.

## Структура `.comment` (типичный файл)

```text
GCC: (AstraLinuxSE 8.3.0-6) 8.3.0
(c) ARQA Technologies, 2000-2026
DBSTUBRC Library
Quik Server
16.0.0.1                              ← версия продукта (не git)
9ae10425c6bbb99c7ee1f71a3941fd7aee058227  ← git commit SHA
```

Проверка одного файла:

```bash
./bench-dedup inspect-file --path "$FILE"
readelf -p .comment "$FILE"
```

## Группировка для диффа

| `--group-by` | Ключ | Когда использовать |
|--------------|------|-------------------|
| **stem** (default) | project + file_stem | Межсборочные дельты одной `.so` — **ваш случай** |
| stem-version | + version из имени | Если одна stem встречается с разными M.m.p |
| strategy-a | + git commit из `.comment` | Файлы **одного коммита** (редко между build_*) |

## Результаты smoke (5 групп, 40 файлов)

| Сценарий | savings | verify | encode | Вердикт |
|----------|---------|--------|--------|---------|
| **xdelta3 + none** | ~21% | 0 fail | ~40 s | **Рекомендуется** |
| xdelta3 + dwz | — | 5 errors | — | **Не работает**: сжатый `.debug_aranges` |
| xdelta3 + decompress-dwz | ? | ? | ? | Прогнать отдельно (см. ниже) |
| bsdiff + none | ~21% | 35 fail | ~6 min | **Отпадает** |
| hdiffpatch | — | — | — | Не установлен |

Phase1 (все 9 групп, xdelta3): **~18–23%** на библиотеку, verify OK.

## Почему dwz «не работает»

Quik `.debug` уже содержит **сжатые DWARF-секции** (`SHF_COMPRESSED`).

```text
dwz: Found compressed .debug_aranges section, not attempting dwz compression
```

| Подход | Описание |
|--------|----------|
| `dwz` | Сразу отказ на ingest |
| `decompress-dwz` | `objcopy --decompress-debug-sections` → `dwz` → diff (эксперимент) |
| build-time dwz | До сжатия секций в CI — вне scope server-side |

## Рекомендуемая матрица прогонов

```bash
export WORKDIR=/tmp/bench-dedup-final
mkdir -p "$WORKDIR"

# 1. Baseline — главный кандидат
./bench-dedup --scan-path "$SCAN_PATH" --project "$PROJECT" \
  --workdir "$WORKDIR/xdelta-none" \
  --algos xdelta3 --preprocess none \
  --format json --output "$WORKDIR/01-xdelta-none.json"

# 2. dwz после распаковки (честное сравнение)
./bench-dedup --scan-path "$SCAN_PATH" --project "$PROJECT" \
  --workdir "$WORKDIR/xdelta-decompress-dwz" \
  --algos xdelta3 --preprocess decompress-dwz \
  --format json --output "$WORKDIR/02-xdelta-decompress-dwz.json"

# 3. objcopy zstd на base после дельт
./bench-dedup --scan-path "$SCAN_PATH" --project "$PROJECT" \
  --workdir "$WORKDIR/xdelta-objcopy" \
  --algos xdelta3 --preprocess none --post-compress-base \
  --format json --output "$WORKDIR/03-xdelta-objcopy-base.json"
```

## Итоговая рекомендация (предварительно)

1. **Production:** `xdelta3` + группировка `stem` (~20% на ваших данных)
2. **dwz:** только `decompress-dwz` в бенчмарке; для prod — build-time
3. **bsdiff:** не использовать (verify failures)
4. **git commit** в метаданных: полный SHA из `.comment`, не `16.0.0.1`
