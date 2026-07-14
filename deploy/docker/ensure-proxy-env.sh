#!/usr/bin/env bash
# Подхватывает системный прокси для docker compose build (из /etc/environment и profile.d).
set -euo pipefail

if [ -z "${HTTP_PROXY:-}" ] && [ -z "${http_proxy:-}" ] && [ -f /etc/environment ]; then
	set -a
	# shellcheck disable=SC1091
	source /etc/environment
	set +a
fi

for f in /etc/profile.d/*.sh; do
	[ -f "$f" ] || continue
	case "$f" in
		*/proxy.sh|*/proxies.sh|*/http_proxy.sh) ;;
		*) continue ;;
	esac
	set -a
	# shellcheck disable=SC1090
	source "$f"
	set +a
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
