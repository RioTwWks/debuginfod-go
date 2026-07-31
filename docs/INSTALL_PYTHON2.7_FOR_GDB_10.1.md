# Шпаргалка: установка Python 2.7 (с UCS4) для GDB на Astra Linux 1.8.4

> **Цель:** Установить Python 2.7 с поддержкой Unicode UCS4 и динамической библиотекой `libpython2.7.so.1.0`, чтобы GDB (собранный под UCS4) мог работать без ошибок.

---

## 📌 Вариант 1: Сборка из исходников (проверено)

Этот вариант гарантирует нужную конфигурацию, независимо от репозиториев.

### 1. Установить зависимости для сборки
```bash
sudo apt update
sudo apt install -y make build-essential libssl-dev zlib1g-dev libbz2-dev \
libreadline-dev libsqlite3-dev wget curl llvm libncursesw5-dev xz-utils \
tk-dev libxml2-dev libxmlsec1-dev libffi-dev liblzma-dev
```

### 2. Скачать исходный код Python 2.7.18 (последняя версия 2.7)
```bash
wget https://www.python.org/ftp/python/2.7.18/Python-2.7.18.tgz
tar -xzf Python-2.7.18.tgz
cd Python-2.7.18
```

### 3. Сконфигурировать с нужными флагами
```bash
./configure --prefix=/usr/local/python2.7 --enable-shared --enable-unicode=ucs4
```

### 4. Собрать и установить
```bash
make
sudo make install
```

### 5. Зарегистрировать библиотеку в системе
```bash
sudo ldconfig
```

Если библиотека не обнаружена автоматически, создайте файл конфигурации:
```bash
echo "/usr/local/python2.7/lib" | sudo tee /etc/ld.so.conf.d/python2.7.conf
sudo ldconfig
```

### 6. Проверить
```bash
# Версия Python
/usr/local/python2.7/bin/python2.7 --version

# Проверка Unicode-режима (должно быть 1114111)
/usr/local/python2.7/bin/python2.7 -c "import sys; print(sys.maxunicode)"

# Проверка GDB
ldd $(which gdb) | grep python     # должен видеть libpython2.7.so.1.0
gdb --version                      # без ошибок
```

---

## 📌 Вариант 2: Установка из репозитория `frozen` (если подойдёт)

В репозитории Astra есть пакеты Python 2.7, но неизвестно, с UCS2 или UCS4. Проверьте:

```bash
# Добавить репозиторий (если ещё не добавлен)
echo "deb https://download.astralinux.ru/astra/frozen/orel-1.11/repository-update/ ./" | sudo tee -a /etc/apt/sources.list
sudo apt update
sudo apt install python2.7 python2.7-dev
```

После установки проверьте `sys.maxunicode`:
```bash
python2.7 -c "import sys; print(sys.maxunicode)"
```
* Если выведет `1114111` → UCS4, GDB заработает (если библиотека найдена, выполните `sudo ldconfig`).
* Если выведет `65535` → UCS2, GDB выдаст ошибку символа. Тогда используйте **Вариант 1**.

---

## 🔧 Дополнительные полезные команды

### Проверка, какая версия Unicode используется в Python
```bash
python2.7 -c "import sys; print(sys.maxunicode)"
```
* `1114111` = UCS4
* `65535`   = UCS2

### Принудительная подгрузка библиотеки (временное решение)
Если GDB всё ещё не видит библиотеку, можно указать путь через переменную окружения:
```bash
export LD_LIBRARY_PATH=/usr/local/python2.7/lib:$LD_LIBRARY_PATH
gdb --version
```
Для постоянного использования добавьте эту строку в `~/.bashrc`.

### Проверка зависимостей GDB
```bash
ldd $(which gdb) | grep python
```
Должен показывать путь к `libpython2.7.so.1.0` и статус `found`.

---

## ⚠️ Важное примечание
Python 2.7 больше не поддерживается. Используйте его только для совместимости со старыми инструментами (например, GDB). Для новых проектов переходите на Python 3.

---

## 📝 Краткий конспект (для быстрого копирования)

```bash
# Скачать и собрать Python 2.7 с UCS4
wget https://www.python.org/ftp/python/2.7.18/Python-2.7.18.tgz
tar -xzf Python-2.7.18.tgz
cd Python-2.7.18
./configure --prefix=/usr/local/python2.7 --enable-shared --enable-unicode=ucs4
make -j$(nproc)
sudo make install
sudo ldconfig

# Проверка
/usr/local/python2.7/bin/python2.7 -c "import sys; print(sys.maxunicode)"  # 1114111
gdb --version
```

---

Готово!
