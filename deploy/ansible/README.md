# Ansible: раскатка debuginfod-go на Astra/Ubuntu (deb) и RedOS/CentOS (rpm).

## Требования

- Ansible 2.14+
- SSH-доступ к целевым хостам
- Собранный пакет: `make package` → `dist/*.deb` или `dist/*.rpm`
- Для **оффлайн** целевых хостов: сначала `make offline-bundle-*`, пакет в bundle уже с зависимостями

## Быстрый старт

```bash
# 1. Собрать пакет на control-node
make package VERSION=0.1.0

# 2. Inventory
cp inventory.example.yml inventory.yml
# отредактировать хосты и debuginfod_package_path

# 3. Раскатка
cd deploy/ansible
ansible-playbook -i inventory.yml playbooks/site-deb.yml   # Astra / Ubuntu
ansible-playbook -i inventory.yml playbooks/site-rpm.yml   # RedOS / CentOS
ansible-playbook -i inventory.yml playbooks/site.yml       # все группы debuginfod
```

## Переменные роли

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `debuginfod_package_path` | — | Путь к `.deb`/`.rpm` на control-node |
| `debuginfod_package_url` | — | Альтернатива: URL пакета |
| `debuginfod_scan_path` | `/usr/lib/debug` | Пути индексации |
| `debuginfod_port` | `8002` | HTTP-порт (localhost; снаружи — nginx) |
| `debuginfod_zabbix_key` | `""` | Токен `/zabbix` |
| `debuginfod_firewall_enabled` | `false` | UFW / firewalld для порта сервиса |
| `debuginfod_firewall_allowed_cidrs` | private nets | Сети для firewall |

Полный список: [roles/debuginfod-go/defaults/main.yml](./roles/debuginfod-go/defaults/main.yml)

## Примеры inventory

### Astra / Ubuntu

```yaml
debuginfod_deb:
  hosts:
    debuginfod-01:
      ansible_host: 10.0.1.50
  vars:
    debuginfod_package_path: /build/dist/debuginfod-go_0.1.0_amd64.deb
    debuginfod_scan_path: /usr/lib/debug,/var/cache/apt/archives
```

### RedOS / CentOS

```yaml
debuginfod_rpm:
  hosts:
    debuginfod-01:
      ansible_host: 10.0.1.60
  vars:
    debuginfod_package_path: /build/dist/debuginfod-go-0.1.0-1.x86_64.rpm
    debuginfod_scan_path: /usr/lib/debug,/var/cache/dnf
```

## Оффлайн-целевые хосты

1. На build-хосте: `make offline-bundle-deb` (или `-rpm`)
2. Перенести bundle на целевой хост и установить: `sudo ./install-offline.sh`
3. Ansible использовать для **конфигурации** (только `common.yml` задачи) — или задать `debuginfod_package_path` к локальному `.deb` в bundle

Для полностью изолированных хостов без Ansible: достаточно `install-offline.sh` + ручная правка `/etc/debuginfod-go/debuginfod-go.env`.

## Проверка

```bash
ansible debuginfod -m uri -a "url=http://127.0.0.1:8002/healthz return_content=yes"
```

## Связанные компоненты

- [nginx](../nginx/README.md) — reverse proxy, TLS, ACL
- [zabbix](../zabbix/README.md) — мониторинг, template
- [offline](../offline/README.md) — bundle без интернета
