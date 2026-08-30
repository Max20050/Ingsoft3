"use client";

import { useState, type FormEvent } from "react";
import type { DataSourceInput } from "@/lib/api";

type SqlDataSourceType = "postgres" | "mysql";

const TYPE_OPTIONS: { value: SqlDataSourceType; label: string }[] = [
  { value: "postgres", label: "PostgreSQL" },
  { value: "mysql", label: "MySQL" },
];

const DEFAULT_PORTS: Record<SqlDataSourceType, number> = {
  postgres: 5432,
  mysql: 3306,
};

const inputClass = "rounded border border-black/[.1] px-3 py-2 dark:border-white/[.15] dark:bg-black";

export function DataSourceForm({
  onTest,
  onCreate,
}: {
  onTest: (input: DataSourceInput) => Promise<void>;
  onCreate: (input: DataSourceInput) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [type, setType] = useState<SqlDataSourceType>("postgres");
  const [host, setHost] = useState("");
  const [port, setPort] = useState(DEFAULT_PORTS.postgres);
  const [dbName, setDbName] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  function currentInput(): DataSourceInput {
    return { name, type, host, port, dbName, username, password };
  }

  async function handleTest() {
    setError(null);
    setStatus(null);
    setPending(true);
    try {
      await onTest(currentInput());
      setStatus("Connection successful");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Connection failed");
    } finally {
      setPending(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setStatus(null);
    setPending(true);
    try {
      await onCreate(currentInput());
      setName("");
      setHost("");
      setDbName("");
      setUsername("");
      setPassword("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex w-full max-w-md flex-col gap-4">
      <div className="flex flex-col gap-1">
        <label htmlFor="ds-name" className="text-sm font-medium">
          Name
        </label>
        <input
          id="ds-name"
          required
          value={name}
          onChange={(e) => setName(e.target.value)}
          className={inputClass}
        />
      </div>
      <div className="flex flex-col gap-1">
        <label htmlFor="ds-type" className="text-sm font-medium">
          Type
        </label>
        <select
          id="ds-type"
          value={type}
          onChange={(e) => {
            const nextType = e.target.value as SqlDataSourceType;
            setType(nextType);
            setPort(DEFAULT_PORTS[nextType]);
          }}
          className={inputClass}
        >
          {TYPE_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>
      <div className="flex gap-4">
        <div className="flex flex-1 flex-col gap-1">
          <label htmlFor="ds-host" className="text-sm font-medium">
            Host
          </label>
          <input
            id="ds-host"
            required
            value={host}
            onChange={(e) => setHost(e.target.value)}
            className={inputClass}
          />
        </div>
        <div className="flex w-28 flex-col gap-1">
          <label htmlFor="ds-port" className="text-sm font-medium">
            Port
          </label>
          <input
            id="ds-port"
            type="number"
            required
            value={port}
            onChange={(e) => setPort(Number(e.target.value))}
            className={inputClass}
          />
        </div>
      </div>
      <div className="flex flex-col gap-1">
        <label htmlFor="ds-dbname" className="text-sm font-medium">
          Database name
        </label>
        <input
          id="ds-dbname"
          required
          value={dbName}
          onChange={(e) => setDbName(e.target.value)}
          className={inputClass}
        />
      </div>
      <div className="flex flex-col gap-1">
        <label htmlFor="ds-username" className="text-sm font-medium">
          Username
        </label>
        <input
          id="ds-username"
          required
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          className={inputClass}
        />
      </div>
      <div className="flex flex-col gap-1">
        <label htmlFor="ds-password" className="text-sm font-medium">
          Password
        </label>
        <input
          id="ds-password"
          type="password"
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className={inputClass}
        />
      </div>
      {error && <p className="text-sm text-red-600">{error}</p>}
      {status && <p className="text-sm text-green-600">{status}</p>}
      <div className="flex gap-3">
        <button
          type="button"
          onClick={handleTest}
          disabled={pending}
          className="rounded-full border border-black/[.08] px-5 py-2 transition-colors hover:bg-black/[.04] disabled:opacity-50 dark:border-white/[.145] dark:hover:bg-[#1a1a1a]"
        >
          Test connection
        </button>
        <button
          type="submit"
          disabled={pending}
          className="rounded-full bg-foreground px-5 py-2 text-background transition-colors hover:bg-[#383838] disabled:opacity-50 dark:hover:bg-[#ccc]"
        >
          {pending ? "Please wait…" : "Save data source"}
        </button>
      </div>
    </form>
  );
}
