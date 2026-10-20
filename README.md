# SplitTheBill

Приложение для разделения счёта в ресторане между участниками. Организатор
создаёт комнату, добавляет позиции чека и участников, а затем каждый участник
отмечает свои блюда. Сервис рассчитывает итоговые суммы с учётом сервисного
сбора, чаевых и скидки, а также строит список взаимных долгов.

## Возможности

* Расчёт долей с сервисным сбором, чаевыми и скидкой.
* Два режима распределения скидки: `proportional` и `equal`.
* Жизненный цикл комнаты: `draft` → `claiming` → `finalized`.
* Режимы организатора (админ-токен) и участника (токен участника).
* Веса для позиций, которые делятся между несколькими участниками.
* Бесплатные позиции (`unit_price = 0`) без искажения распределения сборов,
  чаевых и скидки.
* Итоговый список долгов «кто кому сколько должен».
* SSE-обновления комнаты с резервным polling на клиенте.
* Rate limiting, ограничение размера тела запроса и заголовки безопасности.

## Архитектура

```text
SplitTheBill/
├── backend/                 # Go API (net/http), PostgreSQL или in-memory хранилище
│   ├── cmd/api/             # точка входа и HTTP middleware
│   ├── internal/            # domain, store, room (HTTP-хендлеры), calculation, migration
│   └── migrations/          # SQL-миграции (golang-migrate)
├── web/                     # Next.js (App Router) + React + TypeScript
│   ├── app/                 # страницы: лендинг и комната
│   └── lib/                 # API-клиент, расчёты, работа с деньгами и сессией
├── docs/                    # документация (deployment, API, current-state, Kubernetes)
├── infra/                   # Kubernetes-манифесты (Kustomize) и Ansible-плейбуки
├── docker-compose.yml       # весь стек: postgres + backend + web
└── .github/workflows/       # CI
```

* **Backend** — Go, стандартная библиотека `net/http`, деньги хранятся как
  `int64` в минорных единицах (копейках/центах).
* **Хранилище** — PostgreSQL в Docker Compose или in-memory (локальная
  разработка без базы данных).
* **Frontend** — Next.js (App Router), React 19, TypeScript.

## Требования

* Go 1.26+ (см. [`backend/go.mod`](backend/go.mod))
* Node.js 22+ и npm
* Docker с Compose v2

## Быстрый старт (Docker Compose)

Одна команда поднимает PostgreSQL, backend и web:

```bash
docker compose up --build -d
```

После старта:

* веб-интерфейс: <http://localhost:3000>
* API: <http://localhost:8080>
* PostgreSQL: `localhost:5432` (база/пользователь/пароль — `splitthebill`)

Проверка живости backend:

```bash
curl http://localhost:8080/health
# => ok
```

Остановка:

```bash
docker compose down
```

Удалить также данные PostgreSQL:

```bash
docker compose down -v
```

Подробнее о сборке и развёртывании — в [`docs/deployment.md`](docs/deployment.md).

## Локальный запуск без Docker

### Backend в in-memory режиме (без базы данных)

Если переменная `DATABASE_URL` не задана, сервер использует in-memory
хранилище — данные живут только до перезапуска процесса.

```bash
cd backend
go run ./cmd/api
```

Сервер поднимется на `http://localhost:8080` (или на порту из `PORT`).

### Backend с PostgreSQL

```bash
cd backend
DATABASE_URL="postgres://splitthebill:splitthebill@localhost:5432/splitthebill?sslmode=disable" go run ./cmd/api
```

Миграции применяются автоматически при старте.

### Frontend в режиме разработки

```bash
cd web
cp .env.local.example .env.local
npm install
npm run dev
```

Приложение откроется на `http://localhost:3000`.

## Переменные окружения

### Backend

| Переменная                       | Назначение                                                                  | По умолчанию                                                           |
|----------------------------------|-----------------------------------------------------------------------------|------------------------------------------------------------------------|
| `DATABASE_URL`                   | Строка подключения к PostgreSQL. Если пусто — используется in-memory режим. | пусто                                                                  |
| `PORT`                           | Порт HTTP-сервера.                                                          | `8080`                                                                 |
| `ALLOWED_ORIGINS`                | Список разрешённых CORS-origin через запятую. `*` не поддерживается.        | `http://localhost:3000,http://127.0.0.1:3000,http://192.168.56.1:3000` |
| `MAX_REQUEST_BODY_BYTES`         | Максимальный размер тела запроса в байтах (превышение — `413`).             | `1048576`                                                              |
| `RATE_LIMIT_REQUESTS_PER_SECOND` | Скорость пополнения токенов rate limiter (запросов в секунду на IP).        | `10`                                                                   |
| `RATE_LIMIT_BURST`               | Размер «ведра» токенов rate limiter (пиковая ёмкость на IP).                | `20`                                                                   |

Пример `DATABASE_URL`:

```text
postgres://splitthebill:splitthebill@localhost:5432/splitthebill?sslmode=disable
```

### Web

| Переменная            | Назначение                                                                | По умолчанию            |
|-----------------------|---------------------------------------------------------------------------|-------------------------|
| `NEXT_PUBLIC_API_URL` | Базовый URL backend API. Встраивается в клиентский бандл на этапе сборки. | `http://localhost:8080` |

## Миграции

