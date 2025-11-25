# PR Reviewer Assignment Service

Сервис для автоматического назначения ревьюеров на Pull Request'ы внутри команд. Реализованы все эндпоинты из `openapi.yml`, храним данные в PostgreSQL, есть миграции и контейнеризация через Docker Compose.

## Стек

- Go 1.24
- Gin (HTTP API)
- PostgreSQL 15
- pgx/v5 (драйвер и пул)
- Docker + docker-compose
- golangci-lint

## Архитектура каталогов

- `cmd/` — точка входа http-сервера.
- `internal/http` — HTTP-обработчики по спецификации OpenAPI.
- `internal/service` — бизнес-логика и выбор ревьюеров.
- `internal/storage` — доступ к БД, транзакции, миграции.
- `internal/db/sql` — SQL-миграции.
- `internal/domain` — модели предметной области и ошибки.

## Требования

- Go ≥ 1.24
- PostgreSQL 15 (локально или в docker-compose)
- `golangci-lint v1.64.8`
- Docker + docker-compose (для контейнерного запуска)

## Запуск через Docker Compose

```bash
make compose-up
```

Команда поднимет PostgreSQL и сервис в одной сети. В `.env.example` уже прописана строка подключения к контейнерной базе. Для остановки:

```bash
make compose-down
```

## Переменные окружения

- `DATABASE_URL` — строка подключения к PostgreSQL (обязательна).
- `PORT` — порт HTTP сервера (по умолчанию 8080).
- `LOG_LEVEL` — `debug|info|warn|error`.

## Тесты

```bash
go test ./...
```


## Линтер
  ```bash
  make lint
  ```

## CI/CD

Workflow `.github/workflows/ci.yml` содержит два джоба:

1. `lint` — устанавливает Go 1.24, ставит `golangci-lint v1.64.8` и выполняет `make lint`.
2. `test` — после успешного линтинга запускает `go test ./...`.

Пайплайн срабатывает на `push` и `pull_request` в ветку `main`.

## Допущения

- В задании не было требований к полноценной системе аутентификации, поэтому она не была реализована.
- Файл `.env` был добавлен в репозиторий по требованию из письма, отправленного на почту.
