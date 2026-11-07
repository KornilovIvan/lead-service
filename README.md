# lead-service

HTTP-сервис приёма лидов и асинхронной выгрузки в **Bitrix24**.

Принимает лидов, хранит их в PostgreSQL, кладёт задачу в очередь `jobs`, а воркер синхронизирует лид с Bitrix. Лимит запросов к Bitrix контролируется через Redis, зависшие задачи возвращает reaper. Дополнительно есть импорт, правила квалификации, экспорт в Google Sheets/вебхук и метрики Prometheus.

## Возможности

- Клиенты и лиды: создание, чтение, обновление, список с keyset-пагинацией.
- Импорт лидов из CSV и формы.
- Правила квалификации по `name` / `phone` / `email` с результатом `qualified` или `rejected`.
- Экспорт лида в Google Sheets или на вебхук.
- Асинхронная синхронизация с Bitrix через очередь `jobs`.
- Идемпотентность по `ORIGIN_ID` (ID лида в этом сервисе).
- Rate limit Bitrix на Redis: фиксированное окно, один атомарный `EVAL`.
- Backoff 30 секунд для временных ошибок и повторный захват задач.
- Reaper для задач, зависших в `in_progress`.
- Метрика `sync_jobs_failed_total` и `GET /metrics`.

## Архитектура

![Архитектура lead-service](docs/architecture.png)

Исходник схемы — `docs/architecture.svg`, PNG — для GitHub и просмотра без SVG-рендера.

Слои:

```
cmd/app              — сборка зависимостей, HTTP-сервер, воркеры, reaper
cmd/bitrixfake       — локальная заглушка Bitrix REST
cmd/sheetsfake       — заглушка Google Sheets
cmd/webhookfake      — заглушка вебхука
internal/handler     — chi-роуты, JSON, HTTP-коды
internal/service     — валидация, бизнес-логика, crm_label
internal/repository  — SQL через sqlx, транзакции
internal/worker      — воркер синхронизации и reaper
internal/bitrix      — клиент Bitrix
internal/ratelimit   — лимитер на Redis
internal/metrics     — Prometheus-счётчик
internal/model       — доменные структуры
migrations           — схема PostgreSQL
```

## Быстрый старт

### Весь стек в Docker

```bash
docker compose up -d --build postgres redis
make migrate-up
docker compose up -d --build
curl -sS http://localhost:8080/health
# {"status":"ok"}
```

Поднимаются:

| Сервис | Порт |
|---|---|
| `app` | 8080 |
| `bitrixfake` | 8081 |
| `sheetsfake` | 8082 |
| `webhookfake` | 8083 |
| `postgres` | 5432 |
| `redis` | 6379 |

`app` получает `DATABASE_URL`, `BITRIX_BASE_URL`, `SHEETS_BASE_URL`, `REDIS_URL` и работает внутри compose-сети.

### Локально

```bash
make up            # postgres + redis
make migrate-up

docker compose up -d bitrixfake sheetsfake webhookfake

export DATABASE_URL='postgres://leads:leads@localhost:5432/leads?sslmode=disable'
export HTTP_ADDR=':8080'
export BITRIX_BASE_URL='http://localhost:8081/rest/1/token'
export SHEETS_BASE_URL='http://localhost:8082'
export REDIS_URL='redis://localhost:6379/0'

go run ./cmd/app
```

## Переменные окружения

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `DATABASE_URL` | — | DSN PostgreSQL, обязательна |
| `HTTP_ADDR` | `:8080` | адрес HTTP-сервера |
| `REDIS_URL` | — | Redis для лимитера, обязательна |
| `BITRIX_BASE_URL` | — | базовый URL вебхука Bitrix |
| `BITRIX_TIMEOUT` | `5s` | таймаут одного запроса к Bitrix |
| `BITRIX_LIMIT` | `2` | сколько вызовов разрешено в окне |
| `BITRIX_WINDOW_SECONDS` | `1` | длина окна rate limit |
| `SHEETS_BASE_URL` | — | базовый URL сервиса Sheets |
| `WEBHOOK_TIMEOUT` | `3s` | таймаут экспортного вебхука |
| `WORKER_CONCURRENCY` | `4` | число горутин-воркеров |
| `REAPER_EVERY` | `1m` | как часто запускается reaper |
| `REAPER_AFTER` | `30s` | порог «зависшей» задачи, должен быть больше `BITRIX_TIMEOUT` |

