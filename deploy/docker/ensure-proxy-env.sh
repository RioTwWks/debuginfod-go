#!/bin/sh
# Подхватывает системный прокси для docker compose build.
# Безопасен для source из sh/dash и make (не использует pipefail / source всего /etc/environment).
#
#   . deploy/docker/ensure-proxy-env.sh
#   deploy/docker/compose.sh -f docker-compose.postgres.yml up -d --build --wait

_read_env_var() {
	_file="$1"
	_key="$2"
	[ -f "$_file" ] || return 0
	_line=$(grep -E "^[[:space:]]*${_key}=" "$_file" 2>/dev/null | tail -1) || return 0
	_val=${_line#*=}
	# trim spaces and optional quotes
	_val=$(printf '%s' "$_val" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' \
		-e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//")
	[ -n "$_val" ] || return 0
	export "$_key=$_val"
}

_load_proxy_from_file() {
	_file="$1"
	_read_env_var "$_file" HTTP_PROXY
	_read_env_var "$_file" http_proxy
	_read_env_var "$_file" HTTPS_PROXY
	_read_env_var "$_file" https_proxy
	_read_env_var "$_file" NO_PROXY
	_read_env_var "$_file" no_proxy
}

if [ -z "${HTTP_PROXY:-}" ] && [ -z "${http_proxy:-}" ]; then
	_load_proxy_from_file /etc/environment
fi

for _f in /etc/profile.d/proxy.sh /etc/profile.d/proxies.sh /etc/profile.d/http_proxy.sh; do
	if [ -f "$_f" ]; then
		# shellcheck disable=SC1090
		. "$_f" 2>/dev/null || true
	fi
done

export HTTP_PROXY="${HTTP_PROXY:-${http_proxy:-}}"
export HTTPS_PROXY="${HTTPS_PROXY:-${https_proxy:-$HTTP_PROXY}}"
export NO_PROXY="${NO_PROXY:-${no_proxy:-}}"
export http_proxy="$HTTP_PROXY"
export https_proxy="$HTTPS_PROXY"
export no_proxy="$NO_PROXY"

if [ -n "$HTTP_PROXY" ]; then
	echo "Docker build proxy: HTTP_PROXY=$HTTP_PROXY"
else
	echo "Docker build proxy: not set (export HTTP_PROXY or configure /etc/environment)"
fi
