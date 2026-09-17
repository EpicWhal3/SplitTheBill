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
* Итоговый список долгов «кто кому сколько должен».

## Архитектура

```markdown
SplitTheBill/
├── backend/          # Go API (net/http), PostgreSQL или in-memory хранилище
│   ├── cmd/api/      # точка входа
│   ├── internal/     # domain, store, room (HTTP-хендлеры), calculation, migration
│   └── migrations/   # SQL-миграции (golang-migrate)
├── web/              # Next.js (App Router) + React + TypeScript
│   ├── app/          # страницы: лендинг и комната
│   └── lib/          # API-клиент, расчёты, работа с деньгами и сессией
└── .github/workflows # CI

```

* **Backend** — Go, стандартная библиотека `net/http`, деньги хранятся как
`int64` в минорных единицах (копейках).
* **Хранилище** — PostgreSQL (продакшен) или in-memory (локальная разработка
  без базы данных).
* **Frontend** — Next.js 16 (App Router), React 19, TypeScript.

## Требования

* Go 1.24+ (см. [`backend/go.mod`](backend/go.mod))
* Node.js 22+ и npm
* Docker (опционально, для PostgreSQL)

## Быстрый старт

### 1. Backend в in-memory режиме (без базы данных)

Если переменная `DATABASE_URL` не задана, сервер использует in-memory
хранилище — данные живут только до перезапуска процесса.

```bash
cd backend
go run ./cmd/api
```

Сервер поднимется на `http://localhost:8080` (или на порту из `PORT` ).

### 2. PostgreSQL через Docker Compose

```bash
cd backend
docker compose up -d
```

Поднимается PostgreSQL 17 на `localhost:5432` с базой, пользователем и паролем
`splitthebill` .

### 3. Переменные окружения backend

Скопируйте пример и при необходимости отредактируйте:

```bash
cd backend
cp .env.example .env
```

| Переменная | Назначение | По умолчанию |
| ----------------- | -------------------------------------------------------------------------- | ------------ |
| `DATABASE_URL` | Строка подключения к PostgreSQL. Если пусто — используется in-memory режим. | пусто |
| `PORT` | Порт HTTP-сервера. | `8080` |
| `ALLOWED_ORIGINS` | Список разрешённых CORS-origin через запятую. `*` не поддерживается. | `http://localhost:3000,http://127.0.0.1:3000,http://192.168.56.1:3000` |

Пример `DATABASE_URL` :

```cmd
postgres://splitthebill:splitthebill@localhost:5432/splitthebill?sslmode=disable
```

### 4. Миграции

Миграции применяются автоматически при старте backend, если задан
`DATABASE_URL` (см. [ `backend/internal/migration/migration.go` ](backend/internal/migration/migration.go)).
Отдельная команда не требуется — достаточно запустить сервер:

```bash
cd backend
DATABASE_URL="postgres://splitthebill:splitthebill@localhost:5432/splitthebill?sslmode=disable" go run ./cmd/api
```

### 5. Frontend

```bash
cd web
cp .env.local.example .env.local
npm install
npm run dev
```

Приложение откроется на `http://localhost:3000` .

| Переменная            | Назначение                          | По умолчанию            |
| --------------------- | ----------------------------------- | ----------------------- |
| `NEXT_PUBLIC_API_URL` | Базовый URL backend API.            | `http://localhost:8080` |

## Миграции и существующие базы

Схема развивалась вместе с приложением, поэтому база, созданная старой версией
миграции `000001` , может не содержать колонок `discount_mode` , `status` и
`finalized_at` в таблице `rooms` .

* **Если база создана старой схемой** — примените новую миграцию
  [ `000002_room_lifecycle_compatibility.up.sql` ](backend/migrations/000002_room_lifecycle_compatibility.up.sql).
  Она идемпотентна: добавляет недостающие колонки через
`ADD COLUMN IF NOT EXISTS` , проставляет значения по умолчанию для старых
  строк и безопасно создаёт CHECK-ограничения. Достаточно запустить backend с
`DATABASE_URL` — миграция применится автоматически.
* **Для локальной разработки без важных данных** — проще пересоздать базу:

  

```bash
  cd backend
  docker compose down -v
  docker compose up -d
  ```

  Флаг `-v` удаляет том с данными PostgreSQL.

## Тесты и проверки

```bash
# Backend: тесты
cd backend && go test ./...

# Frontend: тесты, линтер и production-сборка
cd web && npm run test && npm run lint && npm run build
```

Те же проверки выполняются в CI ([ `.github/workflows/ci.yml` ](.github/workflows/ci.yml)).
Для `npm ci` в репозитории должен присутствовать
[ `web/package-lock.json` ](web/package-lock.json) — он закоммичен.

## Жизненный цикл комнаты

| Статус      | Описание                                                              |
| ----------- | --------------------------------------------------------------------- |
| `draft` | Организатор готовит чек: добавляет позиции, участников и правила.     |
| `claiming` | Участники выбирают свои блюда и указывают веса.                       |
| `finalized` | Распределение завершено, суммы и долги зафиксированы.                 |

Переходы: `draft` → `claiming` ( `POST /rooms/{roomId}/open` ), 
`claiming` → `finalized` ( `POST /rooms/{roomId}/finalize` ), 
возврат в `claiming` ( `POST /rooms/{roomId}/reopen` ).

## API

| Метод и путь                                          | Назначение                          |
| ----------------------------------------------------- | ----------------------------------- |
| `POST /rooms` | Создать комнату                     |
| `GET /rooms/{roomId}` | Получить состояние комнаты          |
| `PATCH /rooms/{roomId}` | Обновить параметры комнаты          |
| `POST /rooms/{roomId}/join` | Присоединиться как участник         |
| `POST /rooms/{roomId}/participants` | Добавить участника (организатор)    |
| `PATCH /rooms/{roomId}/participants/{participantId}` | Изменить участника                  |
| `DELETE /rooms/{roomId}/participants/{participantId}` | Удалить участника                   |
| `POST /rooms/{roomId}/items` | Добавить позицию чека               |
| `PATCH /rooms/{roomId}/items/{itemId}` | Изменить позицию                    |
| `DELETE /rooms/{roomId}/items/{itemId}` | Удалить позицию                     |
| `POST /rooms/{roomId}/assignments` | Назначить позицию участнику         |
| `DELETE /rooms/{roomId}/assignments/{itemId}/{participantId}` | Снять назначение            |
| `PUT /rooms/{roomId}/selections/{itemId}` | Участник выбирает позицию           |
| `DELETE /rooms/{roomId}/selections/{itemId}` | Участник снимает выбор              |
| `POST /rooms/{roomId}/calculate` | Рассчитать итоги                    |
| `POST /rooms/{roomId}/open` | Перевести в `claiming` |
| `POST /rooms/{roomId}/finalize` | Зафиксировать результат             |
| `POST /rooms/{roomId}/reopen` | Вернуть в `claiming` |
| `GET /health` | Проверка живости сервиса            |

Авторизация передаётся заголовками `X-Admin-Token` (организатор) и
`X-Participant-Token` (участник).
