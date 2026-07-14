#!/bin/sh
# Копирует GPG/distrust ключи APT с хоста (нужно для APT_PROFILE=astra).
set -eu

if [ ! -d /host-apt-trust ]; then
	exit 0
fi

if [ -f /host-apt-trust/trusted.gpg ]; then
	cp /host-apt-trust/trusted.gpg /etc/apt/trusted.gpg
fi

if [ -d /host-apt-trust/trusted.gpg.d ] && [ -n "$(ls -A /host-apt-trust/trusted.gpg.d 2>/dev/null)" ]; then
	mkdir -p /etc/apt/trusted.gpg.d
	cp -a /host-apt-trust/trusted.gpg.d/. /etc/apt/trusted.gpg.d/
fi

if [ -d /host-apt-trust/keyrings ] && [ -n "$(ls -A /host-apt-trust/keyrings 2>/dev/null)" ]; then
	mkdir -p /usr/share/keyrings
	cp -a /host-apt-trust/keyrings/. /usr/share/keyrings/
fi

count=$(find /host-apt-trust -type f 2>/dev/null | wc -l)
if [ "$count" -gt 0 ]; then
	echo "Installed host APT trust keys (${count} files)"
fi
