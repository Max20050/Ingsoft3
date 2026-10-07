// Pure logic pulled out of app/ui/report-builder.tsx so it can be unit
// tested without rendering the component: no DOM, no fetch, no React state.

import type { Operator, OperatorArity, QueryFilter } from "@/lib/api";
import { OPERATORS } from "@/lib/api";

// errorMessage extracts a user-facing message from whatever a failed request
// rejected with. Not every rejection is an Error — a fetch that gets
// cancelled, or a throw of a plain string, both land here too.
export function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

// sameColumns tells whether a query preview's columns are the same, in the
// same order, as the ones a report's current field list would produce —
// used to decide whether a stale preview can still be trusted.
export function sameColumns(left: string[], right: string[]): boolean {
  return left.length === right.length && left.every((column, index) => column === right[index]);
}

// operatorArity looks up how many values a filter operator takes, from the
// same OPERATORS table the filter editor renders from. Falls back to "one"
// for an operator value the table doesn't (yet) list.
export function operatorArity(operator: Operator): OperatorArity {
  return OPERATORS.find((candidate) => candidate.value === operator)?.arity ?? "one";
}

// columnBadge shortens a schema column's declared type down to a one-glyph
// hint the field picker shows next to each checkbox.
export function columnBadge(type: string | undefined): string {
  const t = (type ?? "").toLowerCase();
  if (/int|numeric|decimal|float|double|serial/.test(t)) return "123";
  if (/date|time/.test(t)) return "\u{1F4C5}";
  if (/bool/.test(t)) return "✓";
  return "A";
}

// filterSummary turns a filter row into the plain-language sentence a
// screen reader announces for it, since the row itself is just a handful of
// unlabelled selects and inputs sitting side by side.
export function filterSummary(filter: QueryFilter): string {
  const operatorLabel = OPERATORS.find((candidate) => candidate.value === filter.operator)?.label ?? filter.operator;

  switch (operatorArity(filter.operator)) {
    case "none":
      return `${filter.column} ${operatorLabel}`;
    case "pair": {
      const [from, to] = filter.values ?? [];
      return `${filter.column} between ${from ?? "?"} and ${to ?? "?"}`;
    }
    case "many":
      return `${filter.column} in ${(filter.values ?? []).join(", ")}`;
    case "count":
      return `${filter.column} in the last ${filter.value ?? "?"} days`;
    default:
      return `${filter.column} ${operatorLabel} ${filter.value ?? "?"}`;
  }
}
