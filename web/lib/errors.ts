const translations: Record<string, string> = {
  "room is finalized":
    "Комната уже завершена и заблокирована. Верните её к редактированию, чтобы вносить изменения.",
  "room is not open for selections":
    "Организатор ещё не открыл выбор блюд. Дождитесь начала распределения.",
  "room must be open for selections before finalization":
    "Сначала откройте распределение, чтобы участники выбрали блюда.",
  "add at least one item before opening selections":
    "Добавьте хотя бы одну позицию в чек, прежде чем открывать распределение.",
  "payer is required before finalization":
    "Перед завершением выберите человека, который оплатил чек.",
  "calculated total does not match expected total":
    "Рассчитанная сумма не совпадает с итогом на чеке. Проверьте позиции и правила.",
  "all items must have at least one participant":
    "Каждая позиция должна быть назначена хотя бы одному участнику.",
  "weight must be between 1 and 1000":
    "Вес должен быть целым числом от 1 до 1000.",
  "organizer access required":
    "Нужны права организатора. Откройте комнату по ссылке с ключом организатора.",
  "participant session is invalid":
    "Сессия участника недействительна. Войдите в комнату заново.",
  "participant name is already taken":
    "Такое имя уже занято в этой комнате. Выберите другое.",
  "item does not exist in this room":
    "Эта позиция не найдена в комнате. Обновите страницу.",
  "participant does not exist in this room":
    "Этот участник не найден в комнате. Обновите страницу.",
  "room not found": "Комната не найдена. Проверьте ссылку.",
  "selection not found": "Выбор уже снят.",
  "assignment not found": "Назначение уже удалено.",
  "method not allowed": "Это действие здесь недоступно.",
  "route not found": "Запрошенный раздел не найден.",
  "invalid json": "Не удалось прочитать данные запроса.",
  "title is required": "Укажите название комнаты.",
  "title is too long": "Название комнаты слишком длинное.",
  "currency must be a three-letter code":
    "Код валюты должен состоять из трёх латинских букв.",
  "currency must contain only latin letters":
    "Код валюты должен содержать только латинские буквы.",
  "service_fee, tip_amount, discount and expected_total must be non-negative":
    "Суммы не могут быть отрицательными.",
  "discount_mode must be proportional or equal":
    "Режим скидки должен быть «пропорционально» или «поровну».",
  "name is required": "Укажите имя.",
  "participant name is too long": "Имя участника слишком длинное.",
  "item name is too long": "Название позиции слишком длинное.",
  "quantity must be positive": "Количество должно быть больше нуля.",
  "unit_price must be positive": "Цена должна быть больше нуля.",
  "payer must be a participant of this room":
    "Плательщиком может быть только участник этой комнаты.",
  "item_id and participant_id are required": "Выберите позицию и участника.",
  "no participants": "В комнате пока нет участников.",
  "no receipt items": "В чеке пока нет позиций.",
  "subtotal must be positive": "Сумма позиций должна быть больше нуля.",
  "discount exceeds bill total": "Скидка больше суммы чека. Уменьшите скидку.",
  "payer is required": "Выберите плательщика.",
  "payer is not a participant": "Плательщик не является участником комнаты.",
  "unsupported discount mode": "Неподдерживаемый режим скидки.",
  "assignment weight must be positive": "Вес должен быть больше нуля.",
  "item has no assignments":
    "У этой позиции нет участников. Назначьте её хотя бы одному человеку.",
  "internal server error": "На сервере произошла ошибка. Попробуйте ещё раз.",
  "failed to create room": "Не удалось создать комнату. Попробуйте ещё раз.",
  "failed to save assignment":
    "Не удалось сохранить назначение. Попробуйте ещё раз.",
};

export function translateError(message: string): string {
  const trimmed = message.trim();

  if (translations[trimmed]) {
    return translations[trimmed];
  }

  for (const [key, value] of Object.entries(translations)) {
    if (trimmed.startsWith(`${key}:`)) {
      return value;
    }
  }

  return message;
}
