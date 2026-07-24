# PostgreSQL в production

Когда использовать PostgreSQL вместо SQLite, настройка, миграция и несколько инстансов за nginx.

## Когда оставить SQLite

Подходит для:

- один сервер, один инстанс debuginfod-go;
- до ~500k артефактов в индексе (ориентир, не жёсткий лимит);
- простое развёртывание без отдельной СУБД;
- оффлайн-контуры с минимальными зависимостями.

По умолчанию пакет использует `/var/lib/debuginfod-go/debuginfod.sqlite`.

## Когда переходить на PostgreSQL

| Сценарий | Почему PostgreSQL |
|----------|-------------------|
| **Несколько инстансов** за nginx | Общий индекс metadata, один scan fleet |
| **Высокая доступность БД** | Репликация, backup pg_dump, PITR |
| **Большой индекс** | Лучше concurrent read/write при нагрузке |
| **Корпоративный стандарт** | Уже есть PostgreSQL в инфраструктуре |

Cache (`DEBUGINFOD_CACHE_DIR`) остаётся **локальным** на каждом инстансе — в PostgreSQL хранится только индекс.

## Тесты и локальная разработка (Docker)

Образ собирается из `deploy/postgresql/Dockerfile` (как [PVS-Studio-Tracker](https://github.com/RioTwWks/PVS-Studio-Tracker) — proxy в `build.args`).

```bash
# Прокси из /etc/environment (без source всего файла — совместимо с dash на Astra)
. deploy/docker/ensure-proxy-env.sh

deploy/docker/compose.sh -f docker-compose.postgres.yml up -d --build --wait
# или
make postgres-test-up

export DEBUGINFOD_DATABASE_URL=postgres://debuginfod:debuginfod@127.0.0.1:5433/debuginfod?sslmode=disable
make test-postgres
```

Переменные в `.env` рядом с compose или в shell:

```bash
HTTP_PROXY=http://proxy.corp:3128
HTTPS_PROXY=http://proxy.corp:3128
NO_PROXY=localhost,127.0.0.1,.corp.local
```

Интеграционные тесты без compose:

```bash
DEBUGINFOD_TEST_DATABASE_URL=postgres://debuginfod:debuginfod@127.0.0.1:5433/debuginfod?sslmode=disable \
  go test -tags=integration -v ./internal/storage -run Postgres
```

### permission denied (Docker socket)

Если `connect: permission denied` на `/var/run/docker.sock`:

```bash
sudo usermod -aG docker "$USER"
newgrp docker   # или перелогиниться
```

Либо разово: `sudo deploy/docker/compose.sh -f docker-compose.postgres.yml up -d --build --wait`

В проде контейнер не обязателен — достаточно системного PostgreSQL (ниже).

## Настройка PostgreSQL

### Debian / Ubuntu / Astra

```bash
sudo apt install postgresql
sudo -u postgres createuser --pwprompt debuginfod
sudo -u postgres createdb -O debuginfod debuginfod
```

### RedOS / CentOS

```bash
sudo dnf install postgresql-server postgresql
sudo postgresql-setup --initdb
sudo systemctl enable --now postgresql
sudo -u postgres createuser --pwprompt debuginfod
sudo -u postgres createdb -O debuginfod debuginfod
```

### Конфигурация debuginfod-go

`/etc/debuginfod-go/debuginfod-go.env`:

```env
DEBUGINFOD_DATABASE_URL=postgres://debuginfod:PASSWORD@127.0.0.1:5432/debuginfod?sslmode=disable
# DEBUGINFOD_DB_PATH=...   # закомментировать SQLite
```

```bash
sudo systemctl restart debuginfod-go
curl http://127.0.0.1:8002/ui/api/stats
```

Схема создаётся автоматически при первом запуске (`internal/storage/postgres.go`).

## Миграция с SQLite

### Вариант A: полный rescan (простой)

1. Настроить PostgreSQL и `DEBUGINFOD_DATABASE_URL`
2. Остановить сервис, очистить не нужно — пустая БД
3. Запустить — выполнится scan с нуля

Подходит, если rescan занимает приемлемое время (зависит от `DEBUGINFOD_SCAN_PATH`).

### Вариант B: перенос индекса (без rescan)

```bash
sudo systemctl stop debuginfod-go
sudo deploy/postgresql/migrate-sqlite-to-postgres.sh \
  /var/lib/debuginfod-go/debuginfod.sqlite \
  'postgres://debuginfod:PASSWORD@127.0.0.1:5432/debuginfod?sslmode=disable'
# Обновить .env → DEBUGINFOD_DATABASE_URL
sudo systemctl start debuginfod-go
```

Требует: `sqlite3`, `psql`.

## Несколько инстансов за nginx

```text
                    ┌── debuginfod-1 :8002 (cache local)
GDB → nginx :443 ───┼── debuginfod-2 :8002 (cache local)
                    └── debuginfod-3 :8002 (cache local)
                              │
                              ▼
                    PostgreSQL (общий индекс)
```

### nginx upstream

```nginx
upstream debuginfod_cluster {
    server 10.0.1.11:8002;
    server 10.0.1.12:8002;
    server 10.0.1.13:8002;
    keepalive 32;
}

location / {
    proxy_pass http://debuginfod_cluster;
    # ... proxy_set_header и т.д.
}
```

На каждом инстансе:

```env
DEBUGINFOD_DATABASE_URL=postgres://debuginfod:pass@10.0.1.100:5432/debuginfod
DEBUGINFOD_SCAN_PATH=/usr/lib/debug
DEBUGINFOD_CACHE_DIR=/var/cache/debuginfod-go
```

### Scan при нескольких инстансах

- **Designated scanner (рекомендуется):** на read-only репликах `DEBUGINFOD_SCAN_ENABLED=false` — индекс только из PostgreSQL, `/readyz` сразу `200`
- **Альтернатива:** все инстансы сканируют одни пути; `scanned_files` в PostgreSQL дедуплицирует работу по mtime/size (инкрементальный scan)

### Федерация как альтернатива

Вместо кластера за nginx — primary + backup через `DEBUGINFOD_URLS` у клиентов (см. [deploy/README.md](../README.md#федерация-резерв)).

## Backup

PostgreSQL: [backup/README.md](../backup/README.md) — `pg_dump` в `debuginfod.pgdump`.

```bash
sudo deploy/backup/backup.sh
```

## Мониторинг

Zabbix template не различает backend — метрики те же (`/zabbix`).  
Дополнительно мониторьте PostgreSQL стандартными средствами (connections, disk, replication lag).

## Безопасность

- `sslmode=require` или `verify-full` для production PostgreSQL
- Отдельный пользователь БД с правами только на `debuginfod`
- Не открывать порт 5432 наружу; только localhost или private network
