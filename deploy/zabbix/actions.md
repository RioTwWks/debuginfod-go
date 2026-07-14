# Рекомендуемые Actions в Zabbix (настраиваются вручную после импорта template)

Template содержит **triggers**; **actions** зависят от ваших media types (email, Telegram, Slack).

## 1. Создать Action

**Configuration → Actions → Trigger actions → Create action**

| Поле | Значение |
|------|----------|
| Name | `debuginfod-go alert` |
| Conditions | Trigger severity >= Warning AND Trigger name contains `debuginfod` |

## 2. Operations

| Operation | Детали |
|-----------|--------|
| Send message | Default media (email/Telegram) |
| Subject | `debuginfod alert: {TRIGGER.NAME}` |
| Message | `{HOST.NAME}: {TRIGGER.NAME}\n{TRIGGER.URL}\nLast value: {ITEM.LASTVALUE}` |

## 3. Recovery operations

Включить уведомление о восстановлении (Recovery operations → Send message).

## Триггеры в template

| Триггер | Severity | Условие |
|---------|----------|---------|
| сервис недоступен (healthz) | High | web scenario `/healthz` failed 3 раза |
| рост HTTP 5xx | Average | `change(http_5xx_total) > 0` |
| ошибки scan | Warning | `last_scan_errors > 0` |
| долгий scan | Warning | `last_scan_duration_ms > {$DEBUGINFOD.SCAN_MAX_MS}` |

## Импорт template

```text
Configuration → Templates → Import → template_debuginfod-go.xml
```

После импорта:

1. Link template к host
2. Переопределить макросы `{$DEBUGINFOD.URL}`, `{$DEBUGINFOD.ZABBIX_KEY}`
3. Создать Action (см. выше)
