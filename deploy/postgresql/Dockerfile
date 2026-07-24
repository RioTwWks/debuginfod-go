# PostgreSQL 16 для тестов debuginfod-go.
#
# Корпоративный proxy в build (pull базового образа). Как PVS-Studio-Tracker:
#   source deploy/docker/ensure-proxy-env.sh
#   docker compose -f docker-compose.postgres.yml up -d --build --wait
#
# Или: make postgres-test-up

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

FROM postgres:16-alpine

# Базовый образ postgres:16-alpine; proxy передаётся через build.args в compose.
