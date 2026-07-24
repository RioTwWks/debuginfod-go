# PostgreSQL для тестов — из APT (Astra / Debian), без pull postgres:* с Docker Hub.
#
# Astra 1.7 (как make docker-astra):
#   make docker-prep
#   . deploy/docker/ensure-proxy-env.sh
#   deploy/docker/compose.sh -f docker-compose.postgres.yml up -d --build --wait
#
# Требуется локально debian:buster-slim (кэш от docker-astra) или доступ к Hub один раз.

ARG DEBIAN_SUITE=buster
FROM debian:${DEBIAN_SUITE}-slim

ARG APT_PROFILE=astra
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

ENV PGDATA=/var/lib/postgresql/data \
	POSTGRES_USER=debuginfod \
	POSTGRES_PASSWORD=debuginfod \
	POSTGRES_DB=debuginfod

COPY .docker-build/ssl-certs/ /host-ssl-certs/
COPY .docker-build/apt-trust/ /host-apt-trust/
COPY deploy/docker/sources.astra-1.7.list /astra-sources.list
COPY deploy/docker/install-astra-apt.sh /install-astra-apt.sh
COPY deploy/docker/configure-proxy.sh /configure-proxy.sh
COPY deploy/docker/install-host-certs.sh /install-host-certs.sh
COPY deploy/docker/install-host-apt-trust.sh /install-host-apt-trust.sh
COPY deploy/postgresql/docker-entrypoint.sh /docker-entrypoint.sh

RUN chmod +x /install-astra-apt.sh /configure-proxy.sh /install-host-certs.sh \
		/install-host-apt-trust.sh /docker-entrypoint.sh \
	&& APT_INSECURE="${APT_INSECURE}" /configure-proxy.sh \
	&& /install-host-certs.sh \
	&& /install-host-apt-trust.sh \
	&& APT_PROFILE="${APT_PROFILE}" APT_MIRROR="${APT_MIRROR}" /install-astra-apt.sh \
	&& apt-get -o Acquire::Retries=5 update \
	&& DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
		postgresql \
		postgresql-client \
		ca-certificates \
		locales \
	&& rm -rf /var/lib/apt/lists/* \
	&& mkdir -p "$PGDATA" \
	&& chown -R postgres:postgres /var/lib/postgresql

EXPOSE 5432
VOLUME /var/lib/postgresql/data

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["postgres"]
