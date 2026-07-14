#!/bin/sh
# Копирует CA-сертификаты хоста в build context (debian:*-slim не содержит ca-certificates).
set -eu

dest="/etc/ssl/certs"
mkdir -p "$dest"

if [ -f /host-ssl-certs/ca-certificates.crt ]; then
	cp /host-ssl-certs/ca-certificates.crt "$dest/ca-certificates.crt"
elif [ -d /host-ssl-certs ] && [ -n "$(ls -A /host-ssl-certs 2>/dev/null)" ]; then
	cp -a /host-ssl-certs/. "$dest/"
else
	echo "No host SSL certs in build context"
	exit 0
fi

if [ -f "$dest/ca-certificates.crt" ]; then
	bytes=$(wc -c < "$dest/ca-certificates.crt")
	echo "Installed host ca-certificates.crt (${bytes} bytes)"
else
	count=$(find "$dest" -type f 2>/dev/null | wc -l)
	echo "Installed ${count} SSL cert files from host"
fi
