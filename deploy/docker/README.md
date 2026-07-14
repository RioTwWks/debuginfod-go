# Docker на Astra Linux и в изолированных сетях

## Проблема

`docker compose up --build` с корневым `Dockerfile` требует внутри контейнера:

1. `apt-get` → `deb.debian.org`
2. `go mod download` → proxy.golang.org

На Astra Linux 1.7.4 и в закрытых контурах внешние зеркала часто **недоступны** (`Ign: … InRelease`).

`make build` / `make run-env` на **хосте** при этом работают — используйте **prebuilt**-сборку.

## Рекомендуемый способ (Astra)

```bash
# Зависимости на хосте (один раз)
sudo apt-get install -y gcc libsqlite3-dev

make -C examples/sample
make build

# Сборка образа только с runtime-зависимостями (без Go в Docker)
docker compose -f docker-compose.yml -f docker-compose.prebuilt.yml up --build
```

Проверка:

```bash
curl http://127.0.0.1:8002/healthz
curl http://127.0.0.1:8002/readyz
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
| 1.7.x (часто) | bullseye (11) | `bullseye` |
| старые CE | buster (10) | `buster` |

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
