# SplitTheBill — web

Frontend-приложение на Next.js (App Router) + React + TypeScript.

## Разработка

```bash
npm install
npm run dev
```

Приложение откроется на `http://localhost:3000`.

## Переменные окружения

Скопируйте пример и при необходимости отредактируйте:

```bash
cp .env.local.example .env.local
```

| Переменная            | Назначение               | По умолчанию            |
|-----------------------|--------------------------|-------------------------|
| `NEXT_PUBLIC_API_URL` | Базовый URL backend API. | `http://localhost:8080` |

## Скрипты

| Команда                | Назначение                                 |
|------------------------|--------------------------------------------|
| `npm run dev`          | Dev-сервер                                 |
| `npm run build`        | Production-сборка (`output: "standalone"`) |
| `npm run start`        | Запуск production-сборки                   |
| `npm run lint`         | ESLint                                     |
| `npm run test`         | Vitest (unit-тесты `lib`)                  |
| `npm run format`       | Prettier (запись)                          |
| `npm run format:check` | Prettier (проверка)                        |

## Сборка Docker-образа

```bash
docker build \
  --build-arg NEXT_PUBLIC_API_URL=http://localhost:8080 \
  -t splitthebill-web .
```

Подробнее о полном развёртывании — в корневом
[`README.md`](../README.md) и [`docs/deployment.md`](../docs/deployment.md).
