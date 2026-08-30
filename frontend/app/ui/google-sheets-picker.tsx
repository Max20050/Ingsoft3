"use client";

import { useEffect, useState, type FormEvent } from "react";
import {
  listGoogleSheetsSpreadsheets,
  type GoogleSheetsDataSourceInput,
  type GoogleSheetsSpreadsheet,
} from "@/lib/api";

const inputClass = "rounded border border-black/[.1] px-3 py-2 dark:border-white/[.15] dark:bg-black";

export function GoogleSheetsPicker({
  token,
  connectionId,
  onCreate,
}: {
  token: string;
  connectionId: string;
  onCreate: (input: GoogleSheetsDataSourceInput) => Promise<void>;
}) {
  const [sheets, setSheets] = useState<GoogleSheetsSpreadsheet[] | null>(null);
  const [name, setName] = useState("");
  const [spreadsheetId, setSpreadsheetId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  useEffect(() => {
    listGoogleSheetsSpreadsheets(token, connectionId)
      .then((result) => {
        setSheets(result);
        if (result.length > 0) {
          setSpreadsheetId(result[0].id);
          setName(result[0].name);
        }
      })
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to list spreadsheets"));
  }, [token, connectionId]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const sheet = sheets?.find((s) => s.id === spreadsheetId);
    if (!sheet) return;
    setError(null);
    setPending(true);
    try {
      await onCreate({ name, connectionId, spreadsheetId: sheet.id, spreadsheetName: sheet.name });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to add data source");
    } finally {
      setPending(false);
    }
  }

  if (error) {
    return <p className="text-sm text-red-600">{error}</p>;
  }

  if (!sheets) {
    return <p className="text-sm text-zinc-600 dark:text-zinc-400">Loading your spreadsheets…</p>;
  }

  if (sheets.length === 0) {
    return (
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        No spreadsheets found in the connected account.
      </p>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="flex w-full max-w-md flex-col gap-4">
      <div className="flex flex-col gap-1">
        <label htmlFor="sheet-select" className="text-sm font-medium">
          Spreadsheet
        </label>
        <select
          id="sheet-select"
          value={spreadsheetId}
          onChange={(e) => {
            setSpreadsheetId(e.target.value);
            const sheet = sheets.find((s) => s.id === e.target.value);
            if (sheet) setName(sheet.name);
          }}
          className={inputClass}
        >
          {sheets.map((sheet) => (
            <option key={sheet.id} value={sheet.id}>
              {sheet.name}
            </option>
          ))}
        </select>
      </div>
      <div className="flex flex-col gap-1">
        <label htmlFor="sheet-name" className="text-sm font-medium">
          Name
        </label>
        <input
          id="sheet-name"
          required
          value={name}
          onChange={(e) => setName(e.target.value)}
          className={inputClass}
        />
      </div>
      <button
        type="submit"
        disabled={pending}
        className="rounded-full bg-foreground px-5 py-2 text-background transition-colors hover:bg-[#383838] disabled:opacity-50 dark:hover:bg-[#ccc]"
      >
        {pending ? "Please wait…" : "Use this spreadsheet"}
      </button>
    </form>
  );
}
