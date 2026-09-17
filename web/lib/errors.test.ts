import { describe, expect, it } from "vitest";

import { translateError } from "./errors";

describe("translateError", () => {
  it("translates the required server errors", () => {
    const cases: Array<[string, string]> = [
      ["room is finalized", "Комната уже завершена"],
      ["room is not open for selections", "Организатор ещё не открыл"],
      [
        "room must be open for selections before finalization",
        "откройте распределение",
      ],
      [
        "add at least one item before opening selections",
        "хотя бы одну позицию",
      ],
      ["payer is required before finalization", "оплатил чек"],
      ["calculated total does not match expected total", "не совпадает"],
      [
        "all items must have at least one participant",
        "хотя бы одному участнику",
      ],
      ["weight must be between 1 and 1000", "от 1 до 1000"],
      ["organizer access required", "права организатора"],
      ["participant session is invalid", "Сессия участника недействительна"],
    ];

    for (const [input, expectedPart] of cases) {
      const translated = translateError(input);

      expect(translated).not.toBe(input);
      expect(translated).toContain(expectedPart);
    }
  });

  it("translates messages with a dynamic suffix", () => {
    const translated = translateError("item has no assignments: Pizza");

    expect(translated).not.toBe("item has no assignments: Pizza");
    expect(translated).toContain("нет участников");
  });

  it("translates discount errors with a dynamic suffix", () => {
    const translated = translateError(
      "discount exceeds bill total: maximum is 900",
    );

    expect(translated).toContain("Скидка больше суммы чека");
  });

  it("returns unknown messages unchanged", () => {
    expect(translateError("some brand new error")).toBe("some brand new error");
  });

  it("ignores surrounding whitespace", () => {
    expect(translateError("  room is finalized  ")).toContain(
      "Комната уже завершена",
    );
  });
});
