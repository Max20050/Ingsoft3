"use client";

import { useState, type FormEvent } from "react";
import type { RestApiDataSourceInput, RestAuthType, RestHeader } from "@/lib/api";

const HTTP_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"];

const AUTH_OPTIONS: { value: RestAuthType; label: string }[] = [
  { value: "none", label: "None" },
  { value: "basic", label: "Basic (username/password)" },
  { value: "bearer", label: "Bearer token" },
  { value: "api_key", label: "API key header" },
];

const inputClass = "rounded border border-black/[.1] px-3 py-2 dark:border-white/[.15] dark:bg-black";

function buildAuth(
  type: RestAuthType,
  fields: { username: string; password: string; token: string; headerName: string; headerValue: string },
): RestApiDataSourceInput["auth"] {
  switch (type) {
    case "basic":
      return { type, username: fields.username, password: fields.password };
    case "bearer":
      return { type, token: fields.token };
    case "api_key":
      return { type, headerName: fields.headerName, headerValue: fields.headerValue };
    default:
      return { type: "none" };
  }
}

export function RestApiDataSourceForm({
  onTest,
  onCreate,
}: {
  onTest: (input: RestApiDataSourceInput) => Promise<void>;
  onCreate: (input: RestApiDataSourceInput) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [method, setMethod] = useState("GET");
  const [headers, setHeaders] = useState<RestHeader[]>([]);
  const [authType, setAuthType] = useState<RestAuthType>("none");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [token, setToken] = useState("");
  const [headerName, setHeaderName] = useState("");
  const [headerValue, setHeaderValue] = useState("");
  const [body, setBody] = useState("");
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  function currentInput(): RestApiDataSourceInput {
    return {
      name,
      url,
      method,
      headers: headers.filter((h) => h.key.trim() !== ""),
      auth: buildAuth(authType, { username, password, token, headerName, headerValue }),
      body: body || undefined,
    };
  }

  function updateHeader(index: number, field: keyof RestHeader, value: string) {
    setHeaders((prev) => prev.map((h, i) => (i === index ? { ...h, [field]: value } : h)));
  }

  function removeHeader(index: number) {
    setHeaders((prev) => prev.filter((_, i) => i !== index));
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
      setUrl("");
      setHeaders([]);
      setBody("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex w-full max-w-md flex-col gap-4">
      <div className="flex flex-col gap-1">
        <label htmlFor="rest-name" className="text-sm font-medium">
          Name
        </label>
        <input
          id="rest-name"
          required
          value={name}
          onChange={(e) => setName(e.target.value)}
          className={inputClass}
        />
      </div>

      <div className="flex gap-4">
        <div className="flex flex-1 flex-col gap-1">
          <label htmlFor="rest-url" className="text-sm font-medium">
            URL
          </label>
          <input
            id="rest-url"
            type="url"
            required
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            className={inputClass}
            placeholder="https://api.example.com/orders"
          />
        </div>
        <div className="flex w-32 flex-col gap-1">
          <label htmlFor="rest-method" className="text-sm font-medium">
            Method
          </label>
          <select
            id="rest-method"
            value={method}
            onChange={(e) => setMethod(e.target.value)}
            className={inputClass}
          >
            {HTTP_METHODS.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between">
          <span className="text-sm font-medium">Headers</span>
          <button
            type="button"
            onClick={() => setHeaders((prev) => [...prev, { key: "", value: "" }])}
            className="text-sm underline"
          >
            Add header
          </button>
        </div>
        {headers.map((header, index) => (
          <div key={index} className="flex gap-2">
            <input
              value={header.key}
              onChange={(e) => updateHeader(index, "key", e.target.value)}
              placeholder="Header name"
              className={`${inputClass} flex-1`}
            />
            <input
              value={header.value}
              onChange={(e) => updateHeader(index, "value", e.target.value)}
              placeholder="Value"
              className={`${inputClass} flex-1`}
            />
            <button
              type="button"
              onClick={() => removeHeader(index)}
              className="text-sm text-red-600 hover:underline"
            >
              Remove
            </button>
          </div>
        ))}
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="rest-auth-type" className="text-sm font-medium">
          Authentication
        </label>
        <select
          id="rest-auth-type"
          value={authType}
          onChange={(e) => setAuthType(e.target.value as RestAuthType)}
          className={inputClass}
        >
          {AUTH_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>

      {authType === "basic" && (
        <div className="flex gap-4">
          <div className="flex flex-1 flex-col gap-1">
            <label htmlFor="rest-username" className="text-sm font-medium">
              Username
            </label>
            <input
              id="rest-username"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className={inputClass}
            />
          </div>
          <div className="flex flex-1 flex-col gap-1">
            <label htmlFor="rest-password" className="text-sm font-medium">
              Password
            </label>
            <input
              id="rest-password"
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={inputClass}
            />
          </div>
        </div>
      )}

      {authType === "bearer" && (
        <div className="flex flex-col gap-1">
          <label htmlFor="rest-token" className="text-sm font-medium">
            Token
          </label>
          <input
            id="rest-token"
            type="password"
            required
            value={token}
            onChange={(e) => setToken(e.target.value)}
            className={inputClass}
          />
        </div>
      )}

      {authType === "api_key" && (
        <div className="flex gap-4">
          <div className="flex flex-1 flex-col gap-1">
            <label htmlFor="rest-header-name" className="text-sm font-medium">
              Header name
            </label>
            <input
              id="rest-header-name"
              required
              value={headerName}
              onChange={(e) => setHeaderName(e.target.value)}
              className={inputClass}
              placeholder="X-API-Key"
            />
          </div>
          <div className="flex flex-1 flex-col gap-1">
            <label htmlFor="rest-header-value" className="text-sm font-medium">
              Header value
            </label>
            <input
              id="rest-header-value"
              type="password"
              required
              value={headerValue}
              onChange={(e) => setHeaderValue(e.target.value)}
              className={inputClass}
            />
          </div>
        </div>
      )}

      <div className="flex flex-col gap-1">
        <label htmlFor="rest-body" className="text-sm font-medium">
          Request body (optional)
        </label>
        <textarea
          id="rest-body"
          value={body}
          onChange={(e) => setBody(e.target.value)}
          rows={3}
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
