# TODO — план развития debuginfod-go

Список улучшений по приоритету. Выполненное — `[x]`.

**Статус проекта (2026-07-14):** MVP + elfutils-совместимость + эксплуатация (Zabbix, federation, PostgreSQL) + архивы (deb/rpm/tar/lazy/SRPM) + Web UI.

## Целевое развёртывание

Эксплуатация только на **Astra Linux**, **Ubuntu**, **RedOS**, **CentOS**.

| ОС | Пакеты в приоритете |
|----|---------------------|
| Astra Linux, Ubuntu | `.deb`, `.dsc`, `/usr/lib/debug`, plain tar |
| RedOS, CentOS | `.rpm`, `.src.rpm`, `/usr/lib/debug`, plain tar |

`.apk` и `.pacman` / `.pkg.tar.*` **не поддерживаются** — только deb/rpm-стек целевых ОС.

---

## Высокий приоритет (совместимость с elfutils debuginfod)

- [x] **`/buildid/<id>/section/<name>`** — сырые ELF-секции
- [x] **Точный `fnmatch` для metadata** — `FNM_PATHNAME`
- [x] **Лимит времени metadata** — `--metadata-maxtime` (5 с)
- [x] **Инкрементальная индексация** — `scanned_files` по mtime/size
- [x] **`.tar.zst` в `.deb`**
- [x] **Интеграционные тесты HTTP**

## Средний приоритет (эксплуатация и производительность)

- [x] **Структурированное логирование** — `log/slog`
- [x] **Метрики для Zabbix** — `/zabbix` (вместо Prometheus)
- [x] **Ограничение размера кэша** — LRU `DEBUGINFOD_CACHE_DIR`
- [x] **Параллельный scan** — `DEBUGINFOD_SCAN_WORKERS`
- [x] **Федерация** — `DEBUGINFOD_URLS` при 404
- [x] **PostgreSQL** — `DEBUGINFOD_DATABASE_URL`
- [x] **systemd unit** — `deploy/debuginfod-go.service`
- [x] **Сжатие HTTP** — gzip middleware

## Архивы и форматы пакетов

### Целевые (Astra / Ubuntu / RedOS / CentOS)

- [x] **`.deb`** — Debian/Ubuntu/Astra
- [x] **`.rpm`** — RedOS/CentOS
- [x] **Plain tar/tar.gz/tar.xz** — каталоги debuginfo
- [x] **Отложенное извлечение** — `DEBUGINFOD_LAZY_EXTRACT`
- [x] **Индексация исходников из SRPM/DSC**

### Не поддерживается (вне целевых ОС)

- [x] ~~**`.apk` (Alpine)**~~ — удалено, не в scope
- [x] ~~**`.pacman` / `.pkg.tar.zst` (Arch)**~~ — удалено, не в scope

## API и клиенты

- [ ] **CLI `debuginfod-find`** — обёртка над HTTP API
- [ ] **Пагинация metadata** — cursor/offset
- [ ] **CORS и rate limiting**
- [ ] **Аутентификация** — Basic Auth / mTLS
- [ ] **OpenAPI/Swagger** — `docs/openapi.yaml`

## Go-экосистема

- [ ] **Совместимость с `go tool buildid`** — документировать маппинг
- [ ] **Delve integration** — пример `DEBUGINFOD_URLS`
- [ ] **`-buildmode=pie` и внешний линкер** — тесты GNU build-id

## Качество и CI

- [ ] **golangci-lint в CI**
- [ ] **Coverage badge** — Codecov
- [ ] **Benchmark-тесты**
- [ ] **Fuzzing** — ELF notes, ar/tar
- [ ] **Кросс-компиляция** — GOOS/GOARCH матрица

## Безопасность

- [ ] **Валидация путей** — path traversal
- [ ] **Лимит размера архива**
- [ ] **IMA/подписи**

## Документация

- [ ] **Примеры в `examples/`** — docker-compose, GDB-скрипт
- [x] **Диаграмма потока данных** — mermaid в DEVELOPMENT.md
- [x] **Сравнение с upstream debuginfod** — таблица в DEVELOPMENT.md

---

## Идеи «на будущее»

- [x] **Web UI** — `/ui/` дашборд: статистика, поиск по build-id
- [ ] **Webhook при завершении scan**
- [ ] **S3/MinIO backend**
- [ ] **Kubernetes Helm chart**
- [ ] **Плагинная система форматов архивов**

---

*Последнее обновление: 2026-07-14*
