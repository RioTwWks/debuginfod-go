# Развёртывание debuginfod-go

Целевые ОС: **Astra Linux**, **Ubuntu**, **RedOS**, **CentOS**.  
Kubernetes не используется — нативные пакеты + systemd.

## Компоненты

| Путь | Назначение |
|------|------------|
| [debuginfod-go.service](./debuginfod-go.service) | systemd unit |
| [nfpm.yaml](./nfpm.yaml) | Манифест `.deb` / `.rpm` |
| [package/](./package/) | postinstall, env example |
| [offline/](./offline/) | **Оффлайн-установка** пакетов и зависимостей |
| [zabbix/](./zabbix/) | Мониторинг Zabbix |

## Установка

### Онлайн (из собранного пакета)

```bash
make package
sudo dpkg -i dist/debuginfod-go_*_amd64.deb    # Debian/Ubuntu/Astra
# или
sudo dnf install dist/debuginfod-go-*.rpm      # RedOS/CentOS
```

### Оффлайн (рекомендуется для изолированных контуров)

См. **[offline/README.md](./offline/README.md)**:

```bash
make offline-bundle-deb   # или offline-bundle-rpm
# перенос tar.gz → целевой хост → sudo ./install-offline.sh
```

## systemd

```bash
sudo systemctl enable --now debuginfod-go
sudo systemctl status debuginfod-go
journalctl -u debuginfod-go -f
```

Конфиг: `/etc/debuginfod-go/debuginfod-go.env`

## Сборка пакетов

Требования на build-хосте:

- Go 1.21+, CGO (`gcc`, `libsqlite3-dev` / `sqlite-devel`)
- [nfpm](https://nfpm.goreleaser.com/): `go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest`

```bash
make package          # .deb + .rpm в dist/
make package-deb      # только .deb
make package-rpm      # только .rpm
```
