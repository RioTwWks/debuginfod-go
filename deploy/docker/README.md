# Docker на Astra Linux и в изолированных сетях

## Проблема

`docker compose up --build` требует сетевой доступ **внутри build-контейнера**:

1. `apt-get` → репозитории Debian или Astra
2. `go mod download` → proxy.golang.org (полный `Dockerfile`)

На корпоративных хостах интернет часто доступен **только через HTTP(S)-прокси**. Переменные прокси в `/etc/environment` действуют на `apt`/`curl` **на хосте**, но Docker build **не подхватывает** их автоматически — `apt-get` внутри контейнера зависает на `Ign: … InRelease`.

`make build` / `make run-env` на **хосте** при этом работают — используйте **prebuilt**-сборку.

## Корпоративный прокси (частая причина зависания)

### 1. Экспортировать прокси в текущую сессию

```bash
# Проверить, что прокси виден в shell (не только в /etc/environment)
echo "$HTTP_PROXY $http_proxy"

# Подхватить из /etc/environment
set -a && source /etc/environment && set +a

# Или явно
export HTTP_PROXY=http://proxy.corp:3128
export HTTPS_PROXY=http://proxy.corp:3128
export NO_PROXY=localhost,127.0.0.1,.corp.local
```

### 2. Собрать с прокси

`make docker-astra` и другие docker-цели автоматически вызывают `deploy/docker/ensure-proxy-env.sh`.

В логе build должно появиться:

```text
APT proxy: http://proxy.corp:3128
```

Для ручного `docker compose`:

```bash
source deploy/docker/ensure-proxy-env.sh   # или export вручную
make build
make docker-astra
```

Прокси передаётся в build через `HTTP_PROXY` / `HTTPS_PROXY` build-args и настраивает `/etc/apt/apt.conf.d/99proxy` внутри контейнера (`deploy/docker/configure-proxy.sh`).

### 3. Прокси для демона Docker (pull образов)

Если `docker pull debian:buster-slim` без прокси не работает, настройте systemd:

```bash
sudo mkdir -p /etc/systemd/system/docker.service.d
sudo tee /etc/systemd/system/docker.service.d/http-proxy.conf <<'EOF'
[Service]
Environment="HTTP_PROXY=http://proxy.corp:3128"
Environment="HTTPS_PROXY=http://proxy.corp:3128"
Environment="NO_PROXY=localhost,127.0.0.1"
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker
```

## Рекомендуемый способ (Astra)

```bash
# Зависимости на хосте (один раз)
sudo apt-get install -y gcc libsqlite3-dev

make -C examples/sample
make build

# APT внутри контейнера — те же репозитории, что и на хосте (download.astralinux.ru)
make docker-astra
# или:
docker compose -f docker-compose.yml -f docker-compose.prebuilt.yml -f docker-compose.astra.yml up --build
```

Проверка:

```bash
curl http://127.0.0.1:8002/healthz
curl http://127.0.0.1:8002/readyz
```

## Профиль APT_PROFILE=astra

Опционально: если `deb.debian.org` недоступен, а `download.astralinux.ru` — доступен (часто через тот же прокси), overlay `docker-compose.astra.yml` подменяет sources.list:

Overlay `docker-compose.astra.yml` включает:

| Переменная | Значение | Назначение |
|------------|----------|------------|
| `APT_PROFILE` | `astra` | Подмена `/etc/apt/sources.list` на репозитории Astra 1.7 |
| `DEBIAN_SUITE` | `buster` | База образа Debian 10 (совместима с Astra 1.7) |

Файлы:

- `deploy/docker/sources.astra-1.7.list` — те же зеркала, что у `apt` на хосте
- `deploy/docker/install-astra-apt.sh` — подключает sources при сборке образа

Ручной запуск без overlay:

```bash
export APT_PROFILE=astra
export DEBIAN_SUITE=buster
make build
docker compose -f docker-compose.yml -f docker-compose.prebuilt.yml up --build
```

## Зеркало APT внутри контейнера

Если `deb.debian.org` недоступен, но есть внутреннее зеркало Debian:

```bash
export DEBIAN_SUITE=bullseye
export APT_MIRROR=mirror.yandex.ru/debian   # или корпоративное зеркало
docker compose -f docker-compose.yml -f docker-compose.prebuilt.yml up --build
```

Скопируйте [.env.docker.example](../../.env.docker.example) → `.env.docker` и подставьте свои значения.

## Полностью без apt в контейнере

На машине **с сетью** один раз подготовьте runtime-образ:

```bash
docker build -f Dockerfile.prebuilt --build-arg DEBIAN_SUITE=bullseye -t debuginfod-go-runtime:astra .
docker save debuginfod-go-runtime:astra -o debuginfod-go-runtime.tar
```

На **Astra** (оффлайн):

```bash
docker load -i debuginfod-go-runtime.tar
# Переименуйте базу в Dockerfile.prebuilt: FROM debuginfod-go-runtime:astra
# и соберите с SKIP_APT_INSTALL=true
export SKIP_APT_INSTALL=true
make build
docker compose -f docker-compose.yml -f docker-compose.prebuilt.yml up --build
```

Или отредактируйте `Dockerfile.prebuilt`: `FROM debuginfod-go-runtime:astra` и уберите `RUN apt-get`.

## Версия Debian и Astra 1.7

| Astra | База Debian | `DEBIAN_SUITE` |
|-------|-------------|----------------|
| 1.7.x SE | buster (10) | `buster` |
| CE 2.12+ | bullseye (11) | `bullseye` |

Для Astra 1.7 используйте `make docker-astra` или `APT_PROFILE=astra` + `DEBIAN_SUITE=buster`.

Бинарник с хоста должен собираться на той же или **более старой** glibc, чем в контейнере. При `GLIBC_2.xx not found` — понизьте `DEBIAN_SUITE` или соберите бинарник в контейнере на той же базе.

## Сеть сборки

`docker-compose.prebuilt.yml` использует `network: host` для этапа build — Docker наследует DNS/маршрутизацию хоста Astra.

Для полного `Dockerfile` (с Go внутри):

```bash
docker compose build --network=host
```

## Production

На целевых ОС Docker **не рекомендуется** для production — используйте `.deb`/systemd ([deploy/README.md](../README.md)).

Docker — только dev/demo ([examples/](../../examples/)).
