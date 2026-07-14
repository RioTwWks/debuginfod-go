#!/bin/sh
set -e

# Пользователь и каталоги для systemd-сервиса.
if ! getent group debuginfod >/dev/null 2>&1; then
	if command -v groupadd >/dev/null 2>&1; then
		groupadd --system debuginfod
	else
		addgroup --system debuginfod
	fi
fi

if ! getent passwd debuginfod >/dev/null 2>&1; then
	if command -v useradd >/dev/null 2>&1; then
		useradd --system -g debuginfod -d /var/lib/debuginfod-go -s /sbin/nologin debuginfod
	else
		adduser --system --ingroup debuginfod --home /var/lib/debuginfod-go --no-create-home debuginfod
	fi
fi

mkdir -p /var/lib/debuginfod-go /var/cache/debuginfod-go /etc/debuginfod-go
chown debuginfod:debuginfod /var/lib/debuginfod-go /var/cache/debuginfod-go

if [ ! -f /etc/debuginfod-go/debuginfod-go.env ]; then
	cp /etc/debuginfod-go/debuginfod-go.env.example /etc/debuginfod-go/debuginfod-go.env
fi
chown root:debuginfod /etc/debuginfod-go/debuginfod-go.env
chmod 640 /etc/debuginfod-go/debuginfod-go.env

if command -v systemctl >/dev/null 2>&1; then
	systemctl daemon-reload
	systemctl enable debuginfod-go.service 2>/dev/null || true
fi
