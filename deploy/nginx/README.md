# nginx reverse proxy для debuginfod-go

TLS-терминация, ACL и rate limiting на периметре. Сервис слушает **только localhost:8002**; снаружи — nginx на 443.

## Установка

### Debian / Ubuntu / Astra

```bash
sudo apt install nginx
sudo cp debuginfod-go.conf /etc/nginx/sites-available/
sudo ln -sf /etc/nginx/sites-available/debuginfod-go.conf /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

### RedOS / CentOS

```bash
sudo dnf install nginx
sudo cp debuginfod-go.conf /etc/nginx/conf.d/debuginfod-go.conf
sudo nginx -t && sudo systemctl enable --now nginx
```

## Настройка

1. Замените `debuginfod.example.com` на FQDN хоста.
2. Укажите пути TLS-сертификатов или используйте certbot:
   ```bash
   sudo certbot --nginx -d debuginfod.example.com
   ```
3. **ACL:** раскомментируйте `allow`/`deny` в конфиге для ограничения по IP.
4. **Rate limit:** по умолчанию `20 req/s` на IP (`limit_req_zone`).

## Рекомендуемая схема

```text
Клиент (GDB) → nginx:443 → 127.0.0.1:8002 (debuginfod-go)
```

- Firewall: открыть **443**, порт **8002** не публиковать (`debuginfod_firewall_enabled: false` в Ansible).
- Basic Auth / mTLS приложения можно дублировать или заменить на nginx `auth_basic` / client certificates.

## Проверка

```bash
curl -k https://debuginfod.example.com/healthz
curl -k https://debuginfod.example.com/ui/api/stats
```

## Связанные файлы

- [deploy/ansible/README.md](../ansible/README.md) — раскатка сервиса
- [deploy/zabbix/README.md](../zabbix/README.md) — мониторинг (URL через nginx или напрямую :8002)