Миграции применяются автоматически при старте backend, если задан
`DATABASE_URL` (см. [`backend/internal/migration/migration.go`](backend/internal/migration/migration.go)).
Схема собирается из последовательных версий в
[`backend/migrations/`](backend/migrations): базовая схема
[`000001_init.up.sql`](backend/migrations/000001_init.up.sql) создаёт таблицы
`rooms`, `participants`, `receipt_items`, `item_assignments` и индексы,
[`000002_participant_claims.up.sql`](backend/migrations/000002_participant_claims.up.sql)
добавляет флаг `claimed`,
[`000003_organizer_credentials.up.sql`](backend/migrations/000003_organizer_credentials.up.sql)
— имя и хэш пароля организатора,
[`000004_allow_zero_unit_price.up.sql`](backend/migrations/000004_allow_zero_unit_price.up.sql)
разрешает нулевые `unit_price` и `total`. Применённые миграции не
редактируются: любое изменение схемы — новая версия.

## Тесты и проверки

```bash
# Backend: формат, статический анализ и тесты
cd backend
test -z "$(gofmt -l .)"
go vet ./...
go test -race -coverprofile=coverage.out ./...

# Frontend: установка зависимостей, линтер, тесты и production-сборка
cd web
npm ci
npm run lint
npm run test
npm run build
```

Те же проверки выполняются в CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)),
включая валидацию `docker compose config` и сборку образов `docker compose build`.

## Жизненный цикл комнаты

| Статус      | Описание                                                          |
|-------------|-------------------------------------------------------------------|
| `draft`     | Организатор готовит чек: добавляет позиции, участников и правила. |
| `claiming`  | Участники выбирают свои блюда и указывают веса.                   |
| `finalized` | Распределение завершено, суммы и долги зафиксированы.             |

Переходы:

* `draft` → `claiming`: `POST /rooms/{roomId}/open`
* `claiming` → `finalized`: `POST /rooms/{roomId}/finalize`
* `finalized` → `claiming`: `POST /rooms/{roomId}/reopen`

В `draft` и `claiming` разрешено изменение данных комнаты; в `finalized`
комната доступна только для чтения (кроме удаления и ротации токена).

## API

Авторизация передаётся заголовками `X-Admin-Token` (организатор) и
`X-Participant-Token` (участник). Денежные суммы передаются в минорных
единицах `int64` (копейки/центы).

Обзор всех эндпоинтов:

| Метод и путь                                                  | Назначение                                          |
|---------------------------------------------------------------|-----------------------------------------------------|
| `GET /health`                                                 | Проверка живости сервиса                            |
| `POST /rooms`                                                 | Создать комнату                                     |
| `GET /rooms/{roomId}`                                         | Получить состояние комнаты                          |
| `PATCH /rooms/{roomId}`                                       | Обновить параметры комнаты                          |
| `DELETE /rooms/{roomId}`                                      | Удалить комнату                                     |
| `POST /rooms/{roomId}/admin-token/rotate`                     | Перевыпустить админ-токен                           |
| `POST /rooms/{roomId}/open`                                   | Перевести комнату в `claiming`                      |
| `POST /rooms/{roomId}/finalize`                               | Зафиксировать результат                             |
| `POST /rooms/{roomId}/reopen`                                 | Вернуть комнату в `claiming`                        |
| `GET /rooms/{roomId}/events`                                  | SSE-поток событий `room.updated`                    |
| `POST /rooms/{roomId}/participants`                           | Добавить участника (организатор)                    |
| `POST /rooms/{roomId}/join`                                   | Присоединиться как участник                         |
| `PATCH /rooms/{roomId}/participants/{participantId}`          | Изменить участника                                  |
| `DELETE /rooms/{roomId}/participants/{participantId}`         | Удалить участника                                   |
| `POST /rooms/{roomId}/items`                                  | Добавить позицию чека                               |
| `PATCH /rooms/{roomId}/items/{itemId}`                        | Изменить позицию                                    |
| `DELETE /rooms/{roomId}/items/{itemId}`                       | Удалить позицию                                     |
| `POST /rooms/{roomId}/assignments`                            | Назначить позицию участнику                         |
| `DELETE /rooms/{roomId}/assignments/{itemId}/{participantId}` | Снять назначение                                    |
| `PUT /rooms/{roomId}/selections/{itemId}`                     | Участник выбирает позицию                           |
| `DELETE /rooms/{roomId}/selections/{itemId}`                  | Участник снимает выбор (идемпотентно, всегда `204`) |
| `POST /rooms/{roomId}/calculate`                              | Рассчитать итоги                                    |

Подробное описание схем запросов/ответов и кодов ошибок:

* [`docs/openapi.yaml`](docs/openapi.yaml) — машиночитаемая OpenAPI-спецификация;
* [`docs/api.md`](docs/api.md) — краткий справочник по эндпоинтам.

## Kubernetes и Ansible

Инфраструктура для Kubernetes и Ansible находится в [`infra/`](infra):

* [`infra/kubernetes`](infra/kubernetes) — Kustomize base и overlays `dev`/`prod`
  для PostgreSQL, backend, web и ingress;
* [`infra/ansible`](infra/ansible) — плейбуки деплоя и dev-установки K3s.

Подробная инструкция: [`docs/kubernetes-ansible.md`](docs/kubernetes-ansible.md).

`docker-compose` остаётся основным способом локального запуска; Kubernetes и
Ansible не меняют локальный стек и `render.yaml`.

## Документация

* [`docs/deployment.md`](docs/deployment.md) — развёртывание и сборка Docker-образов;
* [`docs/kubernetes-ansible.md`](docs/kubernetes-ansible.md) — Kubernetes, Kustomize и Ansible;
* [`docs/api.md`](docs/api.md) и [`docs/openapi.yaml`](docs/openapi.yaml) — документация API;
* [`docs/current-state.md`](docs/current-state.md) — текущее состояние проекта.
