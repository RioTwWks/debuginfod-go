# TODO — план развития debuginfod-go

Список улучшений по приоритету. Выполненное — `[x]`.

**Статус проекта (2026-07-14):** MVP завершён — elfutils-совместимость, эксплуатация (Zabbix, offline, Ansible, nginx), Web UI, API, security, docs, Go-экосистема, CI. Ниже — опциональные улучшения.

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
| Мониторинг | **Zabbix** (`/zabbix`, `/healthz`) | ✅ template + triggers + actions.md |
| Доставка | **нативные `.deb` / `.rpm`** | ✅ nfpm + `make package` |
| Оффлайн | **bundle без интернета** | ✅ `deploy/offline/`, `make offline-bundle-*` |
| Конфигурация | **Ansible** | ✅ `deploy/ansible/` |
| Периметр | **nginx** (TLS, ACL, rate limit) | ✅ `deploy/nginx/` |
| БД (опц.) | **PostgreSQL** + backup | ✅ драйвер + [backup/](./deploy/backup/), [postgresql/](./deploy/postgresql/) |
| Оркестрация | ~~Kubernetes~~ | ❌ вне scope |

Docker — только для dev/demo (`examples/`, корневой `docker-compose.yml`), не для продакшн-развёртывания на целевых ОС.

---

## DevOps и развёртывание

### Высокий приоритет

- [x] **Нативные пакеты `.deb` / `.rpm`** — nfpm: `deploy/nfpm.yaml`, postinstall, systemd, `/etc/debuginfod-go`
- [x] **Оффлайн-установка** — `deploy/offline/`: скачивание зависимостей, bundle `.tar.gz`, install без сети
- [x] **Ansible playbook** — `deploy/ansible/`: deb/rpm, env, systemd, firewall (опц.)
- [x] **nginx reverse proxy** — `deploy/nginx/`: TLS, ACL, rate limit
- [x] **Zabbix template** — `template_debuginfod-go.xml`, triggers, [actions.md](./deploy/zabbix/actions.md)

### Средний приоритет

- [x] **systemd unit** — `deploy/debuginfod-go.service`
- [x] ~~**Публикация пакетов из CI**~~ — не планируется (достаточно offline bundle + ручная/Ansible доставка)
- [x] **Backup** — `deploy/backup/`: SQLite/PostgreSQL, config, timer, restic/rsync
- [x] **Документация продакшн-схемы** — `deploy/README.md` (чеклист, эксплуатация)
- [x] **Рекомендации PostgreSQL в проде** — `deploy/postgresql/`: миграция, кластер за nginx

### Не планируется

- [x] ~~**Kubernetes / Helm chart**~~ — вне scope, не будет
- [x] ~~**Prometheus + Grafana**~~ — сознательно заменены Zabbix `/zabbix`
- [x] ~~**Docker в продакшн**~~ — только dev/demo; на целевых ОС — пакеты + systemd
- [x] ~~**CI → Nexus/aptly/pulp**~~ — не требуется при offline bundle

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

- [x] **Совместимость с `go tool buildid`** — документировать маппинг
- [x] **Delve integration** — пример `DEBUGINFOD_URLS`
- [x] **`-buildmode=pie` и внешний линкер** — тесты GNU build-id

## Качество и CI

- [x] **golangci-lint в CI**
- [x] **Сборка `.deb`/`.rpm` в CI** — upload артефактов в GitHub Actions (без внутреннего репозитория)
- [x] **Coverage badge** — Codecov
- [x] **Benchmark-тесты**
- [x] **Fuzzing** — ELF notes, ar/tar
- [x] **Кросс-компиляция** — GOOS/GOARCH матрица

## Безопасность

- [x] **Валидация путей** — `internal/pathsafe`, проверка в webapi/archive
- [x] ~~**Лимит размера архива**~~ — не планируется
- [x] **IMA/подписи** — `deploy/security/README.md`: подпись пакетов, IMA appraisal, рекомендации
- [x] **systemd hardening** — `ProtectSystem`, `ReadWritePaths` в unit + документация

## Документация

- [x] **Примеры в `examples/`** — docker-compose, GDB, Delve
- [x] **Диаграмма потока данных** — mermaid в DEVELOPMENT.md
- [x] **Сравнение с upstream debuginfod** — таблица в DEVELOPMENT.md
- [x] **Руководство по эксплуатации** — [deploy/OPERATIONS.md](./deploy/OPERATIONS.md): backup, PostgreSQL, мониторинг, troubleshooting

---

## Следующие шаги (рекомендуется)

Пункты с реальной пользой для эксплуатации на целевых ОС. Приоритет — сверху вниз.

### Эксплуатация

- [ ] **Readiness probe `/readyz`** — 200 после первого успешного scan (Ansible/nginx: не отдавать трафик на «пустой» индекс; сейчас `/healthz` только liveness)
- [ ] **Ручной rescan** — `SIGUSR1` или защищённый `POST /admin/rescan` без ожидания `DEBUGINFOD_RESCAN_INTERVAL` (после заливки пакетов в scan path)
- [ ] **Designated scanner** — `DEBUGINFOD_SCAN_ENABLED=false` на read-only репликах PostgreSQL-кластера (сейчас — workaround через `RESCAN_INTERVAL=0`; см. [deploy/postgresql/README.md](./deploy/postgresql/README.md))
- [ ] **Webhook при завершении scan** — HTTP POST с `indexed/skipped/errors/duration` (интеграция с CI, Zabbix trapper, внутренние уведомления)

### CI и релизы

- [ ] **E2E smoke в CI** — прогон `examples/` (docker-compose: healthz → metadata → GDB/Delve batch) на каждый PR
- [ ] **GitHub Releases** — публикация `.deb`/`.rpm` по git tag (артефакты из CI `package` job → Release assets)

### По запросу (платформы / upstream)

- [ ] **Пакеты arm64** — `.deb`/`.rpm` для `aarch64` (сейчас nfpm и CI — только `amd64`; cross-build `debuginfod` для linux/arm64 уже есть)
- [ ] **IMA verification** — опциональная проверка подписей при federation/скачивании (`DEBUGINFOD_IMA=enforcing`, parity с elfutils 0.192+; рекомендации уже в [deploy/security/README.md](./deploy/security/README.md))

### UI (низкий приоритет)

- [x] **Web UI: поиск metadata** — glob/file в дашборде (сейчас `/ui/api/search` — только prefix по build-id)

---

## Идеи «на будущее»

Крупные изменения архитектуры — только при явной потребности (кластер, нестандартные форматы).

- [x] **Web UI** — `/ui/` дашборд: статистика, поиск по build-id
- [ ] **S3/MinIO backend** — хранение извлечённых артефактов (альтернатива локальному cache; общий кэш для нескольких инстансов)
- [ ] **Плагинная система форматов архивов** — подключение обработчиков `.deb`/`.rpm`/tar без правок `internal/archive`
- [ ] **Централизованные логи** — journald/rsyslog → ELK (если уже есть в инфраструктуре; код не обязателен — достаточно runbook)

### Не планируется

- [x] ~~**Kubernetes / Helm**~~ — вне scope
- [x] ~~**LDAP-авторизация**~~ — Basic Auth / mTLS / nginx ACL достаточно для целевого деплоя
- [x] ~~**Prometheus**~~ — Zabbix `/zabbix`

---

*Последнее обновление: 2026-07-14*
