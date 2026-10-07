import { describe, expect, it } from "vitest";
import { columnBadge, errorMessage, operatorArity, sameColumns } from "./report-builder-helpers";

describe("errorMessage", () => {
  it("returns the message of a real Error", () => {
    const result = errorMessage(new Error("the data source could not answer this report's query"), "fallback");
    expect(result).toBe("the data source could not answer this report's query");
  });

  it.each([
    ["a plain string throw", "network down"],
    ["a thrown object with no message", { code: 500 }],
    ["null", null],
    ["undefined", undefined],
  ])("falls back for %s", (_case, thrown) => {
    expect(errorMessage(thrown, "Failed to run the query")).toBe("Failed to run the query");
  });
});

describe("operatorArity", () => {
  it.each([
    ["eq", "one"],
    ["between", "pair"],
    ["is_null", "none"],
    ["in", "many"],
  ])("maps %s to arity %s", (operator, want) => {
    expect(operatorArity(operator as Parameters<typeof operatorArity>[0])).toBe(want);
  });

  it("falls back to one for an operator the table doesn't list", () => {
    expect(operatorArity("matches_regex" as Parameters<typeof operatorArity>[0])).toBe("one");
  });
});

describe("columnBadge", () => {
  it.each([
    ["int4", "123"],
    ["numeric", "123"],
    ["timestamp", "\u{1F4C5}"],
    ["date", "\u{1F4C5}"],
    ["boolean", "✓"],
    ["text", "A"],
    [undefined, "A"],
  ])("badges %s as %s", (type, want) => {
    expect(columnBadge(type)).toBe(want);
  });
});

describe("sameColumns", () => {
  it("is true for the same columns in the same order", () => {
    expect(sameColumns(["region", "revenue"], ["region", "revenue"])).toBe(true);
  });

  it("is false when the order differs", () => {
    expect(sameColumns(["region", "revenue"], ["revenue", "region"])).toBe(false);
  });

  it("is false when the lengths differ", () => {
    expect(sameColumns(["region"], ["region", "revenue"])).toBe(false);
  });
});
