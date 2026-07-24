# Docker Compose — PostgreSQL для тестов

Как [PVS-Studio-Tracker/deploy/docker-compose](https://github.com/RioTwWks/PVS-Studio-Tracker/tree/main/deploy/docker-compose).

## Быстрый старт

```bash
cd deploy/docker-compose
cp .env.example .env
# при необходимости — proxy в daemon.json (см. ниже)

./compose.sh -f docker-compose.postgres.yml up -d
```

Из корня репозитория:

```bash
make postgres-test-up
make test-postgres-integration
```

`DEBUGINFOD_DATABASE_URL=postgres://debuginfod:debuginfod@127.0.0.1:5433/debuginfod?sslmode=disable`

## Корпоративный proxy

Ошибка `Get "https://registry-1.docker.io/v2/": ... Client.Timeout` — **Docker daemon не ходит в Hub**.  
`HTTP_PROXY` в `.env` **не влияет** на `docker pull` образа `postgres:16-alpine`.

### 1. Proxy для Docker daemon (обязательно для pull)

```bash
sudo cp deploy/docker-compose/daemon.json.example /etc/docker/daemon.json
# отредактируйте httpProxy / httpsProxy / noProxy (ваш proxy: 192.168.250.193:3128)
sudo systemctl restart docker

docker pull postgres:16-alpine
cd deploy/docker-compose
./compose.sh -f docker-compose.postgres.yml up -d
```

Альтернатива (systemd):

```bash
sudo mkdir -p /etc/systemd/system/docker.service.d
sudo tee /etc/systemd/system/docker.service.d/http-proxy.conf <<'EOF'
[Service]
Environment="HTTP_PROXY=http://192.168.250.193:3128"
Environment="HTTPS_PROXY=http://192.168.250.193:3128"
Environment="NO_PROXY=localhost,127.0.0.1"
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker
```

### 2. Права на Docker

```bash
sudo usermod -aG docker "$USER"
newgrp docker
```

## Тесты

```bash
export DEBUGINFOD_TEST_DATABASE_URL=postgres://debuginfod:debuginfod@127.0.0.1:5433/debuginfod?sslmode=disable
go test -tags=integration -v ../../internal/storage -run Postgres
```

Продакшен PostgreSQL без Docker: [../postgresql/README.md](../postgresql/README.md).
