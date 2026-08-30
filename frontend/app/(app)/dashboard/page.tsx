"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  authFetch,
  listDataSources,
  listRecipientGroups,
  listRecipients,
  listReports,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

type Profile = { id: string; email: string };

type Summary = {
  dataSources: number;
  reports: number;
  recipients: number;
  recipientGroups: number;
};

const CARDS: { key: keyof Summary; label: string; href: string }[] = [
  { key: "reports", label: "Reports", href: "/reports" },
  { key: "dataSources", label: "Connectors", href: "/datasources" },
  { key: "recipients", label: "Recipients", href: "/recipients" },
  { key: "recipientGroups", label: "Recipient groups", href: "/recipients" },
];

export default function DashboardPage() {
  const router = useRouter();
  const { token, logout } = useAuth();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [summary, setSummary] = useState<Summary | null>(null);

  useEffect(() => {
    if (!token) return;

    Promise.all([
      authFetch("/api/me", token).then((res) => res.json() as Promise<Profile>),
      listReports(token),
      listDataSources(token),
      listRecipients(token),
      listRecipientGroups(token),
    ])
      .then(([loadedProfile, reports, dataSources, recipients, recipientGroups]) => {
        setProfile(loadedProfile);
        setSummary({
          reports: reports.length,
          dataSources: dataSources.length,
          recipients: recipients.length,
          recipientGroups: recipientGroups.length,
        });
      })
      .catch(() => {
        logout();
        router.replace("/login");
      });
  }, [token, router, logout]);

  if (!profile || !summary) {
    return (
      <div className="flex flex-1 items-center justify-center py-32">
        <p>Loading…</p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col gap-8 px-8 py-10">
      <h1 className="text-2xl font-semibold">Welcome, {profile.email}</h1>

      <div className="grid w-full max-w-3xl grid-cols-2 gap-4 sm:grid-cols-4">
        {CARDS.map((card) => (
          <Link
            key={card.key}
            href={card.href}
            className="flex flex-col gap-1 rounded border border-black/[.1] px-4 py-3 transition-colors hover:bg-black/[.04] dark:border-white/[.15] dark:hover:bg-white/[.06]"
          >
            <span className="text-2xl font-semibold">{summary[card.key]}</span>
            <span className="text-sm text-zinc-600 dark:text-zinc-400">{card.label}</span>
          </Link>
        ))}
      </div>
    </div>
  );
}
