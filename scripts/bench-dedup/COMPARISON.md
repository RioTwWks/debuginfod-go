# Strategy A — краткая сводка

> **Полный документ:** [docs/DEDUP_STRATEGY_COMPARISON.md](../../docs/DEDUP_STRATEGY_COMPARISON.md)

## Итог (72 файла Quik, 2026-07-20)

| Сценарий | Savings | Verify | Вердикт |
|----------|---------|--------|---------|
| **xdelta3 + decompress-dwz + zstd base** | **76.0%** | 0 | **Production** (`internal/dedup`) |
| xdelta3 + decompress-dwz | 55.1% | 0 | Fallback без zstd base |
| xdelta3 без preprocess | 17.2% | 0 | `DEBUGINFOD_DEDUP_STRATEGY=xdelta` |
| bsdiff | 17–69% | **63 fail** | Отклонён |
| zstd whole-file CAS | ~1.8% | OK | Отклонён |

## Быстрый прогон

```bash
make build-bench-dedup
export SCAN_PATH=/path/to/debug_linux
export PROJECT="Released/QuikServer_16.0_Common_Linux"
./scripts/bench-dedup/run-full-matrix.sh
```

Методика: [README.md](./README.md).
