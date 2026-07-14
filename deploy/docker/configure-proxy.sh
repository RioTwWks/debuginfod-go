#!/bin/sh
# Настраивает прокси для apt внутри Docker build (HTTP_PROXY / http_proxy).
set -eu

http_proxy_val="${HTTP_PROXY:-${http_proxy:-}}"
https_proxy_val="${HTTPS_PROXY:-${https_proxy:-$http_proxy_val}}"

if [ -n "$http_proxy_val" ]; then
	mkdir -p /etc/apt/apt.conf.d
	{
		printf 'Acquire::http::Proxy "%s";\n' "$http_proxy_val"
		if [ -n "$https_proxy_val" ]; then
			printf 'Acquire::https::Proxy "%s";\n' "$https_proxy_val"
		fi
	} > /etc/apt/apt.conf.d/99proxy
	echo "APT proxy: $http_proxy_val"
fi

if [ "${APT_INSECURE:-}" = "true" ]; then
	mkdir -p /etc/apt/apt.conf.d
	{
		echo 'Acquire::https::Verify-Peer "false";'
		echo 'Acquire::https::Verify-Host "false";'
	} >> /etc/apt/apt.conf.d/99proxy
	echo "APT: HTTPS certificate verification disabled (APT_INSECURE=true)"
fi
