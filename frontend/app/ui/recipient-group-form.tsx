"use client";

import { useState, type FormEvent } from "react";

const inputClass = "rounded border border-black/[.1] px-3 py-2 dark:border-white/[.15] dark:bg-black";

export function RecipientGroupForm({ onCreate }: { onCreate: (name: string) => Promise<void> }) {
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setPending(true);
    try {
      await onCreate(name);
      setName("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save recipient group");
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex w-full max-w-md flex-col gap-4">
      <div className="flex flex-col gap-1">
        <label htmlFor="group-name" className="text-sm font-medium">
          Group name
        </label>
        <input
          id="group-name"
          required
          value={name}
          onChange={(e) => setName(e.target.value)}
          className={inputClass}
        />
      </div>
      {error && <p className="text-sm text-red-600">{error}</p>}
      <div>
        <button
          type="submit"
          disabled={pending}
          className="rounded-full bg-foreground px-5 py-2 text-background transition-colors hover:bg-[#383838] disabled:opacity-50 dark:hover:bg-[#ccc]"
        >
          {pending ? "Saving…" : "Save group"}
        </button>
      </div>
    </form>
  );
}
