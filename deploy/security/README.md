# Безопасность debuginfod-go

Меры защиты на уровне приложения, systemd и эксплуатации на целевых ОС.

## Path traversal (приложение)

Пакет `internal/pathsafe` проверяет пути перед отдачей файлов:

| Точка | Проверка |
|-------|----------|
| `GET /buildid/.../source/<path>` | Абсолютный путь, без `..` |
| `GET /buildid/.../section/<name>` | Имя секции без `/` и `..` |
| Файлы с диска (`file_path`) | Только под `DEBUGINFOD_SCAN_PATH` + `DEBUGINFOD_CACHE_DIR` |
| Члены архивов (`member_path`) | Без выхода за корень архива |
| Путь к архиву | Только под scan paths |

При нарушении: `400 Bad Request` (некорректный URL) или `403 Forbidden` (путь вне корней).

Разрешённые корни задаются автоматически из конфигурации при старте сервера.

## systemd hardening

В [debuginfod-go.service](../debuginfod-go.service) включены:

| Директива | Назначение |
|-----------|------------|
| `NoNewPrivileges` | Запрет повышения привилегий |
| `ProtectSystem=strict` | `/usr`, `/etc`, `/boot` только для чтения |
| `ProtectHome=true` | Изоляция `/home` |
| `PrivateTmp=true` | Отдельный `/tmp` |
| `ReadWritePaths` | Запись только в `/var/lib/debuginfod-go`, `/var/cache/debuginfod-go` |
| `RestrictAddressFamilies` | Сеть: UNIX, IPv4, IPv6 |

Конфиг `/etc/debuginfod-go/` читается через `EnvironmentFile` до применения sandbox.

### PostgreSQL unix-socket

Если `DEBUGINFOD_DATABASE_URL` использует сокет `/var/run/postgresql`, добавьте в unit:

```ini
BindReadOnlyPaths=/var/run/postgresql
```

### Ослабление hardening

Для отладки можно временно использовать drop-in:

```bash
sudo systemctl edit debuginfod-go
# [Service]
# ProtectSystem=false
```

## IMA и подписи пакетов

debuginfod-go **не проверяет IMA/EVM** отдаваемых ELF на лету — это задача ОС и политики доверия к scan paths.

### Подпись пакета debuginfod-go

Перед установкой `.deb` / `.rpm`:

```bash
# RPM (RedOS / CentOS)
rpm -K dist/debuginfod-go-*.rpm

# DEB (Astra / Ubuntu) — при наличии debsigs
dpkg-sig --verify dist/debuginfod-go_*.deb 2>/dev/null || echo "проверьте источник пакета"
```

Устанавливайте пакеты только из доверенного offline bundle или внутренней сборки CI.

### IMA appraisal для индексируемых путей

На хостах с включённым IMA (типично для hardened RHEL/Astra) можно ограничить доверие к debuginfo:

1. Включить IMA в enforcing (политика ОС).
2. Индексировать только `/usr/lib/debug` и доверенные кэши пакетов (`/var/cache/dnf`, `/var/cache/apt/archives`).
3. Не добавлять в `DEBUGINFOD_SCAN_PATH` world-writable каталоги.

Пример просмотра IMA на бинарнике:

```bash
getfattr -n security.ima -e hex /usr/bin/debuginfod 2>/dev/null || true
```

### fsverity (опционально)

Для неизменяемых debuginfo-файлов в read-only tree можно использовать fsverity на уровне ФС — вне scope сервера, см. документацию ядра.

## Рекомендации production

| Мера | Где |
|------|-----|
| nginx + TLS на периметре | [nginx/README.md](../nginx/README.md) |
| Basic Auth / mTLS | `DEBUGINFOD_BASIC_AUTH_*`, `DEBUGINFOD_TLS_*` |
| Zabbix + алерты | [zabbix/README.md](../zabbix/README.md) |
| Порт 8002 только localhost | firewall / nginx |
| `DEBUGINFOD_ZABBIX_KEY` | защита `/zabbix` |
| Backup | [backup/README.md](../backup/README.md) |

## Что не реализовано

- **Лимит размера архива** — не планируется (контроль на уровне scan paths и диска).
- **Проверка IMA при каждом HTTP-запросе** — делегировано ОС; scan только доверенных путей.
