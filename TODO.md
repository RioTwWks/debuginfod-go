# TODO — план развития debuginfod-go

Список возможных улучшений, сгруппированных по приоритету. Отмечайте выполненное, меняя `[ ]` на `[x]`.

---

## Высокий приоритет (совместимость с elfutils debuginfod)

- [ ] **`/buildid/<id>/section/<name>`** — отдача сырого содержимого ELF-секции (сначала из debuginfo, иначе из executable)
- [ ] **Точный `fnmatch` для metadata** — поведение как `FNM_PATHNAME` в glibc, а не только `filepath.Match`
- [ ] **Лимит времени metadata-запросов** — флаг `--metadata-maxtime` (по умолчанию 5 с, как в upstream)
- [ ] **Инкрементальная индексация** — переиндексировать только изменённые файлы (по `mtime`/хешу), а не весь каталог
- [ ] **Поддержка `.tar.zst` в `.deb`** — сейчас обрабатываются `.tar.gz` и `.tar.xz`
- [ ] **Интеграционные тесты HTTP** — end-to-end: поднять сервер, загрузить ELF, проверить `/buildid` и `/metadata`

## Средний приоритет (эксплуатация и производительность)

- [ ] **Структурированное логирование** — `log/slog` с уровнем из `DEBUGINFOD_LOG_LEVEL`
- [ ] **Метрики Prometheus** — `/metrics`: число артефактов, длительность scan, HTTP latency
- [ ] **Ограничение размера кэша** — ротация/очистка `DEBUGINFOD_CACHE_DIR` по LRU или лимиту в ГБ
- [ ] **Параллельный scan** — worker pool при обходе больших деревьев каталогов
- [ ] **Федерация** — опрос upstream-серверов из `DEBUGINFOD_URLS` при отсутствии артефакта локально
- [ ] **PostgreSQL** — опциональный backend вместо SQLite для production-кластера
- [ ] **systemd unit** — пример `debuginfod-go.service` для деплоя без Docker
- [ ] **Сжатие HTTP-ответов** — `gzip` middleware для крупных debuginfo-файлов

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
