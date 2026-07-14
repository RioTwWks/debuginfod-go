# TODO — план развития debuginfod-go

Список возможных улучшений, сгруппированных по приоритету. Отмечайте выполненное, меняя `[ ]` на `[x]`.

---

## Высокий приоритет (совместимость с elfutils debuginfod)

- [x] **`/buildid/<id>/section/<name>`** — отдача сырого содержимого ELF-секции (сначала из debuginfo, иначе из executable)
- [x] **Точный `fnmatch` для metadata** — поведение как `FNM_PATHNAME` в glibc, а не только `filepath.Match`
- [x] **Лимит времени metadata-запросов** — флаг `--metadata-maxtime` (по умолчанию 5 с, как в upstream)
- [x] **Инкрементальная индексация** — переиндексировать только изменённые файлы (по `mtime`/размеру), а не весь каталог
- [x] **Поддержка `.tar.zst` в `.deb`** — сейчас обрабатываются `.tar.gz`, `.tar.xz` и `.tar.zst`
- [x] **Интеграционные тесты HTTP** — end-to-end: поднять сервер, загрузить ELF, проверить `/buildid` и `/metadata`

## Средний приоритет (эксплуатация и производительность)

- [x] **Структурированное логирование** — `log/slog` с уровнем из `DEBUGINFOD_LOG_LEVEL`
- [x] **Метрики для Zabbix** — `/zabbix` JSON endpoint для HTTP agent (вместо Prometheus)
- [x] **Ограничение размера кэша** — ротация `DEBUGINFOD_CACHE_DIR` по LRU (старые файлы)
- [x] **Параллельный scan** — worker pool (`DEBUGINFOD_SCAN_WORKERS`)
- [x] **Федерация** — опрос upstream из `DEBUGINFOD_URLS` при 404
- [x] **PostgreSQL** — `DEBUGINFOD_DATABASE_URL` как альтернатива SQLite
- [x] **systemd unit** — `deploy/debuginfod-go.service`
- [x] **Сжатие HTTP-ответов** — gzip middleware

## Архивы и форматы пакетов

- [ ] **`.apk` (Alpine)** — индексация ELF из apk-архивов
- [ ] **`.pacman` / `.pkg.tar.zst`** — пакеты Arch Linux
- [ ] **Plain tar/tar.gz/tar.xz** — каталоги с отладочными символами без обёртки deb/rpm
- [ ] **Отложенное извлечение** — не кэшировать ELF при индексации, извлекать из архива по запросу
- [ ] **Индексация исходников из SRPM/DSC** — не только бинарные артефакты

## API и клиенты

- [ ] **CLI `debuginfod-find`** — тонкая обёртка над HTTP API для ручной отладки
- [ ] **Пагинация metadata** — `complete: false` + cursor/offset для больших выборок
- [ ] **CORS и rate limiting** — для публичных инсталляций
- [ ] **Аутентификация** — Basic Auth или mTLS для закрытых серверов
- [ ] **OpenAPI/Swagger** — описание HTTP API в `docs/openapi.yaml`

## Go-экосистема

- [ ] **Совместимость с `go tool buildid`** — документировать маппинг raw → SHA-256 hex
- [ ] **Delve integration** — пример `DEBUGINFOD_URLS` для отладки Go-бинарников
- [ ] **`-buildmode=pie` и внешний линкер** — тесты GNU build-id у Go с `-ldflags=-linkmode=external`

## Качество и CI

- [ ] **golangci-lint в CI** — добавить job в GitHub Actions
- [ ] **Coverage badge** — `go test -coverprofile` + Codecov
- [ ] **Benchmark-тесты** — `BenchmarkParseBuildID`, `BenchmarkScan`
- [ ] **Fuzzing** — `go test -fuzz` для парсера ELF notes и ar/tar
- [ ] **Кросс-компиляция** — матрица GOOS/GOARCH в CI (без CGO где возможно)

## Безопасность

- [ ] **Валидация путей** — защита от path traversal при `source` и cache
- [ ] **Лимит размера архива** — не распаковывать гигантские rpm/deb целиком в память
- [ ] **IMA/подписи** — опциональная верификация загрузок (как в elfutils 0.192+)

## Документация

- [ ] **Примеры в `examples/`** — docker-compose с тестовым пакетом, скрипт проверки GDB
- [ ] **Диаграмма потока данных** — scan → SQLite → HTTP в DEVELOPMENT.md
- [ ] **Сравнение с upstream debuginfod** — таблица поддерживаемых фич

---

## Идеи «на будущее»

- [ ] Web UI — простой дашборд: статистика индекса, поиск по build-id
- [ ] Webhook при завершении scan — уведомление CI/мониторинга
- [ ] S3/MinIO backend — хранение артефактов в объектном хранилище
- [ ] Kubernetes Helm chart
- [ ] Плагинная система для новых форматов архивов

---

*Последнее обновление: 2026-07-13*
