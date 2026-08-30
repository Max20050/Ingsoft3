"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  deleteLLMConfig,
  getLLMConfig,
  saveLLMConfig,
  type LLMConfig,
  type LLMConfigInput,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { LLMConfigForm } from "@/app/ui/llm-config-form";

const PROVIDER_LABELS: Record<string, string> = {
  claude: "Claude",
  openai: "GPT-4o (OpenAI)",
};

export default function SettingsPage() {
  const router = useRouter();
  const { token, logout } = useAuth();
  const [config, setConfig] = useState<LLMConfig | null | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    getLLMConfig(token)
      .then(setConfig)
      .catch(() => {
        logout();
        router.replace("/login");
      });
  }, [token, router, logout]);

  async function handleSave(input: LLMConfigInput) {
    if (!token) return;
    const saved = await saveLLMConfig(token, input);
    setConfig(saved);
  }

  async function handleDelete() {
    if (!token) return;
    setError(null);
    try {
      await deleteLLMConfig(token);
      setConfig(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove LLM configuration");
    }
  }

  if (!token || config === undefined) {
    return (
      <div className="flex flex-1 items-center justify-center py-32">
        <p>Loading…</p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col items-center gap-10 px-6 py-16">
      <div className="flex w-full max-w-md items-center justify-between">
        <h1 className="text-2xl font-semibold">Settings</h1>
      </div>

      <div className="flex w-full max-w-md flex-col gap-4">
        <h2 className="text-lg font-semibold">LLM provider</h2>
        {config ? (
          <div className="flex items-center justify-between rounded border border-black/[.1] px-4 py-3 dark:border-white/[.15]">
            <div>
              <p className="font-medium">{PROVIDER_LABELS[config.provider] ?? config.provider}</p>
              <p className="text-sm text-zinc-600 dark:text-zinc-400">API key configured</p>
            </div>
            <button onClick={handleDelete} className="text-sm text-red-600 hover:underline">
              Remove
            </button>
          </div>
        ) : (
          <p className="text-sm text-zinc-600 dark:text-zinc-400">No LLM provider configured yet.</p>
        )}

        {error && <p className="text-sm text-red-600">{error}</p>}

        <div className="flex flex-col gap-4 border-t border-black/[.1] pt-8 dark:border-white/[.15]">
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            {config ? "Update your LLM configuration" : "Add an LLM configuration"}
          </p>
          <LLMConfigForm onSave={handleSave} />
        </div>
      </div>
    </div>
  );
}
