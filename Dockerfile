# Сборка debuginfod-go (Debian — совместимо с Astra/Ubuntu).
# На Astra без доступа к deb.debian.org используйте Dockerfile.prebuilt (см. deploy/docker/README.md).
# За корпоративным прокси: export HTTP_PROXY/HTTPS_PROXY или deploy/docker/ensure-proxy-env.sh

ARG DEBIAN_SUITE=bookworm
FROM golang:1.21-${DEBIAN_SUITE} AS builder

ARG APT_PROFILE=
ARG APT_MIRROR=
ARG APT_INSECURE=false
ARG HTTP_PROXY=
ARG HTTPS_PROXY=
ARG NO_PROXY=

ENV HTTP_PROXY=${HTTP_PROXY} \
	HTTPS_PROXY=${HTTPS_PROXY} \
	NO_PROXY=${NO_PROXY} \
	http_proxy=${HTTP_PROXY} \
	https_proxy=${HTTPS_PROXY} \
	no_proxy=${NO_PROXY}

COPY .docker-build/ssl-certs/ /host-ssl-certs/
COPY deploy/docker/sources.astra-1.7.list /astra-sources.list
COPY deploy/docker/install-astra-apt.sh /install-astra-apt.sh
COPY deploy/docker/configure-proxy.sh /configure-proxy.sh
COPY deploy/docker/install-host-certs.sh /install-host-certs.sh

RUN chmod +x /install-astra-apt.sh /configure-proxy.sh /install-host-certs.sh \
	&& APT_INSECURE="${APT_INSECURE}" /configure-proxy.sh \
	&& /install-host-certs.sh \
	&& APT_PROFILE="${APT_PROFILE}" APT_MIRROR="${APT_MIRROR}" /install-astra-apt.sh \
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

ARG APT_PROFILE=
ARG APT_MIRROR=
ARG APT_INSECURE=false
ARG HTTP_PROXY=
ARG HTTPS_PROXY=
ARG NO_PROXY=

ENV HTTP_PROXY=${HTTP_PROXY} \
	HTTPS_PROXY=${HTTPS_PROXY} \
	NO_PROXY=${NO_PROXY} \
	http_proxy=${HTTP_PROXY} \
	https_proxy=${HTTPS_PROXY} \
	no_proxy=${NO_PROXY}

COPY .docker-build/ssl-certs/ /host-ssl-certs/
COPY deploy/docker/sources.astra-1.7.list /astra-sources.list
COPY deploy/docker/install-astra-apt.sh /install-astra-apt.sh
COPY deploy/docker/configure-proxy.sh /configure-proxy.sh
COPY deploy/docker/install-host-certs.sh /install-host-certs.sh

RUN chmod +x /install-astra-apt.sh /configure-proxy.sh /install-host-certs.sh \
	&& APT_INSECURE="${APT_INSECURE}" /configure-proxy.sh \
	&& /install-host-certs.sh \
	&& APT_PROFILE="${APT_PROFILE}" APT_MIRROR="${APT_MIRROR}" /install-astra-apt.sh \
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
