"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { Sidebar } from "@/app/ui/sidebar";

// This layout is the single place that gates every authenticated page behind
// a session: pages under app/(app)/ no longer each redirect to /login
// themselves. It also renders the sidebar every one of them shares.
export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { token } = useAuth();

  useEffect(() => {
    if (!token) {
      router.replace("/login");
    }
  }, [token, router]);

  if (!token) {
    return (
      <div className="flex flex-1 items-center justify-center py-32">
        <p>Loading…</p>
      </div>
    );
  }

  return (
    <div className="flex flex-1">
      <Sidebar />
      <main className="flex flex-1 flex-col">{children}</main>
    </div>
  );
}
