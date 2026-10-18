# Текущее состояние проекта SplitTheBill

Документ фиксирует, что уже реализовано, а что ещё предстоит сделать.

## Готово

### Backend (Go, `net/http`)

* CRUD комнаты: `POST /rooms`, `GET/PATCH/DELETE /rooms/{roomId}`.
* Удаление комнаты целиком: `DELETE /rooms/{roomId}`.
* Ротация админ-токена: `POST /rooms/{roomId}/admin-token/rotate`.
* Участники: добавление, присоединение по ссылке, изменение, удаление,
  авторизация по токену участника.
* Позиции чека: добавление, изменение, удаление.
* Назначения и выборы: `assignments`, `selections` с поддержкой весов.
* Расчёт итогов: `POST /rooms/{roomId}/calculate`.
* Жизненный цикл комнаты: `draft` → `claiming` → `finalized`
  (`open`, `finalize`, `reopen`).
* Два режима распределения скидки: `proportional` и `equal`.
* Деньги — `int64` в минорных единицах.
* Два хранилища: PostgreSQL и in-memory (при пустом `DATABASE_URL`).
* Миграции через `golang-migrate`, применяются автоматически при старте.
  Вся схема описана в [`backend/migrations/000001_init.up.sql`](../backend/migrations/000001_init.up.sql).
* Настраиваемый порт `PORT` и CORS через `ALLOWED_ORIGINS`.
* Rate limiting (token bucket) через `RATE_LIMIT_REQUESTS_PER_SECOND` и
  `RATE_LIMIT_BURST` — превышение возвращает `429`.
* Ограничение размера тела запроса через `MAX_REQUEST_BODY_BYTES` —
  превышение возвращает `413`.
* Заголовки безопасности: `X-Content-Type-Options`, `Referrer-Policy`,
  `X-Frame-Options`.
* Эндпоинт `GET /health` (возвращает `200 ok`).
* SSE-обновления комнат через `GET /rooms/{roomId}/events` с резервным polling
  на клиенте.

### Frontend (Next.js, React 19, TypeScript)

* Лендинг с созданием комнаты.
* Страница комнаты с режимами организатора и участника.
* Локальный предпросмотр расчёта, форматирование денег, работа с сессией.
* QR-код ссылки-приглашения (генерируется на клиенте через `qrcode`).
* Человекочитаемые русские сообщения об ошибках (`web/lib/errors.ts`).
* Организатор видит список нераспределённых позиций; финализация
  заблокирована с понятной причиной, пока есть такие позиции.
* Подтверждения перед завершением распределения и возвратом к редактированию.
* Персональный финальный экран участника: своя сумма, кому и сколько должен,
  либо сообщение об отсутствии переводов.
* Кнопка «Скопировать мой итог» на финальном экране участника.
* Удаление комнаты и ротация админ-токена из интерфейса организатора.
* Тесты на Vitest для `web/lib/local-calculation.ts`, `web/lib/money.ts` и
  `web/lib/errors.ts` (`npm run test`).

### Инфраструктура

* [`backend/Dockerfile`](../backend/Dockerfile) — многоступенчатая сборка
  статического бинарника, финальный образ на `alpine` с `ca-certificates` и
  `wget`, `HEALTHCHECK` и `ENV PORT=8080`.
* [`backend/.dockerignore`](../backend/.dockerignore).
* [`web/Dockerfile`](../web/Dockerfile) — многоступенчатая сборка на
  `node:22-alpine`, `output: "standalone"`, передача `NEXT_PUBLIC_API_URL`
  через build arg.
* [`web/.dockerignore`](../web/.dockerignore).
* Корневой [`docker-compose.yml`](../docker-compose.yml) со всеми сервисами:
  `postgres`, `backend`, `web`.
* [`docs/deployment.md`](deployment.md) — инструкции по сборке и развёртыванию.
* [`docs/openapi.yaml`](openapi.yaml) и [`docs/api.md`](api.md) — полная
  документация API.
* Примеры переменных окружения: `backend/.env.example`, `web/.env.local.example`.
* CI (`.github/workflows/ci.yml`): `gofmt`, `go vet`, `go test -race`,
  `npm ci`, `npm run lint`, `npm run test`, `npm run build`, а также
  валидация `docker compose config` и сборка `docker compose build`.

### Тесты

* Backend: unit-тесты расчёта (`calculator_test.go`), хранилища (`memory_test.go`)
  и HTTP-хендлеров (`handler_test.go`) — покрывают жизненный цикл комнаты,
  режимы скидки, веса, финализацию, повторное открытие, идемпотентное снятие
  выбора участником, удаление комнаты и ротацию админ-токена.
* Frontend: Vitest-тесты пропорциональной и равной скидки, распределения
  остатков, весов и работы с деньгами.

## Не готово

* **Нет e2e-тестов.** Есть unit-тесты backend (расчёт, хранилище,
  HTTP-хендлеры) и frontend (Vitest), но нет сквозных браузерных тестов
  пользовательских сценариев.
