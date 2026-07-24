#!/bin/sh
# Настройка HTTP proxy для Docker daemon через systemd (Astra / старый Docker).
# daemon.json с "proxies" поддерживается только с Docker Engine 23+.
#
# Usage: sudo deploy/docker-compose/setup-docker-proxy.sh [http://proxy:port]
set -e

PROXY="${1:-http://192.168.250.193:3128}"
DROPIN_DIR="/etc/systemd/system/docker.service.d"
DROPIN_FILE="$DROPIN_DIR/http-proxy.conf"

if [ "$(id -u)" -ne 0 ]; then
	echo "Run as root: sudo $0 [proxy-url]" >&2
	exit 1
fi

mkdir -p "$DROPIN_DIR"
cat >"$DROPIN_FILE" <<EOF
[Service]
Environment="HTTP_PROXY=$PROXY"
Environment="HTTPS_PROXY=$PROXY"
Environment="NO_PROXY=localhost,127.0.0.1"
EOF

echo "Wrote $DROPIN_FILE"
echo "If /etc/docker/daemon.json contains \"proxies\" and docker fails to start, remove it:"
echo "  sudo rm -f /etc/docker/daemon.json"

systemctl daemon-reload
if systemctl restart docker; then
	echo "Docker restarted OK"
	docker version
else
	echo "Docker failed to start. Recovery:" >&2
	echo "  sudo rm -f /etc/docker/daemon.json" >&2
	echo "  sudo systemctl restart docker" >&2
	exit 1
fi
