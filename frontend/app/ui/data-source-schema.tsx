"use client";

import { useEffect, useState } from "react";
import {
  getDataSourceSchema,
  refreshDataSourceSchema,
  type DataSourceSchema,
  type DataSourceType,
} from "@/lib/api";

// DataSourceSchemaView loads and shows the structure of the selected data
// source: tables and columns for SQL, header/detected fields for Google
// Sheets and REST API sources. `type` is passed in rather than read off the
// response, since the caller already knows it and the schema response itself
// doesn't carry the source's type.
export function DataSourceSchemaView({
  token,
  dataSourceId,
  type,
}: {
  token: string;
  dataSourceId: string;
  type: DataSourceType;
}) {
  const [schema, setSchema] = useState<DataSourceSchema | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  // Callers key this component by data source id, so a different source
  // remounts it rather than reusing the previous source's state.
  useEffect(() => {
    let active = true;

    getDataSourceSchema(token, dataSourceId)
      .then((result) => {
        if (active) setSchema(result);
      })
      .catch((err) => {
        if (active) setError(err instanceof Error ? err.message : "Failed to read schema");
      });

    return () => {
      active = false;
    };
  }, [token, dataSourceId]);

  const handleRefresh = () => {
    setRefreshing(true);
    setError(null);
    refreshDataSourceSchema(token, dataSourceId)
      .then(setSchema)
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to refresh schema"))
      .finally(() => setRefreshing(false));
  };

  const refreshButton = (
    <button
      onClick={handleRefresh}
      disabled={refreshing}
      className="self-start text-sm underline disabled:opacity-50"
    >
      {refreshing ? "Detecting…" : "Detectar campos"}
    </button>
  );

  if (error) {
    return (
      <div className="flex flex-col gap-2">
        <p className="text-sm text-red-600">{error}</p>
        {refreshButton}
      </div>
    );
  }

  if (!schema) {
    return <p className="text-sm text-zinc-600 dark:text-zinc-400">Reading schema…</p>;
  }

  if (type === "google_sheets" || type === "rest_api") {
    const fields = schema.fields ?? [];
    if (fields.length === 0) {
      return (
        <div className="flex flex-col gap-2">
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            {type === "rest_api" ? "No fields detected yet." : "This sheet has no header row."}
          </p>
          {refreshButton}
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-3">
        <div className="flex flex-wrap gap-2">
          {fields.map((field, index) => {
            const fieldType = schema.fieldTypes?.[field];
            return (
              <span
                key={`${field}-${index}`}
                className="rounded bg-black/[.05] px-2 py-1 font-mono text-xs dark:bg-white/[.08]"
              >
                {field || "(unnamed)"}
                {fieldType && <span className="text-zinc-500"> {fieldType}</span>}
              </span>
            );
          })}
        </div>
        {refreshButton}
      </div>
    );
  }

  if (!schema.tables || schema.tables.length === 0) {
    return (
      <div className="flex flex-col gap-2">
        <p className="text-sm text-zinc-600 dark:text-zinc-400">No tables found.</p>
        {refreshButton}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      {schema.tables.map((table) => (
        <div key={table.name}>
          <p className="font-mono text-sm font-medium">{table.name}</p>
          <ul className="mt-1 flex flex-col gap-0.5">
            {table.columns.map((column) => (
              <li key={column.name} className="font-mono text-xs text-zinc-600 dark:text-zinc-400">
                {column.name} <span className="text-zinc-500">{column.type}</span>
              </li>
            ))}
          </ul>
        </div>
      ))}
      {refreshButton}
    </div>
  );
}
