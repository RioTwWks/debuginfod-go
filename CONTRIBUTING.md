# Внесение вклада

Спасибо за интерес к проекту! Ниже — процесс и ожидания по качеству кода.

## Перед началом

1. [TODO.md](./TODO.md) — возможно, задача уже описана.
2. [DEVELOPMENT.md](./DEVELOPMENT.md) — архитектура, эндпоинты, тесты.
3. [.cursor/rules.md](./.cursor/rules.md) — соглашения для Cursor AI.

## Процесс

1. **Fork** [RioTwWks/debuginfod-go](https://github.com/RioTwWks/debuginfod-go).
2. Ветка:
   - `feature/краткое-описание` — функциональность
   - `fix/issue-N` — баг
   - `docs/...` — документация
   - `cursor/<описание>-c951` — ветки Cloud Agent
3. Изменения + **тесты**.
4. Локальная проверка:

   ```bash
   go fmt ./...
   go vet ./...
   make test
   ```

5. **Pull Request** в `main`:
   - что сделано и зачем;
   - как проверить (`curl`, UI, GDB);
   - ссылка на issue / пункт TODO.

## Код-стайл

- `gofmt` / `goimports`.
- Godoc на экспортируемых символах (русский или английский — единообразно в файле).
- Без `panic` в библиотеках; ошибки возвращать явно.
- Логирование — `log/slog`, не `print`/`fmt.Println`.
- `internal/*` не экспортируется из модуля.
- Зависимости — осознанно; предпочитать stdlib.

## Тесты

- `*_test.go` рядом с кодом.
- `gcc` / `rpmbuild` — `t.Skip` если недоступны.
- Не коммитить: бинарник `debuginfod`, `*.sqlite`, `.debuginfod-cache/`.

## Документация

При изменении API, конфига или поведения обновите:

| Файл | Когда |
|------|-------|
| `README.md` | Пользовательская документация, конфиг, API |
| `DEVELOPMENT.md` | Архитектура, схема БД, dev workflow |
| `TODO.md` | Отметить `[x]` выполненные пункты |
| `.env.example` | Новые `DEBUGINFOD_*` переменные |
| `.cursor/rules.md` | Новые пакеты, эндпоинты, соглашения |
| `.cursor/mcp.json` | Новые projectScripts / hints |
| `deploy/zabbix/README.md` | Изменения `/zabbix` |

## CI

PR должен проходить GitHub Actions (`.github/workflows/ci.yml`): `vet`, `test -race`, `build`.

## Вопросы

[GitHub Issues](https://github.com/RioTwWks/debuginfod-go/issues) с тегом `question`.

## Лицензия

PR под лицензией MIT проекта.
