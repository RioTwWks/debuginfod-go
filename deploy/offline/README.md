# Оффлайн-установка debuginfod-go

Установка **debuginfod-go** и **runtime-зависимостей** без доступа в интернет на целевых ОС:

| Семейство | ОС | Формат |
|-----------|-----|--------|
| Debian | Astra Linux, Ubuntu | `.deb` |
| RHEL | RedOS, CentOS | `.rpm` |

## Схема

```text
[Онлайн build-хост]                    [Оффлайн целевой хост]
  make offline-bundle-deb    tar.gz  →   tar xf … && sudo ./install-offline.sh
  make offline-bundle-rpm
```

## Быстрый старт

### Debian / Astra / Ubuntu

На машине **с интернетом**:

```bash
# nfpm: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
sudo apt-get install -y gcc libsqlite3-dev nfpm  # или nfpm из go install

make offline-bundle-deb
# → dist/offline/debuginfod-go-offline-deb-<version>.tar.gz
```

Перенесите архив на изолированный хост и установите:

```bash
tar xf debuginfod-go-offline-deb-*.tar.gz
cd debuginfod-go-offline-deb-*/
sudo ./install-offline.sh
curl http://localhost:8002/healthz
```

### RedOS / CentOS

На машине **с интернетом**:

```bash
sudo dnf install -y gcc sqlite-devel nfpm createrepo
# nfpm: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest

make offline-bundle-rpm
# → dist/offline/debuginfod-go-offline-rpm-<version>.tar.gz
```

На **оффлайн** хосте:

```bash
tar xf debuginfod-go-offline-rpm-*.tar.gz
cd debuginfod-go-offline-rpm-*/
sudo ./install-offline.sh
```

## Пошагово

| Шаг | Команда | Где |
|-----|---------|-----|
| 1. Сборка пакета | `make package` | build-хост |
| 2. Скачать зависимости | `make offline-download-deb` или `-rpm` | build-хост (online) |
| 3. Собрать bundle | `make offline-bundle-deb` или `-rpm` | build-хост |
| 4. Установка | `sudo ./install-offline.sh` | целевой хост (offline) |

## Что входит в пакет

| Файл | Путь после установки |
|------|----------------------|
| `debuginfod` | `/usr/bin/debuginfod` |
| `debuginfod-find` | `/usr/bin/debuginfod-find` |
| systemd unit | `/lib/systemd/system/debuginfod-go.service` |
| конфиг (пример) | `/etc/debuginfod-go/debuginfod-go.env.example` |
| данные | `/var/lib/debuginfod-go/` |
| кэш | `/var/cache/debuginfod-go/` |

Runtime-зависимости (в bundle):

- **deb:** `libsqlite3-0`, `ca-certificates` (+ транзитивные)
- **rpm:** `sqlite-libs`, `ca-certificates` (+ транзитивные)

## Сборка из исходников оффлайн (опционально)

Если на изолированном хосте нет готового пакета, но есть Go toolchain:

```bash
# На online-хосте: go mod vendor && tar czf vendor.tar.gz vendor go.mod go.sum
# На offline-хосте:
tar xf vendor.tar.gz
CGO_ENABLED=1 go build -mod=vendor -o debuginfod ./cmd/debuginfod
```

Для production рекомендуется **готовый .deb/.rpm bundle** — не требует Go и компилятора на целевом хосте.

## Скрипты

| Скрипт | Назначение |
|--------|------------|
| `download-deps-deb.sh` | Скачать `.deb` + зависимости в `dist/offline/deb/pool/` |
| `download-deps-rpm.sh` | Скачать `.rpm` + зависимости в `dist/offline/rpm/pool/` |
| `install-offline-deb.sh` | Локальный apt repo + `apt install` без сети |
| `install-offline-rpm.sh` | `createrepo` + `dnf install` только из file:// |
| `make-bundle.sh` | Упаковать pool + installer в `.tar.gz` |

## Манифесты зависимостей

- `deps-deb.txt` — список seed-пакетов для apt
- `deps-rpm.txt` — список seed-пакетов для dnf

При необходимости дополните (например `adduser` на минимальных системах).

## Конфигурация после установки

```bash
sudo editor /etc/debuginfod-go/debuginfod-go.env
sudo systemctl restart debuginfod-go
```

Типичные пути scan — см. [README.md](../../README.md#типичные-пути-scan).

## Удаление

```bash
# Debian
sudo apt-get remove debuginfod-go
sudo rm -f /etc/apt/sources.list.d/debuginfod-go-offline.list

# RHEL
sudo dnf remove debuginfod-go
sudo rm -f /etc/yum.repos.d/debuginfod-go-offline.repo
```
