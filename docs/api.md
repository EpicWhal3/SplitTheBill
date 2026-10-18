# Документация API SplitTheBill

Машиночитаемая спецификация находится в [`docs/openapi.yaml`](openapi.yaml).
Ниже — краткий справочник по всем эндпоинтам.

## Соглашения

* Base URL: `http://localhost:8080`.
* Денежные суммы — `int64` в **минорных единицах** (копейки/центы).
* Авторизация организатора — заголовок `X-Admin-Token`.
* Авторизация участника — заголовок `X-Participant-Token`.
* Ошибка возвращается в виде `{"error": "сообщение"}`.

## Глобальные коды ошибок

| Код   | Когда возникает                                                            |
|-------|----------------------------------------------------------------------------|
| `405` | Метод не поддерживается для маршрута.                                      |
| `413` | Тело запроса превышает `MAX_REQUEST_BODY_BYTES`.                           |
| `429` | Превышен rate limit (`RATE_LIMIT_REQUESTS_PER_SECOND`/`RATE_LIMIT_BURST`). |
| `500` | Внутренняя ошибка сервера.                                                 |

`400`, `401`, `404`, `409` описаны у конкретных эндпоинтов ниже.

## Система

### GET /health

Проверка живости. Без авторизации.

* `200` — тело `ok`.

## Комнаты и администрирование

### POST /rooms

Создать комнату (статус `draft`). Без авторизации.

* Тело: `title` (обязательно), `currency`, `service_fee`, `tip_amount`,
  `discount`, `discount_mode`, `expected_total` (необязательны).
* `201` — `{room, admin_token}`.
* `400` — неверный JSON или валидация.

### GET /rooms/{roomId}

Состояние комнаты. Без авторизации. Любой статус.

* `200` — `{room, participants, items, assignments, subtotal, unassigned_item_ids}`.
* `404` — комната не найдена.

### PATCH /rooms/{roomId}

Обновить параметры комнаты. `X-Admin-Token`. Статусы `draft`/`claiming`.

* Тело: любые поля комнаты (`title`, `currency`, `service_fee`, `tip_amount`,
  `discount`, `discount_mode`, `expected_total`, `payer_participant_id`).
* `200` — обновлённая комната.
* `400`, `401`, `404`, `409` (`finalized`).

### DELETE /rooms/{roomId}

Удалить комнату со всеми данными. `X-Admin-Token`. Любой статус.

* `204` — удалена.
* `401`, `404`.

### POST /rooms/{roomId}/admin-token/rotate

Перевыпустить админ-токен. `X-Admin-Token`. Любой статус.

* `200` — `{room, admin_token}`.
* `401`, `404`.

### POST /rooms/{roomId}/open

Открыть выбор: `draft`/`claiming` → `claiming`. `X-Admin-Token`.

* `200` — обновлённая комната.
* `401`, `404`, `409` (комната `finalized` или нет позиций).

### POST /rooms/{roomId}/finalize

Зафиксировать результат: `claiming` → `finalized`. `X-Admin-Token`.

* `200` — полный расчёт (см. [`CalculateResponse`](openapi.yaml)).
* `401`, `404`, `409` (не `claiming`, нет плательщика, есть неназначенные
  позиции или итог не совпадает с `expected_total`).

### POST /rooms/{roomId}/reopen

Вернуть в `claiming`: только из `finalized`. `X-Admin-Token`.

* `200` — обновлённая комната.
* `401`, `404`, `409` (комната не `finalized`).

### GET /rooms/{roomId}/events

SSE-поток. Без авторизации. Любой статус.

* `200` — `text/event-stream`; событие `room.updated`, данные
  `{"room_id": "..."}`.
* `404` — комната не найдена.

## Участники

### POST /rooms/{roomId}/participants

Добавить участника. `X-Admin-Token`. Статусы `draft`/`claiming`.

* Тело: `{name}`.
* `201` — `{participant, participant_token}`. Персональная ссылка участника:
  `/rooms/{roomId}?token={participant_token}`. Токен возвращается только в
  ответе на этот запрос — сохраните ссылку или выпустите новую через
  `POST /rooms/{roomId}/participants/{participantId}/invite`.
* `400`, `401`, `404`, `409`.

### POST /rooms/{roomId}/join

Обменять персональный инвайт-токен на сессию участника. Без авторизации.
Статусы `draft`/`claiming`. Свободного входа по имени больше нет.

* Тело: `{token}`.
* `201` — `{participant, participant_token}`; участник помечается как `claimed`.
* `400` — токен не передан (`invite token is required`).
* `401` — токен неизвестен (`invalid participant invite token`).
* `404` — комната не найдена.
* `409` — комната `finalized`.

### POST /rooms/{roomId}/participants/{participantId}/invite

Выпустить новую персональную ссылку участника; предыдущая перестаёт работать.
`X-Admin-Token`. Статусы `draft`/`claiming`.

* `200` — `{participant, participant_token}`.
* `401`, `404`, `409`.

### PATCH /rooms/{roomId}/participants/{participantId}

Переименовать участника. `X-Admin-Token`. Статусы `draft`/`claiming`.

* Тело: `{name}`.
* `200` — участник.
* `400`, `401`, `404`, `409`.

### DELETE /rooms/{roomId}/participants/{participantId}

Удалить участника и его назначения. `X-Admin-Token`. Статусы `draft`/`claiming`.

* `204`.
* `401`, `404`, `409`.

## Позиции

### POST /rooms/{roomId}/items

Добавить позицию. `X-Admin-Token`. Статусы `draft`/`claiming`.

* Тело: `{name, unit_price}`; `quantity` необязателен (по умолчанию `1`).
* `201` — позиция.
* `400`, `401`, `404`, `409`.

### PATCH /rooms/{roomId}/items/{itemId}

Изменить позицию. `X-Admin-Token`. Статусы `draft`/`claiming`.

* Тело: `name`, `quantity`, `unit_price` (необязательны).
* `200` — позиция.
* `400`, `401`, `404`, `409`.

### DELETE /rooms/{roomId}/items/{itemId}

Удалить позицию и связанные назначения. `X-Admin-Token`. Статусы
`draft`/`claiming`.

* `204`.
* `401`, `404`, `409`.

## Назначения (организатор)

### POST /rooms/{roomId}/assignments

Назначить позицию участнику с весом. `X-Admin-Token`. Статусы `draft`/`claiming`.

* Тело: `{item_id, participant_id, weight}` (`weight` 1..1000).
* `201` — назначение.
* `400`, `401`, `404`, `409`.

### DELETE /rooms/{roomId}/assignments/{itemId}/{participantId}

Снять назначение. `X-Admin-Token`. Статусы `draft`/`claiming`.

* `204`.
* `401`, `404`, `409`.

## Выбор позиций (участник)

### PUT /rooms/{roomId}/selections/{itemId}

Участник выбирает позицию. `X-Participant-Token`. Только `claiming`.

* Тело: `{weight}` (необязательно, по умолчанию `1`).
* `200` — назначение.
* `400`, `401`, `404`, `409` (не `claiming`).

### DELETE /rooms/{roomId}/selections/{itemId}

Участник снимает выбор. Идемпотентно: всегда `204`. `X-Participant-Token`.
Только `claiming`.

* `204`.
* `401`, `404`, `409` (не `claiming`).

## Расчёт

### POST /rooms/{roomId}/calculate

Рассчитать итоги без смены статуса. Без авторизации.

* `200` — `{room, results, debts, subtotal, calculated_total, difference, matches_expected_total}`.
* `400` — есть неназначенные позиции, нет участников/позиций или скидка
  превышает итог.
* `404` — комната не найдена.
