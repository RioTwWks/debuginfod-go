# Сборка debuginfod-go (Debian — совместимо с Astra/Ubuntu).
# На Astra без доступа к deb.debian.org используйте Dockerfile.prebuilt (см. deploy/docker/README.md).

ARG DEBIAN_SUITE=bookworm
FROM golang:1.21-${DEBIAN_SUITE} AS builder

ARG APT_MIRROR=
RUN if [ -n "${APT_MIRROR}" ]; then \
		sed -i "s|deb.debian.org|${APT_MIRROR}|g" /etc/apt/sources.list; \
		sed -i "s|security.debian.org|${APT_MIRROR}|g" /etc/apt/sources.list || true; \
	fi \
	&& apt-get -o Acquire::Retries=5 update \
	&& DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
		gcc \
		libc6-dev \
		libsqlite3-dev \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /debuginfod ./cmd/debuginfod

ARG DEBIAN_SUITE=bookworm
FROM debian:${DEBIAN_SUITE}-slim

ARG APT_MIRROR=
RUN if [ -n "${APT_MIRROR}" ]; then \
		sed -i "s|deb.debian.org|${APT_MIRROR}|g" /etc/apt/sources.list; \
		sed -i "s|security.debian.org|${APT_MIRROR}|g" /etc/apt/sources.list || true; \
	fi \
	&& apt-get -o Acquire::Retries=5 update \
	&& DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
		ca-certificates \
		curl \
		libsqlite3-0 \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /data

COPY --from=builder /debuginfod /usr/local/bin/debuginfod

EXPOSE 8002
ENTRYPOINT ["/usr/local/bin/debuginfod"]
CMD ["-p", "8002"]
