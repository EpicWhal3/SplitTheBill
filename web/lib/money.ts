export function parseMoneyToMinorUnits(value: string): number {
  const normalized = value.replace(",", ".").trim();

  if (!normalized) {
    return 0;
  }

  const numberValue = Number(normalized);
  if (Number.isNaN(numberValue)) {
    return 0;
  }

  return Math.round(numberValue * 100);
}

export function formatMoney(amount: number, currency: string): string {
  return new Intl.NumberFormat("ru-RU", {
    style: "currency",
    currency,
  }).format(amount / 100);
}
