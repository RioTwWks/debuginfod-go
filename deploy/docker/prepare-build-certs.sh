#!/bin/sh
# Копирует доверенные CA и APT-ключи хоста в build context для Docker build.
set -e

ssl_dest=".docker-build/ssl-certs"
apt_dest=".docker-build/apt-trust"
rm -rf "$ssl_dest" "$apt_dest"
mkdir -p "$ssl_dest" "$apt_dest/trusted.gpg.d" "$apt_dest/keyrings"

if [ -f /etc/ssl/certs/ca-certificates.crt ]; then
	cp -a /etc/ssl/certs/ca-certificates.crt "$ssl_dest/ca-certificates.crt"
fi

if [ -d /etc/ssl/certs ]; then
	cp -a /etc/ssl/certs/. "$ssl_dest/" 2>/dev/null || true
fi

if [ -d /usr/local/share/ca-certificates ]; then
	mkdir -p "$ssl_dest/local-ca"
	cp -a /usr/local/share/ca-certificates/. "$ssl_dest/local-ca/" 2>/dev/null || true
fi

if [ -f /etc/apt/trusted.gpg ]; then
	cp -a /etc/apt/trusted.gpg "$apt_dest/trusted.gpg"
fi

if [ -d /etc/apt/trusted.gpg.d ]; then
	cp -a /etc/apt/trusted.gpg.d/. "$apt_dest/trusted.gpg.d/" 2>/dev/null || true
fi

if [ -d /usr/share/keyrings ]; then
	cp -a /usr/share/keyrings/. "$apt_dest/keyrings/" 2>/dev/null || true
fi

ssl_count=$(find "$ssl_dest" -type f 2>/dev/null | wc -l)
apt_count=$(find "$apt_dest" -type f 2>/dev/null | wc -l)
echo "Prepared ${ssl_count} certificate file(s) in ${ssl_dest} for Docker build"
echo "Prepared ${apt_count} APT trust file(s) in ${apt_dest} for Docker build"

if [ "$ssl_count" -eq 0 ]; then
	echo "WARN: no host certificates — HTTPS apt may fail; try APT_INSECURE=true" >&2
	touch "$ssl_dest/.keep"
fi

if [ "$apt_count" -eq 0 ]; then
	touch "$apt_dest/.keep"
fi
