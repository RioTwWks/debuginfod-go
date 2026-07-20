# Strategy A — сводка сравнения подходов (Quik .debug)

> Полный документ: [docs/DEDUP_STRATEGY_COMPARISON.md](../../docs/DEDUP_STRATEGY_COMPARISON.md)

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

## Полная матрица (все стратегии)

Один прогон **11 сценариев** (3 algo × 3 preprocess + 2 objcopy) или **15** с `--extended`:

```bash
export SCAN_PATH=/home/ieme/debug_linux
export PROJECT="Released/QuikServer_16.0_Common_Linux"
export WORKDIR=/tmp/bench-dedup-matrix-$(date +%Y%m%d)

# Базовая матрица (~30–60 мин, bsdiff медленный)
./scripts/bench-dedup/run-full-matrix.sh

# + сравнение group-by stem-version / strategy-a (4 доп. сценария)
EXTENDED=1 ./scripts/bench-dedup/run-full-matrix.sh
```

Или напрямую:

```bash
./bench-dedup run-matrix \
  --scan-path "$SCAN_PATH" \
  --project "$PROJECT" \
  --workdir "$WORKDIR" \
  --output "$WORKDIR/matrix"
# → matrix.json, matrix.csv, matrix.txt
```

### Сценарии DefaultMatrix (11)

| ID | algo | preprocess | post-zstd |
|----|------|------------|-----------|
| xdelta3_none | xdelta3 | none | — |
| xdelta3_dwz | xdelta3 | dwz | — |
| xdelta3_decompress-dwz | xdelta3 | decompress-dwz | — |
| bsdiff_none | bsdiff | none | — |
| bsdiff_dwz | bsdiff | dwz | — |
| bsdiff_decompress-dwz | bsdiff | decompress-dwz | — |
| hdiffpatch_none | hdiffpatch | none | — |
| hdiffpatch_dwz | hdiffpatch | dwz | — |
| hdiffpatch_decompress-dwz | hdiffpatch | decompress-dwz | — |
| xdelta3_none_objcopy | xdelta3 | none | base |
| xdelta3_decompress-dwz_objcopy | xdelta3 | decompress-dwz | base |

### Extended (+4): xdelta3 по режимам группировки

- `xdelta3_none_stem-version`
- `xdelta3_none_strategy-a`
- `xdelta3_decompress-dwz_stem-version`
- `xdelta3_decompress-dwz_strategy-a`

**Вне scope bench-dedup:** Strategy C (секционный CAS), zstd whole-file CAS, debuginfod path interning.

## Рекомендуемая матрица прогонов (ручные отдельные тесты)

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

## Итоговая матрица (2026-07-20, 72 файла, group-by stem)

| ID | savings | stored | encode | decode | verify | Вердикт |
|----|---------|--------|--------|--------|--------|---------|
| **xdelta3_decompress-dwz_objcopy** | **76.0%** | 139.6 MiB | 32 с | 4.8 с | 0 | **Максимум** |
| bsdiff_decompress-dwz | 68.7% | 182.6 MiB | 417 с | — | **63** | Отпадает |
| **xdelta3_decompress-dwz** | **55.1%** | 261.7 MiB | 33 с | 4.9 с | 0 | **Рекомендуется** |
| xdelta3_none_objcopy | 18.4% | 475.9 MiB | 72 с | 1.5 с | 0 | Маргинально |
| xdelta3_none | 17.2% | 482.4 MiB | 70 с | 1.5 с | 0 | Простой вариант |
| bsdiff_none | 17.0% | 483.8 MiB | 732 с | — | **63** | Отпадает |
| xdelta3_dwz / bsdiff_dwz | 0% | — | — | — | 9 err | Сжатый DWARF |
| hdiffpatch_* | — | — | — | — | — | Не установлен |

Базовый объём без dedup: **583.1 MiB**.

## Итоговая рекомендация

1. **Максимум (76%):** `decompress → dwz → xdelta3` + `objcopy zstd` на base
2. **Баланс (55%):** `decompress → dwz → xdelta3` — проще restore для GDB
3. **Минимум (17%):** `xdelta3` без preprocess
4. **Не использовать:** bsdiff, dwz на сжатых файлах
