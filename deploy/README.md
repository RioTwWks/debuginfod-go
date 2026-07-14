# Развёртывание debuginfod-go

Целевые ОС: **Astra Linux**, **Ubuntu**, **RedOS**, **CentOS**.  
Kubernetes не используется — нативные пакеты + systemd + Ansible + nginx + Zabbix.

## Компоненты

| Путь | Назначение |
|------|------------|
| [debuginfod-go.service](./debuginfod-go.service) | systemd unit |
| [nfpm.yaml](./nfpm.yaml) | Манифест `.deb` / `.rpm` |
| [package/](./package/) | postinstall, env example |
| [offline/](./offline/) | Оффлайн-установка пакетов и зависимостей |
| [security/](./security/) | Path traversal, IMA, systemd hardening |
| [ansible/](./ansible/) | Ansible playbook (deb/rpm) |
| [nginx/](./nginx/) | Reverse proxy (TLS, ACL, rate limit, cluster) |
| [zabbix/](./zabbix/) | Мониторинг: template, triggers, actions |
| [backup/](./backup/) | Backup/restore БД, config, timer |
| [postgresql/](./postgresql/) | PostgreSQL в проде, миграция, кластер |
| [OPERATIONS.md](./OPERATIONS.md) | **Руководство по эксплуатации** |

## Продакшн-схема

```text
make package / offline-bundle
         │
         ▼
    Ansible ──► systemd :8002 (localhost)
         │              │
         │              ├── SQLite (default) или PostgreSQL
         │              └── backup.timer (ежедневно)
         ▼
    nginx :443 ──► GDB / клиенты
         │
         ▼
    Zabbix template
```

## Порядок развёртывания

| Шаг | Действие | Документация |
|-----|----------|--------------|
| 1 | Пакет (online/offline) | [offline/README.md](./offline/README.md) |
| 2 | Ansible или ручная установка | [ansible/README.md](./ansible/README.md) |
| 3 | nginx + TLS | [nginx/README.md](./nginx/README.md) |
| 4 | Zabbix | [zabbix/README.md](./zabbix/README.md) |
| 5 | Backup timer | [backup/README.md](./backup/README.md) |
| 6 | PostgreSQL (опц.) | [postgresql/README.md](./postgresql/README.md) |

## Установка пакета

```bash
make package
sudo dpkg -i dist/debuginfod-go_*_amd64.deb    # Debian/Ubuntu/Astra
sudo dnf install dist/debuginfod-go-*.rpm      # RedOS/CentOS
```

Оффлайн: `make offline-bundle-deb` → [offline/README.md](./offline/README.md).

## Эксплуатация

Полное руководство оператора: **[OPERATIONS.md](./OPERATIONS.md)** (backup, PostgreSQL, мониторинг, troubleshooting).

Кратко:

```bash
sudo systemctl enable --now debuginfod-go
journalctl -u debuginfod-go -f
```

Конфиг: `/etc/debuginfod-go/debuginfod-go.env`

### Backup (ежедневно)

Пакет устанавливает скрипты в `/usr/libexec/debuginfod-go/`:

```bash
sudo systemctl enable --now debuginfod-go-backup.timer
sudo /usr/libexec/debuginfod-go/backup.sh    # ручной запуск
```

### Восстановление

```bash
sudo systemctl stop debuginfod-go
sudo /usr/libexec/debuginfod-go/restore.sh /var/backups/debuginfod-go/YYYYMMDD-HHMMSS
```

### PostgreSQL

Для кластера или HA БД — [postgresql/README.md](./postgresql/README.md).

### Мониторинг

Импорт [template_debuginfod-go.xml](./zabbix/template_debuginfod-go.xml), макрос `{$DEBUGINFOD.URL}`.

## Федерация (резерв)

```env
DEBUGINFOD_URLS=http://primary:8002,http://backup:8002
```

Или nginx upstream — [nginx/debuginfod-go-cluster.conf.snippet](./nginx/debuginfod-go-cluster.conf.snippet) + общий PostgreSQL.

## Сборка пакетов

```bash
make package
```

Требования: Go 1.21+, CGO, [nfpm](https://nfpm.goreleaser.com/).

## Чеклист production

- [ ] Пакет установлен, `healthz` отвечает
- [ ] nginx с TLS, порт 8002 не опубликован наружу
- [ ] `DEBUGINFOD_ZABBIX_KEY` задан, template импортирован
- [ ] `debuginfod-go-backup.timer` включён
- [ ] PostgreSQL (если >1 инстанса или корпоративный стандарт)
- [ ] Offsite backup (`restic` / `rsync`) при необходимости
