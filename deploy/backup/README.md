# Резервное копирование debuginfod-go

Скрипты для backup/restore БД, конфигурации и (опционально) cache.

## Что копируется

| Компонент | SQLite | PostgreSQL |
|-----------|--------|------------|
| БД | `sqlite3 .backup` → `debuginfod.sqlite` | `pg_dump` → `debuginfod.pgdump` |
| Конфиг | `/etc/debuginfod-go/debuginfod-go.env` | то же |
| Cache | опционально `cache.tar.gz` | опционально |

Каталог по умолчанию: `/var/backups/debuginfod-go/<YYYYMMDD-HHMMSS>/`

## Ручной запуск

```bash
sudo cp deploy/backup/backup.sh /usr/local/sbin/debuginfod-go-backup
sudo chmod +x /usr/local/sbin/debuginfod-go-backup
sudo debuginfod-go-backup
```

Или из репозитория:

```bash
sudo deploy/backup/backup.sh
```

## Восстановление

```bash
sudo systemctl stop debuginfod-go
sudo deploy/backup/restore.sh /var/backups/debuginfod-go/20260714-030000
curl http://127.0.0.1:8002/healthz
```

## systemd timer (ежедневно)

```bash
sudo install -m 0755 deploy/backup/backup.sh /usr/libexec/debuginfod-go/backup.sh
sudo install -m 0644 deploy/backup/debuginfod-go-backup.service /etc/systemd/system/
sudo install -m 0644 deploy/backup/debuginfod-go-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now debuginfod-go-backup.timer
systemctl list-timers | grep debuginfod
```

## Переменные окружения

Задаются в `/etc/debuginfod-go/debuginfod-go.env` или перед запуском:

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `DEBUGINFOD_BACKUP_DIR` | `/var/backups/debuginfod-go` | Каталог backup |
| `DEBUGINFOD_BACKUP_KEEP_DAYS` | `14` | Хранить локальные копии (дней) |
| `DEBUGINFOD_BACKUP_CACHE` | `false` | Архивировать cache (может быть большим) |
| `RESTIC_REPOSITORY` | — | Offsite backup через [restic](https://restic.net/) |
| `RESTIC_PASSWORD` | — | Пароль restic-репозитория |

### Пример restic (offsite)

```bash
# В /etc/debuginfod-go/debuginfod-go.env:
RESTIC_REPOSITORY=sftp:user@backup-host:/backups/debuginfod
RESTIC_PASSWORD_FILE=/etc/debuginfod-go/restic-password
```

В `backup.sh` поддерживается `RESTIC_PASSWORD`; для `RESTIC_PASSWORD_FILE` оберните в systemd:

```ini
EnvironmentFile=/etc/debuginfod-go/restic.env
```

## cron (альтернатива timer)

```cron
0 3 * * * root /usr/libexec/debuginfod-go/backup.sh >> /var/log/debuginfod-go-backup.log 2>&1
```

## rsync на удалённый хост

После локального backup:

```bash
rsync -az --delete /var/backups/debuginfod-go/ backup@nas:/srv/backups/debuginfod-go/
```

Рекомендуется запускать после `backup.sh` через cron или отдельный systemd unit.

## Зависимости

| Backend | Пакеты |
|---------|--------|
| SQLite | `sqlite3` (рекомендуется) |
| PostgreSQL | `postgresql-client` / `postgresql` (pg_dump, pg_restore) |
| Restic | `restic` (опционально) |

## Связанные документы

- [postgresql/README.md](../postgresql/README.md) — PostgreSQL в проде
- [../README.md](../README.md) — общая схема развёртывания
