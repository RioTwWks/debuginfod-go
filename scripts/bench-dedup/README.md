# Strategy A — A/B-бенчмарк дифференциального сжатия Quik `.debug`

Офлайн-инструмент для сравнения **xdelta3**, **bsdiff** и **HDiffPatch** с опциональной предобработкой **dwz** и пост-сжатием base через **objcopy**.

Не требует запущенного `debuginfod-go` и не меняет production dedup pipeline.

## Что сравниваем

| Компонент | Варианты |
|-----------|----------|
| Алгоритм диффа | `xdelta3`, `bsdiff`, `hdiffpatch` |
| Предобработка | `none`, `dwz` (до создания дельт) |
| Пост-сжатие base | `--post-compress-base` → `objcopy --compress-debug-sections=zstd` **после** дельт |

**Группировка (Strategy A):** по умолчанию `(project, file_stem)` — как production dedup.  
Опционально `--group-by stem-version` или `strategy-a` (включает `commit_tag`; часто даёт 0 групп, если в `.comment` уникальные JIRA-теги).

**Метрики:** суммарный размер до/после, % экономии, время encode/decode, SHA256 после восстановления.

**Вне scope:** удаление debug-секций (`.debug_macro`, `.debug_types`).

---

## 1. Установка зависимостей

На машине с реальными `build_*` (Astra / Ubuntu / RedOS / CentOS):

```bash
# Ubuntu / Astra
sudo apt install xdelta3 bsdiff dwz binutils

# HDiffPatch — из исходников или пакета дистрибутива
# https://github.com/sisong/HDiffPatch
# после сборки: hdiffz, hpatchz в PATH

# RedOS / CentOS (имена пакетов могут отличаться)
sudo yum install xdelta bsdiff dwz binutils
```

Проверка:

```bash
make build-bench-dedup
./bench-dedup check-tools
```

Ожидаемый вывод (все `ok` для полного матричного прогона):

```text
xdelta3:         ok
bsdiff:          ok
hdiffpatch:      ok
dwz:             ok
objcopy:         ok
```

---

## 2. Подготовка данных

### 2.1 Scan path

Убедитесь, что путь совпадает с production:

```bash
export SCAN_PATH=/home/ieme/debug_linux   # ваш DEBUGINFOD_SCAN_PATH
```

Структура:

```text
$SCAN_PATH/Released/QuikServer_16.0_Common_Linux/build_1_.../*.debug
$SCAN_PATH/Released/QuikServer_16.0_Common_Linux/build_2_.../*.debug
...
```

### 2.2 Просмотр файлов и групп

```bash
# все .debug с метаданными (stem, version, commit_tag)
./bench-dedup list-files --scan-path "$SCAN_PATH" --project "$PROJECT" | head

# группы для диффа (по умолчанию --group-by stem)
./bench-dedup list-groups \
  --scan-path "$SCAN_PATH" \
  --project "Released/QuikServer_16.0_Common_Linux" \
  --min-files 2
```

Если `groups=0` при `files>0`, в stderr будет диагностика:
`singletons=72` → слишком строгая группировка. Используйте `--group-by stem` (default с этой версии).

Проверьте:

- группы с `files >= 2` (иначе дельты не создаются);
- разумный `commit_tag` в ключе (пустой тег — отдельная группа);
- суммарный объём близок к ожидаемому (~11 GB для вашей выборки).

### 2.3 (Опционально) Снимок для повторяемости

Для долгих прогонов скопируйте выборку на локальный SSD:

```bash
rsync -a --include='*/' --include='*.debug' --exclude='*' \
  "$SCAN_PATH/Released/QuikServer_16.0_Common_Linux/" \
  /tmp/quik-bench-sample/
export SCAN_PATH=/tmp/quik-bench-sample
```

---

## 3. Прогоны бенчмарка

Рабочий каталог должен быть на диске с достаточным местом (копии файлов + патчи):

```bash
export WORKDIR=/tmp/bench-dedup-$(date +%Y%m%d)
mkdir -p "$WORKDIR"
```

### 3.1 Фаза 1 — базовая матрица (без dwz)

Сравнение трёх алгоритмов на сырых `.debug`:

```bash
./bench-dedup \
  --scan-path "$SCAN_PATH" \
  --project "Released/QuikServer_16.0_Common_Linux" \
  --workdir "$WORKDIR/phase1" \
  --algos xdelta3,bsdiff,hdiffpatch \
  --preprocess none \
  --format json \
  --output "$WORKDIR/phase1-results.json"
```

Текстовый отчёт в консоль:

```bash
./bench-dedup \
  --scan-path "$SCAN_PATH" \
  --project "Released/QuikServer_16.0_Common_Linux" \
  --workdir "$WORKDIR/phase1" \
  --algos xdelta3,bsdiff,hdiffpatch \
  --preprocess none \
  --format text
```

**Ожидание:** xdelta3 ~11% (ваш эталон). bsdiff/HDiffPatch — гипотеза меньших патчей, проверяем фактом.

### 3.2 Фаза 2 — dwz перед диффом

