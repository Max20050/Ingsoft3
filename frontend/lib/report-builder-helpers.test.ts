import { describe, expect, it } from "vitest";
import type { QueryFilter } from "@/lib/api";
import { columnBadge, errorMessage, filterSummary, operatorArity, sameColumns } from "./report-builder-helpers";

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

describe("filterSummary", () => {
  it("describes a none-arity filter without a value", () => {
    expect(filterSummary({ column: "region", operator: "is_null" })).toBe("region Is empty");
  });

  it("describes a pair-arity filter with both bounds", () => {
    expect(filterSummary({ column: "units", operator: "between", values: [1, 10] })).toBe(
      "units between 1 and 10",
    );
  });

  it("describes a many-arity filter joining every value", () => {
    expect(filterSummary({ column: "region", operator: "in", values: ["north", "south"] })).toBe(
      "region in north, south",
    );
  });

  it("describes a count-arity filter with its day count", () => {
    expect(filterSummary({ column: "closed_at", operator: "last_days", value: 7 })).toBe(
      "closed_at in the last 7 days",
    );
  });

  it("describes a one-arity filter with its single value", () => {
    expect(filterSummary({ column: "region", operator: "eq", value: "north" })).toBe("region Equals north");
  });

  it("falls back to a placeholder when a pair filter is missing a bound", () => {
    expect(filterSummary({ column: "units", operator: "between", values: [1] })).toBe("units between 1 and ?");
  });

  it("falls back to a placeholder when a pair filter has no values at all", () => {
    expect(filterSummary({ column: "units", operator: "between" })).toBe("units between ? and ?");
  });

  it("falls back to an empty list when a many-arity filter has no values", () => {
    expect(filterSummary({ column: "region", operator: "in" })).toBe("region in ");
  });

  it("falls back to a placeholder when a count-arity filter has no value", () => {
    expect(filterSummary({ column: "closed_at", operator: "last_days" })).toBe(
      "closed_at in the last ? days",
    );
  });

  it("falls back to a placeholder when a one-arity filter has no value", () => {
    expect(filterSummary({ column: "region", operator: "eq" })).toBe("region Equals ?");
  });

  it("falls back to the raw operator when it isn't in the OPERATORS table", () => {
    expect(
      filterSummary({ column: "region", operator: "matches_regex" as QueryFilter["operator"], value: ".*" }),
    ).toBe("region matches_regex .*");
  });
});
