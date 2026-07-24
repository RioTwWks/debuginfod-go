# install-astra-apt.sh — подключает sources.list Astra внутри Docker build.
# Вызывается из Dockerfile при APT_PROFILE=astra.
set -e

if [ "${APT_PROFILE:-}" = "astra" ]; then
	if [ ! -f /astra-sources.list ]; then
		echo "missing /astra-sources.list" >&2
		exit 1
	fi
	cp /astra-sources.list /etc/apt/sources.list
	rm -f /etc/apt/sources.list.d/debian.sources 2>/dev/null || true
	echo "Using Astra Linux APT repositories"
elif [ -n "${APT_MIRROR:-}" ]; then
	sed -i "s|deb.debian.org|${APT_MIRROR}|g" /etc/apt/sources.list
	sed -i "s|security.debian.org|${APT_MIRROR}|g" /etc/apt/sources.list || true
	echo "Using APT mirror: ${APT_MIRROR}"
fi
