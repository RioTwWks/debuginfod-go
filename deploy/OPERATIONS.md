# Руководство по эксплуатации debuginfod-go

Для операторов на **Astra Linux**, **Ubuntu**, **RedOS**, **CentOS**.  
Развёртывание — [README.md](./README.md); разработка — [DEVELOPMENT.md](../DEVELOPMENT.md).

## Содержание

1. [Архитектура](#архитектура)
2. [Ежедневные проверки](#ежедневные-проверки)
3. [Управление сервисом](#управление-сервисом)
4. [Конфигурация](#конфигурация)
5. [Индексация и scan](#индексация-и-scan)
6. [Резервное копирование](#резервное-копирование)
7. [PostgreSQL](#postgresql)
8. [Мониторинг и алерты](#мониторинг-и-алерты)
9. [Обновление и откат](#обновление-и-откат)
10. [Устранение неполадок](#устранение-неполадок)
11. [Связанные документы](#связанные-документы)

---

## Архитектура

```text
GDB / debuginfod-find
        │
        ▼
   nginx :443  (TLS, ACL)          Zabbix
        │                              ▲
        ▼                              │
debuginfod-go :8002 (localhost) ──────┘ /zabbix, /healthz
        │
        ├── SQLite (default)  или  PostgreSQL
        ├── /var/cache/debuginfod-go  (LRU)
        └── scan: /usr/lib/debug, кэши пакетов
```

| Компонент | Путь / порт |
|-----------|-------------|
| Бинарник | `/usr/bin/debuginfod` |
| Конфиг | `/etc/debuginfod-go/debuginfod-go.env` |
| БД SQLite | `/var/lib/debuginfod-go/debuginfod.sqlite` |
| Кэш | `/var/cache/debuginfod-go` |
| systemd | `debuginfod-go.service` |
| Backup timer | `debuginfod-go-backup.timer` |

---

## Ежедневные проверки

```bash
# Живость
curl -sf http://127.0.0.1:8002/healthz

# Статистика (или Web UI)
curl -s http://127.0.0.1:8002/ui/api/stats | jq .

# Последний backup (не старше 25 ч)
ls -lt /var/backups/debuginfod-go/ | head -3

# Ошибки в логе
journalctl -u debuginfod-go --since "24 hours ago" -p err --no-pager
```

В Zabbix: нет активных триггеров `debuginfod:*` (см. [zabbix/README.md](./zabbix/README.md)).

---

## Управление сервисом

```bash
sudo systemctl status debuginfod-go
sudo systemctl restart debuginfod-go
sudo journalctl -u debuginfod-go -f
```

После изменения `/etc/debuginfod-go/debuginfod-go.env`:

```bash
sudo systemctl restart debuginfod-go
```

Graceful shutdown — по `SIGTERM` (systemd stop); активные HTTP-запросы завершаются в пределах timeout.

---

## Конфигурация

Файл: `/etc/debuginfod-go/debuginfod-go.env`. Полный список переменных — [README.md](../README.md#конфигурация) и `.env.example`.

### Типичные настройки по ОС

```bash
# Astra / Ubuntu
DEBUGINFOD_SCAN_PATH=/usr/lib/debug,/var/cache/apt/archives

# RedOS / CentOS
DEBUGINFOD_SCAN_PATH=/usr/lib/debug,/var/cache/dnf,/var/cache/yum
```

### Безопасность

| Переменная | Назначение |
|------------|------------|
| `DEBUGINFOD_ZABBIX_KEY` | Токен для `/zabbix` |
| `DEBUGINFOD_BASIC_AUTH_*` | Basic Auth (опц.) |
| `DEBUGINFOD_TLS_*` | TLS/mTLS (опц.) |

Порт **8002** не публикуйте наружу — только nginx на 443. См. [security/README.md](./security/README.md).

### Проверка после правок

```bash
curl -sf http://127.0.0.1:8002/healthz
curl -s "http://127.0.0.1:8002/metadata?key=glob&value=*" | head -c 200
```

---

## Индексация и scan

Индексация запускается при старте и по таймеру `DEBUGINFOD_RESCAN_INTERVAL` (default `1h`).

### Принудительный rescan

```bash
# HTTP (требует DEBUGINFOD_ADMIN_KEY или DEBUGINFOD_ZABBIX_KEY)
curl -X POST "http://127.0.0.1:8002/admin/rescan?key=${DEBUGINFOD_ZABBIX_KEY}"

# Или сигнал systemd (без перезапуска)
sudo systemctl kill -s USR1 debuginfod-go
```

Альтернатива: `sudo systemctl restart debuginfod-go`.

### Readiness

- `/healthz` — liveness (процесс жив)
- `/readyz` — readiness (первый scan завершён, или `DEBUGINFOD_SCAN_ENABLED=false`)

```bash
curl -sf http://127.0.0.1:8002/readyz
```

Для nginx/Ansible: не направлять трафик, пока `/readyz` не вернёт `200`.

Инкрементальный scan пропускает неизменённые файлы (`scanned_files` по mtime/size).

### Добавление путей scan

1. Добавить путь в `DEBUGINFOD_SCAN_PATH` (через запятую).
2. `sudo systemctl restart debuginfod-go`.
3. Проверить счётчики: `curl -s http://127.0.0.1:8002/zabbix | jq .artifacts_total`.

### Очистка кэша

```bash
sudo systemctl stop debuginfod-go
sudo rm -rf /var/cache/debuginfod-go/*
sudo systemctl start debuginfod-go
```

При `DEBUGINFOD_CACHE_MAX_BYTES > 0` LRU выполняется автоматически.

### Lazy extract

`DEBUGINFOD_LAZY_EXTRACT=true` (default): ELF извлекается из `.deb`/`.rpm` по HTTP-запросу, не при scan. При проблемах с диском — убедитесь, что cache и scan paths на разных разделах.

---

## Резервное копирование

Подробно: [backup/README.md](./backup/README.md).

### Включить ежедневный backup

```bash
sudo systemctl enable --now debuginfod-go-backup.timer
systemctl list-timers | grep debuginfod
```

### Ручной backup

```bash
sudo /usr/libexec/debuginfod-go/backup.sh
# или из исходников: sudo deploy/backup/backup.sh
```

Каталог: `/var/backups/debuginfod-go/<YYYYMMDD-HHMMSS>/`

| Файл | Содержимое |
|------|------------|
| `debuginfod.sqlite` | SQLite backup |
| `debuginfod.pgdump` | PostgreSQL dump |
| `debuginfod-go.env` | Конфиг |
| `cache.tar.gz` | Кэш (если `DEBUGINFOD_BACKUP_CACHE=true`) |

### Offsite (restic / rsync)

```bash
# restic — в .env:
# RESTIC_REPOSITORY=sftp:user@backup:/debuginfod
# RESTIC_PASSWORD=...

# rsync после backup:
rsync -az /var/backups/debuginfod-go/ backup@nas:/srv/backups/debuginfod-go/
```

### Восстановление

```bash
sudo systemctl stop debuginfod-go
sudo /usr/libexec/debuginfod-go/restore.sh /var/backups/debuginfod-go/20260714-030000
curl -sf http://127.0.0.1:8002/healthz
```

**Порядок:** остановить сервис → restore → проверить healthz → убедиться в метриках/артефактах.

---

## PostgreSQL

Подробно: [postgresql/README.md](./postgresql/README.md).

### Когда использовать

| Сценарий | БД |
|----------|-----|
| Один сервер | SQLite (default) |
| Несколько инстансов за nginx | PostgreSQL (общий индекс) |
| Корпоративный стандарт / HA | PostgreSQL |

### Переключение на PostgreSQL

1. Поднять БД, создать пользователя `debuginfod`.
2. Остановить сервис.
3. Миграция (опционально, без rescan):
   ```bash
   sudo deploy/postgresql/migrate-sqlite-to-postgres.sh \
     /var/lib/debuginfod-go/debuginfod.sqlite \
     'postgres://debuginfod:PASS@127.0.0.1:5432/debuginfod?sslmode=disable'
   ```
4. В `.env`:
   ```env
   DEBUGINFOD_DATABASE_URL=postgres://debuginfod:PASS@127.0.0.1:5432/debuginfod?sslmode=disable
   # закомментировать DEBUGINFOD_DB_PATH
   ```
5. При unix-socket PostgreSQL — см. `BindReadOnlyPaths` в [security/README.md](./security/README.md).
6. `sudo systemctl start debuginfod-go`.

### Backup PostgreSQL

`backup.sh` автоматически вызывает `pg_dump` при заданном `DEBUGINFOD_DATABASE_URL`.

### Кластер за nginx

Несколько инстансов с одним `DEBUGINFOD_DATABASE_URL`, локальный cache на каждом.  
nginx upstream: [nginx/debuginfod-go-cluster.conf.snippet](./nginx/debuginfod-go-cluster.conf.snippet).

---

## Мониторинг и алерты

### Zabbix

1. Импорт [template_debuginfod-go.xml](./zabbix/template_debuginfod-go.xml).
2. Макросы хоста: `{$DEBUGINFOD.URL}`, `{$DEBUGINFOD.ZABBIX_KEY}`.
3. Actions — [zabbix/actions.md](./zabbix/actions.md).

| Триггер | Действие оператора |
|---------|-------------------|
| `сервис недоступен (healthz)` | Проверить systemd, порт, firewall |
| `рост HTTP 5xx` | `journalctl -u debuginfod-go`, диск, БД |
| `ошибки scan` | Права на scan paths, формат архивов |
| `долгий scan` | Уменьшить `DEBUGINFOD_SCAN_PATH`, добавить workers |

### Web UI

```bash
curl -s http://127.0.0.1:8002/ui/api/stats
# или браузер: https://debuginfod.example.com/ui/
```

### Метрики вручную

```bash
curl -s -H "X-Zabbix-Token: $KEY" http://127.0.0.1:8002/zabbix | jq .
```

---

## Обновление и откат

### Обновление пакета

```bash
# Online
sudo dpkg -i debuginfod-go_<ver>_amd64.deb   # или dnf install

# Offline bundle
tar xf debuginfod-go-offline-deb-*.tar.gz
cd debuginfod-go-offline-deb-*/
sudo ./install-offline.sh
```

Перед обновлением — backup:

```bash
sudo /usr/libexec/debuginfod-go/backup.sh
```

После обновления:

```bash
sudo systemctl restart debuginfod-go
curl -sf http://127.0.0.1:8002/healthz
```

### Откат версии

1. Backup текущего состояния (на всякий случай).
2. Установить предыдущий `.deb`/`.rpm`.
3. При необходимости — restore БД из backup.
4. `systemctl restart debuginfod-go`.

### Федерация (резервный upstream)

На клиентах (GDB):

```bash
export DEBUGINFOD_URLS="https://primary/debuginfod,https://backup/debuginfod"
```

На сервере — upstream при 404: `DEBUGINFOD_URLS=http://backup-host:8002`.

---

## Устранение неполадок

| Симптом | Проверка | Решение |
|---------|----------|---------|
| `healthz` не отвечает | `systemctl status debuginfod-go` | Логи, права на `/var/lib`, CGO/SQLite |
| 404 на build-id | `curl .../metadata?key=buildid&value=<id>` | Добавить scan path, дождаться rescan |
| Медленный scan | `last_scan_duration_ms` в `/zabbix` | Сузить `SCAN_PATH`, увеличить `SCAN_WORKERS` (до 8 на 10 ГБ RAM) |
| Медленный dedup / рост SWAP | `free -h`, `pgrep -a xdelta` | `DEDUP_WORKERS=4` (не 8) при ~10 ГБ RAM; см. [QUIK_DEDUP.md](../docs/QUIK_DEDUP.md) |
| Диск заполнен | `df`, `cache_bytes` в `/zabbix` | LRU: `CACHE_MAX_BYTES`, очистка cache |
| 403 Forbidden | [security/README.md](./security/README.md) | Путь вне scan roots; проверить `pathsafe` |
| PostgreSQL connection | `journalctl -u debuginfod-go` | URL, firewall, `BindReadOnlyPaths` для socket |
| Zabbix no data | curl `/zabbix` с токеном | `DEBUGINFOD_ZABBIX_KEY`, URL макроса |

### Диагностический набор

```bash
sudo systemctl status debuginfod-go --no-pager
journalctl -u debuginfod-go -n 100 --no-pager
curl -v http://127.0.0.1:8002/healthz
curl -s http://127.0.0.1:8002/ui/api/stats
ls -la /var/lib/debuginfod-go /var/cache/debuginfod-go
```

### Оффлайн-контур

Установка без интернета: [offline/README.md](./offline/README.md).  
Ansible для конфигурации: [ansible/README.md](./ansible/README.md).

---

## Связанные документы

| Документ | Тема |
|----------|------|
| [deploy/README.md](./README.md) | Развёртывание, чеклист production |
| [offline/README.md](./offline/README.md) | Оффлайн bundle |
| [ansible/README.md](./ansible/README.md) | Ansible |
| [nginx/README.md](./nginx/README.md) | Reverse proxy |
| [backup/README.md](./backup/README.md) | Backup/restore |
| [postgresql/README.md](./postgresql/README.md) | PostgreSQL, кластер |
| [zabbix/README.md](./zabbix/README.md) | Мониторинг |
| [security/README.md](./security/README.md) | Hardening, IMA |
| [examples/README.md](../examples/README.md) | GDB demo |

---

*Целевые ОС: Astra Linux, Ubuntu, RedOS, CentOS. Kubernetes не используется.*
