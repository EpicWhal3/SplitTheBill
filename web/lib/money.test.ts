import { describe, expect, it } from "vitest";

import {
  formatMoney,
  parseMoneyToMinorUnits,
  tryParseMoneyToMinorUnits,
} from "./money";

describe("tryParseMoneyToMinorUnits", () => {
  it("parses a dot decimal separator", () => {
    expect(tryParseMoneyToMinorUnits("12.34")).toBe(1234);
  });

  it("parses a comma decimal separator", () => {
    expect(tryParseMoneyToMinorUnits("12,34")).toBe(1234);
  });

  it("parses whole numbers", () => {
    expect(tryParseMoneyToMinorUnits("12")).toBe(1200);
  });

  it("rounds to the nearest minor unit", () => {
    expect(tryParseMoneyToMinorUnits("12.345")).toBe(1235);
    expect(tryParseMoneyToMinorUnits("12.344")).toBe(1234);
  });

  it("ignores surrounding whitespace", () => {
    expect(tryParseMoneyToMinorUnits("  12.34  ")).toBe(1234);
  });

  it("parses negative values", () => {
    expect(tryParseMoneyToMinorUnits("-5.50")).toBe(-550);
  });

  it("returns null for empty input", () => {
    expect(tryParseMoneyToMinorUnits("")).toBeNull();
    expect(tryParseMoneyToMinorUnits("   ")).toBeNull();
  });

  it("returns null for non-numeric input", () => {
    expect(tryParseMoneyToMinorUnits("abc")).toBeNull();
    expect(tryParseMoneyToMinorUnits("12.3.4")).toBeNull();
  });
});

describe("parseMoneyToMinorUnits", () => {
  it("returns the parsed value for valid input", () => {
    expect(parseMoneyToMinorUnits("12.34")).toBe(1234);
  });

  it("falls back to zero for invalid input", () => {
    expect(parseMoneyToMinorUnits("")).toBe(0);
    expect(parseMoneyToMinorUnits("abc")).toBe(0);
  });
});

describe("formatMoney", () => {
  it("formats minor units as a currency amount", () => {
    const formatted = formatMoney(1234, "EUR");

    expect(formatted).toContain("12,34");
  });

  it("formats zero", () => {
    const formatted = formatMoney(0, "EUR");

    expect(formatted).toContain("0,00");
  });

  it("formats large amounts with grouping", () => {
    const formatted = formatMoney(123456789, "EUR");

    expect(formatted).toContain("1");
    expect(formatted).toContain("234");
    expect(formatted).toContain("567,89");
  });

  it("respects the provided currency", () => {
    const formatted = formatMoney(1000, "USD");

    expect(formatted).toMatch(/[$]|USD/);
  });
});