## API

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/health` | проверка живости |
| `GET` | `/metrics` | метрики Prometheus |
| `POST` | `/api/v1/customers` | создать клиента |
| `GET` | `/api/v1/customers/{id}` | получить клиента |
| `POST` | `/api/v1/leads` | создать лида |
| `GET` | `/api/v1/leads` | список по `customer_id`, `limit` (1..100, по умолчанию 20), `status`, `after_created_at` + `after_id` |
| `GET` | `/api/v1/leads/{id}` | получить лида |
| `PATCH` | `/api/v1/leads/{id}` | обновить `name`, `phone`, `email`, `status` |
| `POST` | `/api/v1/leads/{id}/qualify` | применить правила квалификации |
| `POST` | `/api/v1/leads/{id}/retry` | поставить задачу синхронизации заново |
| `POST` | `/api/v1/imports` | импорт (`csv` или `form`) |
| `GET` | `/api/v1/imports/{id}` | статус импорта |
| `POST` | `/api/v1/qualification-rules` | создать правило |
| `GET` | `/api/v1/qualification-rules` | правила по `customer_id` |
| `POST` | `/api/v1/exports` | экспорт (`sheets` или `webhook`) |
| `GET` | `/api/v1/exports` | экспорты по `customer_id` |

`GET /api/v1/leads/{id}` и каждый элемент списка содержат вычисляемое поле `crm_label`.

## Как работает синхронизация

1. `POST /api/v1/leads` в одной транзакции вставляет лида и задачу в `jobs`.
2. Воркер захватывает задачу:
   ```sql
   UPDATE jobs SET status = 'in_progress', locked_at = now()
   WHERE id = (SELECT id FROM jobs WHERE status = 'queued' AND run_at <= now()
               ORDER BY run_at FOR UPDATE SKIP LOCKED LIMIT 1)
   RETURNING id, lead_id, locked_at;
   ```
   Лок коммитится до HTTP-вызова.
3. Проверяется лимит Redis. Нет слота — задача возвращается в `queued` с `run_at = now() + 30s`.
4. `FindByOriginID("leadID")`; если не найден — один `AddLead`.
5. Успех одной транзакцией: `leads.bitrix_id` и `jobs.status = 'done'` с проверкой `locked_at`.

Классификация ошибок:

| Ответ / ошибка | Действие |
|---|---|
| `408`, `429`, таймаут, сеть | `queued`, `run_at = now() + 30s` |
| `400`, `422` | `failed` |
| `401` | `failed`, в лог `webhook revoked` |
| прочие HTTP | `failed` |

`crm_label`:

| Состояние | Значение |
|---|---|
| есть `bitrix_id` | `в CRM ушло` |
| нет `bitrix_id`, последнее задание `failed` | `выгрузка упала` |
| иначе | `в CRM ещё нет` |

Reaper раз в `REAPER_EVERY` возвращает в очередь задачи, у которых `in_progress` и `locked_at` старше `REAPER_AFTER`. `run_at` ставится в `now()`, без backoff.

## Тесты и линт

```bash
make test-db   # создать и мигрировать leads_test
make test      # go test ./... с TEST_DATABASE_URL и REDIS_URL (DB 1)
make lint      # golangci-lint v1.55.2 (govet, errcheck, staticcheck)
```

Интеграционные тесты используют реальные PostgreSQL и Redis, поэтому перед `make test` нужны запущенные `postgres` и `redis`.

## Ограничения

- Нет аутентификации и проверки, что лид принадлежит вызывающему клиенту.
- У задач нет счётчика попыток: при постоянной временной ошибке повтор будет каждые 30 секунд.
- Rate limit — фиксированное окно: на границе окон возможен всплеск до `2 × BITRIX_LIMIT`.
- `/metrics` не защищён и должен быть доступен только изнутри сети.
