# Развертывание SplitTheBill

В этом документе описано, как собирать Docker-образы и запускать весь стек
проекта, какие порты и переменные окружения используются и как применяются
миграции.

## Предварительные требования

* Docker с Compose v2 (команда `docker compose`).

## Запуск всего стека

Одна команда поднимает PostgreSQL, backend и web:

```bash
docker compose up -d
```

При первом запуске или после изменений в коде добавьте флаг `--build`:

```bash
docker compose up --build -d
```

Остановка:

```bash
docker compose down
```

Остановка с удалением данных PostgreSQL:

```bash
docker compose down -v
```

## Порты

| Сервис     | Порт хоста | Порт контейнера | Назначение         |
|------------|------------|-----------------|--------------------|
| `web`      | 3000       | 3000            | Next.js-приложение |
| `backend`  | 8080       | 8080            | Go API             |
| `postgres` | 5432       | 5432            | PostgreSQL         |

Проверка живости backend:

```bash
curl http://localhost:8080/health
# => ok
```

## Сборка образов

### Все образы сразу

```bash
docker compose build
```

### Backend отдельно

```bash
docker build -t splitthebill-backend ./backend
```

Сборка многоступенчатая: в `builder`-стейдже на базе `golang:1.26-alpine`
компилируется статический бинарник из `./cmd/api`, а в финальном образе на
`alpine` остаются только бинарник и директория `migrations` (она нужна, так как
миграции запускаются по относительному пути `file://migrations`).

### Web отдельно

```bash
docker build \
  --build-arg NEXT_PUBLIC_API_URL=http://localhost:8080 \
  -t splitthebill-web ./web
```

Сборка идёт на `node:22-alpine`: `npm ci` → `npm run build`
(`output: "standalone"` в [`web/next.config.ts`](../web/next.config.ts)) →
финальный runner-образ, куда копируются `.next/standalone`, `.next/static` и
`public`.

## Переменные окружения

### Backend

| Переменная                       | По умолчанию                                                           |
|----------------------------------|------------------------------------------------------------------------|
| `DATABASE_URL`                   | пусто (in-memory режим)                                                |
| `PORT`                           | `8080`                                                                 |
| `ALLOWED_ORIGINS`                | `http://localhost:3000,http://127.0.0.1:3000,http://192.168.56.1:3000` |
| `MAX_REQUEST_BODY_BYTES`         | `1048576`                                                              |
| `RATE_LIMIT_REQUESTS_PER_SECOND` | `10`                                                                   |
| `RATE_LIMIT_BURST`               | `20`                                                                   |

В [`docker-compose.yml`](../docker-compose.yml) для backend заданы:

```text
DATABASE_URL=postgres://splitthebill:splitthebill@postgres:5432/splitthebill?sslmode=disable
PORT=8080
ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:3000
MAX_REQUEST_BODY_BYTES=1048576
RATE_LIMIT_REQUESTS_PER_SECOND=10
RATE_LIMIT_BURST=20
```

### Web

| Переменная            | По умолчанию            |
|-----------------------|-------------------------|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` |

## Миграции

Миграции применяются автоматически при старте контейнера backend. Логика
описана в [`backend/internal/migration/migration.go`](../backend/internal/migration/migration.go):
при наличии `DATABASE_URL` сервер выполняет `migrate.Up()` с источником
`file://migrations` (директория [`backend/migrations`](../backend/migrations)),
после чего подключается к PostgreSQL и начинает принимать запросы.

Backend в `docker-compose.yml` объявлен с `depends_on: postgres:
condition: service_healthy`, поэтому он не стартует, пока PostgreSQL не пройдёт
healthcheck `pg_isready`. Это исключает падение на миграциях при холодном
старте базы данных.

## Особенность Next.js: NEXT_PUBLIC_API_URL

`NEXT_PUBLIC_API_URL` встраивается в клиентский JavaScript-бандл на этапе
сборки образа. Поэтому значение задаётся через build arg в
[`web/Dockerfile`](../web/Dockerfile):

```dockerfile
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
```

и передаётся из [`docker-compose.yml`](../docker-compose.yml):

```yaml
build:
  context: ./web
  args:
    NEXT_PUBLIC_API_URL: http://localhost:8080
```

Если нужно указать другой адрес API, пересоберите образ с новым build arg.

## Проверка развертывания

```bash
# валидация конфигурации Compose
docker compose config

# сборка образов
docker compose build

# запуск стека
docker compose up -d

# статус сервисов
docker compose ps

# логи backend (в том числе применение миграций)
docker compose logs -f backend

# проверка API
curl http://localhost:8080/health
```
