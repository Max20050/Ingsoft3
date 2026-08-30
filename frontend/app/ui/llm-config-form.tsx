"use client";

import { useState, type FormEvent } from "react";
import type { LLMConfigInput, LLMProviderType } from "@/lib/api";

const PROVIDER_OPTIONS: { value: LLMProviderType; label: string }[] = [
  { value: "claude", label: "Claude" },
  { value: "openai", label: "GPT-4o (OpenAI)" },
  { value: "openrouter", label: "OpenRouter" },
];

const inputClass = "rounded border border-black/[.1] px-3 py-2 dark:border-white/[.15] dark:bg-black";

export function LLMConfigForm({
  onSave,
}: {
  onSave: (input: LLMConfigInput) => Promise<void>;
}) {
  const [provider, setProvider] = useState<LLMProviderType>("claude");
  const [apiKey, setApiKey] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setStatus(null);
    setPending(true);
    try {
      await onSave({ provider, apiKey });
      setApiKey("");
      setStatus("LLM configuration saved");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex w-full max-w-md flex-col gap-4">
      <div className="flex flex-col gap-1">
        <label htmlFor="llm-provider" className="text-sm font-medium">
          Provider
        </label>
        <select
          id="llm-provider"
          value={provider}
          onChange={(e) => setProvider(e.target.value as LLMProviderType)}
          className={inputClass}
        >
          {PROVIDER_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>
      <div className="flex flex-col gap-1">
        <label htmlFor="llm-api-key" className="text-sm font-medium">
          API key
        </label>
        <input
          id="llm-api-key"
          type="password"
          required
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          className={inputClass}
        />
      </div>
      {error && <p className="text-sm text-red-600">{error}</p>}
      {status && <p className="text-sm text-green-600">{status}</p>}
      <div className="flex gap-3">
        <button
          type="submit"
          disabled={pending}
          className="rounded-full bg-foreground px-5 py-2 text-background transition-colors hover:bg-[#383838] disabled:opacity-50 dark:hover:bg-[#ccc]"
        >
          {pending ? "Please wait…" : "Save LLM configuration"}
        </button>
      </div>
    </form>
  );
}