```bash
./bench-dedup \
  --scan-path "$SCAN_PATH" \
  --project "Released/QuikServer_16.0_Common_Linux" \
  --workdir "$WORKDIR/phase2" \
  --algos xdelta3,bsdiff,hdiffpatch \
  --preprocess dwz \
  --format json \
  --output "$WORKDIR/phase2-dwz-results.json"
```

**Порядок критичен:** `dwz` применяется на копии каждого файла **до** encode.

Если `dwz` падает на конкретном ELF — сценарий завершится с ошибкой по группе; сохраните путь файла из JSON (`errors`).

### 3.3 Фаза 3 — полная матрица (none + dwz)

```bash
./bench-dedup \
  --scan-path "$SCAN_PATH" \
  --project "Released/QuikServer_16.0_Common_Linux" \
  --workdir "$WORKDIR/phase3-full" \
  --algos xdelta3,bsdiff,hdiffpatch \
  --preprocess none,dwz \
  --format csv \
  --output "$WORKDIR/phase3-full.csv"
```

### 3.4 Фаза 4 — objcopy zstd на base (после дельт)

Только для **победителя** фаз 1–3. Флаг `--post-compress-base` сжимает base-файл после расчёта дельт:

```bash
./bench-dedup \
  --scan-path "$SCAN_PATH" \
  --project "Released/QuikServer_16.0_Common_Linux" \
  --workdir "$WORKDIR/phase4" \
  --algos xdelta3 \
  --preprocess dwz \
  --post-compress-base \
  --format json \
  --output "$WORKDIR/phase4-objcopy.json"
```

**Не комбинировать** `--post-compress-base` с диффом по уже сжатым файлам — сжатие применяется только к base в хранилище, не перед encode.

### 3.5 Быстрый smoke на подвыборке

Перед полным прогоном (~часы, много RAM на bsdiff):

```bash
./bench-dedup \
  --scan-path "$SCAN_PATH" \
  --project "Released/QuikServer_16.0_Common_Linux" \
  --workdir "$WORKDIR/smoke" \
  --algos xdelta3,bsdiff \
  --preprocess none,dwz \
  --max-groups 5 \
  --format text
```

---

## 4. Проверка корректности

Инструмент автоматически:

1. Создаёт патч `base → target`
2. Восстанавливает файл через decode
3. Сравнивает SHA256 восстановленного с оригиналом (после preprocess)

Дополнительно вручную (на 1–2 файлах):

```bash
# GDB — файл из workdir (если --keep-workdir)
gdb -batch -ex "info sources" /path/to/restored.debug

# readelf — целостность ELF после dwz
readelf -S file.debug | head -20
```

Критерий успеха фазы: `verify_failures=0`, `errors=0` в summary.

---

## 5. Интерпретация результатов

В JSON смотрите `scenarios[].summary`:

| Поле | Смысл |
|------|-------|
| `original_total` | Сумма размеров всех `.debug` в группах |
| `stored_total` | base + сумма патчей (± сжатый base) |
| `savings_pct` | `(1 - stored/original) * 100` |
| `encode_total_ms` / `decode_total_ms` | Суммарное CPU-время |
| `verify_failures` | Должно быть 0 |

Сравнительная таблица (пример):

| Сценарий | savings % | encode ms | decode ms |
|----------|-----------|-----------|-----------|
| xdelta3 + none | ~11 | … | … |
| bsdiff + none | ? | … | … |
| xdelta3 + dwz | ? | … | … |

**Выбор победителя:** максимальный `savings_pct` при `verify_failures=0` и приемлемом decode time для production restore.

---

## 6. Ограничения и риски

| Риск | Митигация |
|------|-----------|
| bsdiff RAM ~17× размер файла | smoke с `--max-groups`; мониторить `free -h` |
| dwz не на всех ELF | логировать ошибки; отдельный список проблемных файлов |
| HDiffPatch не в репозитории | `check-tools`; пропуск с `skipped` в отчёте |
| Долгий прогон | `--max-groups`, rsync на SSD, ночной запуск |

---

## 7. Следующие шаги после бенчмарка

1. Зафиксировать победителя (algo + preprocess) в issue/PR с JSON-отчётом
2. Интегрировать в `internal/dedup` (group-based pipeline, restore chain)
3. Обновить `docs/QUIK_DEDUP.md` и env-конфиг
4. Стратегии B/C — по результатам A

---

## Справка по CLI

```bash
./bench-dedup check-tools [--xdelta3 PATH] [--bsdiff PATH] ...

./bench-dedup list-groups --scan-path PATH [--project P] [--group-by stem|stem-version|strategy-a] [--min-files N]

./bench-dedup list-files --scan-path PATH [--project P] [--max-files N]

./bench-dedup \
  --scan-path PATH \
  --workdir DIR \
  [--project P] \
  [--group-by stem] \
  [--algos xdelta3,bsdiff,hdiffpatch] \
  [--preprocess none,dwz] \
  [--post-compress-base] \
  [--max-groups N] [--max-files N] \
  [--keep-workdir] \
  [--format text|json|csv] \
  [--output FILE]
```

Сборка: `make build-bench-dedup` или `go build -o bench-dedup ./cmd/bench-dedup`
