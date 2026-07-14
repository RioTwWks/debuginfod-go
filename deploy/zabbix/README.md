# Мониторинг debuginfod-go в Zabbix

Сервер отдаёт метрики по HTTP для **Zabbix HTTP agent** (JSONPath). Prometheus не требуется.

## Endpoint

```http
GET /zabbix
Header: X-Zabbix-Token: <DEBUGINFOD_ZABBIX_KEY>   # если задан ключ
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

## Настройка хоста в Zabbix

1. Создайте host с интерфейсом Agent (любой) или без агента.
2. Добавьте макрос `{$DEBUGINFOD.URL}` = `http://debuginfod-host:8002`.
3. При использовании токена: `{$DEBUGINFOD.ZABBIX_KEY}`.

## Примеры items (HTTP agent)

| Name | Type | URL | JSONPath |
|------|------|-----|----------|
| Artifacts total | HTTP agent | `{$DEBUGINFOD.URL}/zabbix` | `$.artifacts_total` |
| Last scan duration ms | HTTP agent | `{$DEBUGINFOD.URL}/zabbix` | `$.last_scan_duration_ms` |
| HTTP 5xx total | HTTP agent | `{$DEBUGINFOD.URL}/zabbix` | `$.http_5xx_total` |
| Cache bytes | HTTP agent | `{$DEBUGINFOD.URL}/zabbix` | `$.cache_bytes` |

### Headers (если задан ключ)

```
X-Zabbix-Token: {$DEBUGINFOD.ZABBIX_KEY}
```

## Триггеры (примеры)

| Триггер | Условие |
|---------|---------|
| debuginfod down | `nodata(/zabbix,5m)=1` или web scenario на `/healthz` |
| Много 5xx | `last(/debuginfod/zabbix.http_5xx_total)>10` за 5m |
| Долгий scan | `last(/debuginfod/zabbix.last_scan_duration_ms)>60000` |

## Health check

Для простой доступности используйте отдельный item:

```
URL: {$DEBUGINFOD.URL}/healthz
Ожидаемый код: 200
Тело содержит: ok
```

## Переменные окружения

```env
DEBUGINFOD_ZABBIX_KEY=your-secret-token
```

Без ключа endpoint `/zabbix` публичен — в production задайте ключ или ограничьте firewall.
