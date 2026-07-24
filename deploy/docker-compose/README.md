# Docker Compose — PostgreSQL для тестов

Как [PVS-Studio-Tracker/deploy/docker-compose](https://github.com/RioTwWks/PVS-Studio-Tracker/tree/main/deploy/docker-compose).

## Docker не стартует после daemon.json

На **Astra Linux 1.7** (Docker &lt; 23) секция `"proxies"` в `/etc/docker/daemon.json` **не поддерживается** — `docker.service` падает.

**Восстановление:**

```bash
sudo rm -f /etc/docker/daemon.json
sudo systemctl restart docker
sudo systemctl status docker
```

Затем proxy через **systemd** (см. ниже), не через daemon.json.

## Быстрый старт

```bash
# 1) Proxy для docker pull (Astra — systemd)
sudo deploy/docker-compose/setup-docker-proxy.sh http://192.168.250.193:3128
docker pull postgres:16-alpine

# 2) Postgres
make postgres-test-up
```

Или вручную:

```bash
cd deploy/docker-compose
./compose.sh -f docker-compose.postgres.yml up -d
```

`DEBUGINFOD_DATABASE_URL=postgres://debuginfod:debuginfod@127.0.0.1:5433/debuginfod?sslmode=disable`

```bash
make test-postgres-integration
```

## Корпоративный proxy

`HTTP_PROXY` в shell / `.env` **не влияет** на `docker pull` образа `postgres:16-alpine`.

### Astra / Docker Engine &lt; 23 (рекомендуется)

```bash
sudo mkdir -p /etc/systemd/system/docker.service.d
sudo cp deploy/docker-compose/http-proxy.conf.example \
  /etc/systemd/system/docker.service.d/http-proxy.conf
sudo nano /etc/systemd/system/docker.service.d/http-proxy.conf
sudo systemctl daemon-reload
sudo systemctl restart docker

docker pull postgres:16-alpine
```

Или одной командой:

```bash
sudo deploy/docker-compose/setup-docker-proxy.sh http://192.168.250.193:3128
```

### Docker Engine 23+ (daemon.json)

Только если `docker version` показывает Engine 23 или новее:

```bash
sudo cp deploy/docker-compose/daemon.json.example /etc/docker/daemon.json
# отредактируйте httpProxy / httpsProxy
sudo systemctl restart docker
```

На Astra **не используйте** daemon.json с `proxies` — см. раздел восстановления выше.

### Права на Docker

```bash
sudo usermod -aG docker "$USER"
newgrp docker
```

## Тесты

```bash
export DEBUGINFOD_TEST_DATABASE_URL=postgres://debuginfod:debuginfod@127.0.0.1:5433/debuginfod?sslmode=disable
go test -tags=integration -v ./internal/storage -run Postgres
```

Продакшен PostgreSQL без Docker: [../postgresql/README.md](../postgresql/README.md).
