#!/usr/bin/env bash
# Копирует /etc/ssl/certs с хоста в .docker-build/ssl-certs для Docker build.
set -euo pipefail

dest=".docker-build/ssl-certs"
rm -rf "$dest"
mkdir -p "$dest"

if [ -f /etc/ssl/certs/ca-certificates.crt ]; then
	cp -a /etc/ssl/certs/ca-certificates.crt "$dest/ca-certificates.crt"
fi

if [ -d /etc/ssl/certs ]; then
	cp -a /etc/ssl/certs/. "$dest/" 2>/dev/null || true
fi

if [ -d /usr/local/share/ca-certificates ]; then
	mkdir -p "$dest/local-ca"
	cp -a /usr/local/share/ca-certificates/. "$dest/local-ca/" 2>/dev/null || true
fi

count=$(find "$dest" -type f 2>/dev/null | wc -l)
echo "Prepared ${count} certificate file(s) in ${dest} for Docker build"

if [ "$count" -eq 0 ]; then
	echo "WARN: no host certificates — HTTPS apt may fail; try APT_INSECURE=true" >&2
	touch "$dest/.keep"
fi
