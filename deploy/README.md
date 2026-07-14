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
| [ansible/](./ansible/) | **Ansible playbook** (deb/rpm) |
| [nginx/](./nginx/) | **Reverse proxy** (TLS, ACL, rate limit) |
| [zabbix/](./zabbix/) | Мониторинг: template, triggers, actions |

## Продакшн-схема

```text
CI → make package / offline-bundle
         │
         ▼
    Ansible (deploy/ansible)
         │
         ├── пакет .deb/.rpm + /etc/debuginfod-go/
         └── systemd debuginfod-go :8002 (localhost)
                    │
                    ▼
              nginx :443 (TLS, ACL, rate limit)
                    │
         ┌──────────┴──────────┐
         ▼                     ▼
    GDB / клиенты         Zabbix template
```

## Порядок развёртывания

1. **Пакет:** `make package` или `make offline-bundle-*` → [offline/README.md](./offline/README.md)
2. **Сервис:** `ansible-playbook` → [ansible/README.md](./ansible/README.md)
3. **Периметр:** nginx → [nginx/README.md](./nginx/README.md)
4. **Мониторинг:** импорт Zabbix template → [zabbix/README.md](./zabbix/README.md)

## Установка пакета

### Онлайн

```bash
make package
sudo dpkg -i dist/debuginfod-go_*_amd64.deb    # Debian/Ubuntu/Astra
sudo dnf install dist/debuginfod-go-*.rpm      # RedOS/CentOS
```

### Оффлайн

```bash
make offline-bundle-deb   # или offline-bundle-rpm
# tar.gz → целевой хост → sudo ./install-offline.sh
```

См. [offline/README.md](./offline/README.md).

## systemd

```bash
sudo systemctl enable --now debuginfod-go
journalctl -u debuginfod-go -f
```

Конфиг: `/etc/debuginfod-go/debuginfod-go.env`

## Сборка пакетов

```bash
make package          # .deb + .rpm в dist/
```

Требования: Go 1.21+, CGO, [nfpm](https://nfpm.goreleaser.com/).

## Федерация (резерв)

При недоступности основного инстанса клиенты могут использовать upstream:

```env
DEBUGINFOD_URLS=http://primary:8002,http://backup:8002
```

Или nginx upstream с несколькими backend-серверами.
