"use client";

import { useState, type FormEvent } from "react";
import type { RecipientInput } from "@/lib/api";

const inputClass = "rounded border border-black/[.1] px-3 py-2 dark:border-white/[.15] dark:bg-black";
const removeButtonClass =
  "rounded border border-black/[.1] px-2 py-1 text-xs transition-colors hover:bg-black/[.04] dark:border-white/[.15] dark:hover:bg-[#1a1a1a]";

type AttributeRow = { key: string; value: string };

export function RecipientForm({ onCreate }: { onCreate: (input: RecipientInput) => Promise<void> }) {
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [attributes, setAttributes] = useState<AttributeRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  function updateAttribute(index: number, row: AttributeRow) {
    setAttributes(attributes.map((a, i) => (i === index ? row : a)));
  }
  function removeAttribute(index: number) {
    setAttributes(attributes.filter((_, i) => i !== index));
  }
  function addAttribute() {
    setAttributes([...attributes, { key: "", value: "" }]);
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setPending(true);
    try {
      const attributeObject = Object.fromEntries(
        attributes.filter((row) => row.key.trim() !== "").map((row) => [row.key.trim(), row.value]),
      );
      await onCreate({ email, name, attributes: attributeObject });
      setEmail("");
      setName("");
      setAttributes([]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save recipient");
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex w-full max-w-md flex-col gap-4">
      <div className="flex flex-col gap-1">
        <label htmlFor="recipient-email" className="text-sm font-medium">
          Email
        </label>
        <input
          id="recipient-email"
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className={inputClass}
        />
      </div>
      <div className="flex flex-col gap-1">
        <label htmlFor="recipient-name" className="text-sm font-medium">
          Name
        </label>
        <input
          id="recipient-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className={inputClass}
        />
      </div>

      <fieldset className="flex flex-col gap-2">
        <legend className="text-sm font-medium">Attributes</legend>
        <p className="text-sm text-zinc-600 dark:text-zinc-400">
          Free-form values a report&apos;s inputs can filter by, e.g. region or department.
        </p>
        {attributes.map((row, index) => (
          <div key={index} className="flex items-center gap-2">
            <input
              value={row.key}
              onChange={(e) => updateAttribute(index, { ...row, key: e.target.value })}
              placeholder="key"
              className={`${inputClass} py-1 text-sm`}
            />
            <input
              value={row.value}
              onChange={(e) => updateAttribute(index, { ...row, value: e.target.value })}
              placeholder="value"
              className={`${inputClass} py-1 text-sm`}
            />
            <button type="button" onClick={() => removeAttribute(index)} className={removeButtonClass}>
              Remove
            </button>
          </div>
        ))}
        <div>
          <button type="button" onClick={addAttribute} className={removeButtonClass}>
            Add attribute
          </button>
        </div>
      </fieldset>

      {error && <p className="text-sm text-red-600">{error}</p>}
      <div>
        <button
          type="submit"
          disabled={pending}
          className="rounded-full bg-foreground px-5 py-2 text-background transition-colors hover:bg-[#383838] disabled:opacity-50 dark:hover:bg-[#ccc]"
        >
          {pending ? "Saving…" : "Save recipient"}
        </button>
      </div>
    </form>
  );
}
