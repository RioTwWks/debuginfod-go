# TODO — план развития debuginfod-go

Список улучшений по приоритету. Выполненное — `[x]`.

**Статус проекта (2026-07-14):** MVP + elfutils-совместимость + эксплуатация (Zabbix, federation, PostgreSQL) + архивы (deb/rpm/tar/lazy/SRPM) + Web UI + API и клиенты + examples.

## Целевое развёртывание

Эксплуатация только на **Astra Linux**, **Ubuntu**, **RedOS**, **CentOS**.

| ОС | Пакеты в приоритете |
|----|---------------------|
| Astra Linux, Ubuntu | `.deb`, `.dsc`, `/usr/lib/debug`, plain tar |
| RedOS, CentOS | `.rpm`, `.src.rpm`, `/usr/lib/debug`, plain tar |

`.apk` и `.pacman` / `.pkg.tar.*` **не поддерживаются** — только deb/rpm-стек целевых ОС.

### Модель эксплуатации (без Kubernetes)

**Kubernetes / Helm не используются** — нет возможности и нет выгоды для одного HTTP-демона.

Целевой стек DevOps:

```text
CI (GitHub Actions) → .deb / .rpm → Ansible → systemd + nginx → Zabbix
```

| Слой | Инструмент | Статус |
|------|------------|--------|
| Запуск | **systemd** | ✅ `deploy/debuginfod-go.service` |
| Мониторинг | **Zabbix** (`/zabbix`, `/healthz`) | ✅ endpoint; template/triggers — в плане |
| Доставка | **нативные `.deb` / `.rpm`** | ⬜ в плане |
| Конфигурация | **Ansible** (или имеющийся CM) | ⬜ в плане |
| Периметр | **nginx** (TLS, ACL, rate limit) | ⬜ в плане |
| БД (опц.) | **PostgreSQL** + backup | ✅ драйвер; backup — в плане |
| Оркестрация | ~~Kubernetes~~ | ❌ вне scope |

Docker — только для dev/demo (`examples/`, корневой `docker-compose.yml`), не для продакшн-развёртывания на целевых ОС.

---

## DevOps и развёртывание

### Высокий приоритет

- [ ] **Нативные пакеты `.deb` / `.rpm`** — nfpm/fpm: бинарник, пользователь `debuginfod`, каталоги `/var/lib`, `/etc/debuginfod-go`, postinst → `systemctl enable`
- [ ] **Ansible playbook** — раскатка на Astra/Ubuntu (deb) и RedOS/CentOS (rpm): пакет, `.env`, systemd, firewall
- [ ] **nginx reverse proxy** — пример конфига: TLS-терминация, ACL, rate limit на периметре
- [ ] **Zabbix template** — triggers (5xx, `last_scan_errors`, недоступность `/healthz`), actions (alert)

### Средний приоритет

- [x] **systemd unit** — `deploy/debuginfod-go.service`
- [ ] **Публикация пакетов из CI** — артефакты `.deb`/`.rpm` во внутренний apt/dnf-репозиторий (Nexus, aptly, pulp)
- [ ] **Backup** — SQLite/PostgreSQL (`pg_dump`), cache, конфиги; cron + restic/rsync
- [ ] **Документация продакшн-схемы** — `deploy/README.md`: systemd + пакеты + nginx + Zabbix + federation для резерва
- [ ] **Рекомендации PostgreSQL в проде** — когда переходить с SQLite, миграция, несколько инстансов за nginx

### Не планируется

- [x] ~~**Kubernetes / Helm chart**~~ — вне scope, не будет
- [x] ~~**Prometheus + Grafana**~~ — сознательно заменены Zabbix `/zabbix`
- [x] ~~**Docker в продакшн**~~ — только dev/demo; на целевых ОС — пакеты + systemd

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
- [x] **Сжатие HTTP** — gzip middleware

## Архивы и форматы пакетов

### Целевые (Astra / Ubuntu / RedOS / CentOS)

- [x] **`.deb`** — Debian/Ubuntu/Astra (индексация архивов)
- [x] **`.rpm`** — RedOS/CentOS (индексация архивов)
- [x] **Plain tar/tar.gz/tar.xz** — каталоги debuginfo
- [x] **Отложенное извлечение** — `DEBUGINFOD_LAZY_EXTRACT`
- [x] **Индексация исходников из SRPM/DSC**

### Не поддерживается (вне целевых ОС)

- [x] ~~**`.apk` (Alpine)**~~ — удалено, не в scope
- [x] ~~**`.pacman` / `.pkg.tar.zst` (Arch)**~~ — удалено, не в scope

## API и клиенты

- [x] **CLI `debuginfod-find`** — обёртка над HTTP API
- [x] **Пагинация metadata** — offset/limit + `next_offset`
- [x] **CORS и rate limiting**
- [x] **Аутентификация** — Basic Auth / mTLS
- [x] **OpenAPI/Swagger** — `internal/webapi/openapi.yaml`, `GET /openapi.yaml`

## Go-экосистема

- [ ] **Совместимость с `go tool buildid`** — документировать маппинг
- [ ] **Delve integration** — пример `DEBUGINFOD_URLS`
- [ ] **`-buildmode=pie` и внешний линкер** — тесты GNU build-id

## Качество и CI

- [ ] **golangci-lint в CI**
- [ ] **Сборка `.deb`/`.rpm` в CI** — связано с DevOps (nfpm + upload артефактов)
- [ ] **Coverage badge** — Codecov
- [ ] **Benchmark-тесты**
- [ ] **Fuzzing** — ELF notes, ar/tar
- [ ] **Кросс-компиляция** — GOOS/GOARCH матрица

## Безопасность

- [ ] **Валидация путей** — path traversal
- [ ] **Лимит размера архива**
- [ ] **IMA/подписи**
- [ ] **systemd hardening** — раскомментировать/документировать `ProtectSystem`, `ReadWritePaths` в unit

## Документация

- [x] **Примеры в `examples/`** — docker-compose, GDB-скрипт
- [x] **Диаграмма потока данных** — mermaid в DEVELOPMENT.md
- [x] **Сравнение с upstream debuginfod** — таблица в DEVELOPMENT.md
- [ ] **Руководство по эксплуатации** — `deploy/README.md` (см. DevOps)

---

## Идеи «на будущее»

- [x] **Web UI** — `/ui/` дашборд: статистика, поиск по build-id
- [ ] **Webhook при завершении scan**
- [ ] **S3/MinIO backend** — хранение извлечённых артефактов (альтернатива локальному cache)
- [ ] **Плагинная система форматов архивов**
- [ ] **Централизованные логи** — journald/rsyslog → ELK (если уже есть в инфраструктуре)

---

*Последнее обновление: 2026-07-14*
