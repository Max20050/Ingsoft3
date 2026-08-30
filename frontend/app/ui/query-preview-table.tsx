"use client";

import type { QueryPreview } from "@/lib/api";

// formatCell renders whatever the data source returned. Values arrive as raw
// JSON, so anything that isn't a primitive is shown as its JSON text rather
// than as "[object Object]".
function formatCell(value: unknown): string {
  if (value === null || value === undefined) return "—";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

// QueryPreviewTable shows the first rows a generated query returned, so the
// user can judge the query before saving the report.
export function QueryPreviewTable({ preview }: { preview: QueryPreview }) {
  if (preview.columns.length === 0) {
    return <p className="text-sm text-zinc-600 dark:text-zinc-400">The query returned no columns.</p>;
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="overflow-x-auto rounded border border-black/[.1] dark:border-white/[.15]">
        <table className="w-full border-collapse text-left text-sm">
          <thead>
            <tr className="border-b border-black/[.1] dark:border-white/[.15]">
              {preview.columns.map((column, index) => (
                <th key={`${column}-${index}`} className="px-3 py-2 font-medium whitespace-nowrap">
                  {column}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {preview.rows.map((row, rowIndex) => (
              <tr
                key={rowIndex}
                className="border-b border-black/[.05] last:border-0 dark:border-white/[.08]"
              >
                {preview.columns.map((_, cellIndex) => (
                  <td key={cellIndex} className="px-3 py-2 font-mono text-xs whitespace-nowrap">
                    {formatCell(row[cellIndex])}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        {preview.rows.length === 0
          ? "The query returned no rows."
          : `${preview.rows.length} row${preview.rows.length === 1 ? "" : "s"}${
              preview.truncated ? " (preview truncated)" : ""
            }`}
      </p>
    </div>
  );
}
