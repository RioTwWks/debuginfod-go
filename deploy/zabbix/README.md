# Мониторинг debuginfod-go в Zabbix

Сервер отдаёт метрики по HTTP для **Zabbix HTTP agent** (JSONPath). Prometheus не используется.

> **Альтернатива для операторов:** встроенный Web UI на `/ui/` — дашборд со статистикой и поиском по build-id (без Zabbix).

## Endpoint

```http
GET /zabbix
Header: X-Zabbix-Token: <DEBUGINFOD_ZABBIX_KEY>   # если задан ключ
# или: GET /zabbix?key=<DEBUGINFOD_ZABBIX_KEY>
```

Пример ответа:

```json
{
  "uptime_seconds": 3600,
  "artifacts_total": 120,
  "artifacts_executable": 80,
  "artifacts_debuginfo": 40,
  "sources_total": 300,
  "scanned_files_total": 95,
  "last_scan_duration_ms": 1234,
  "last_scan_indexed": 5,
  "last_scan_skipped": 90,
  "last_scan_errors": 0,
  "http_requests_total": 5000,
  "http_2xx_total": 4980,
  "http_4xx_total": 15,
  "http_5xx_total": 5,
  "http_bytes_sent": 104857600,
  "federation_hits": 12,
  "federation_misses": 3,
  "cache_bytes": 52428800
}
```

Web UI (`/ui/api/stats`) возвращает похожие счётчики плюс `last_scan_finished_at`.

## Импорт template (рекомендуется)

```text
Configuration → Templates → Import → deploy/zabbix/template_debuginfod-go.xml
```

Template **debuginfod-go by HTTP** включает:

- HTTP agent items: `http_5xx_total`, `last_scan_errors`, `last_scan_duration_ms`, `artifacts_total`, `cache_bytes`
- Web scenario: `/healthz`
- Triggers: недоступность, рост 5xx, ошибки scan, долгий scan

Макросы на хосте:

| Макрос | Пример |
|--------|--------|
| `{$DEBUGINFOD.URL}` | `https://debuginfod.example.com` или `http://127.0.0.1:8002` |
| `{$DEBUGINFOD.ZABBIX_KEY}` | токен из `DEBUGINFOD_ZABBIX_KEY` |
| `{$DEBUGINFOD.SCAN_MAX_MS}` | `60000` |

**Actions** (email/Telegram): см. [actions.md](./actions.md).

## Настройка хоста в Zabbix

1. Host с интерфейсом Agent (или без агента).
2. Макрос `{$DEBUGINFOD.URL}` = `http://debuginfod-host:8002`.
3. При токене: `{$DEBUGINFOD.ZABBIX_KEY}`.

## Примеры items (HTTP agent)

| Name | URL | JSONPath |
|------|-----|----------|
| Artifacts total | `{$DEBUGINFOD.URL}/zabbix` | `$.artifacts_total` |
| Sources total | `{$DEBUGINFOD.URL}/zabbix` | `$.sources_total` |
| Scanned files | `{$DEBUGINFOD.URL}/zabbix` | `$.scanned_files_total` |
| Last scan duration ms | `{$DEBUGINFOD.URL}/zabbix` | `$.last_scan_duration_ms` |
| Last scan errors | `{$DEBUGINFOD.URL}/zabbix` | `$.last_scan_errors` |
| HTTP 5xx total | `{$DEBUGINFOD.URL}/zabbix` | `$.http_5xx_total` |
| Federation misses | `{$DEBUGINFOD.URL}/zabbix` | `$.federation_misses` |
| Cache bytes | `{$DEBUGINFOD.URL}/zabbix` | `$.cache_bytes` |

### Headers (если задан ключ)

```
X-Zabbix-Token: {$DEBUGINFOD.ZABBIX_KEY}
```

## Триггеры

Включены в [template_debuginfod-go.xml](./template_debuginfod-go.xml). Ручная настройка (без template):

| Триггер | Условие |
|---------|---------|
| debuginfod down | web scenario `/healthz` ≠ 200 |
| Много 5xx | `last(...http_5xx_total)` растёт |
| Долгий scan | `last_scan_duration_ms > 60000` |
| Ошибки scan | `last_scan_errors > 0` |

## Health check

```
URL: {$DEBUGINFOD.URL}/healthz
Код: 200, тело содержит: ok
```

## Web UI (ручная проверка)

```bash
curl http://localhost:8002/ui/api/stats
curl 'http://localhost:8002/ui/api/search?q=dead'
```

Отключить UI: `DEBUGINFOD_UI_ENABLED=false`.

## Переменные окружения

```env
DEBUGINFOD_ZABBIX_KEY=your-secret-token
DEBUGINFOD_UI_ENABLED=true
```

Без ключа `/zabbix` публичен — в production задайте ключ или ограничьте firewall.

## systemd

Unit-файл: [../debuginfod-go.service](../debuginfod-go.service).
