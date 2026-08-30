"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";

const NAV_ITEMS = [
  { href: "/dashboard", label: "Dashboard" },
  { href: "/reports", label: "Reports" },
  { href: "/datasources", label: "Connectors" },
  { href: "/recipients", label: "Recipients" },
  { href: "/settings", label: "Settings" },
];

// Sidebar is the app's persistent navigation, shown on every authenticated
// page via app/(app)/layout.tsx.
export function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { logout } = useAuth();

  function handleLogout() {
    logout();
    router.push("/login");
  }

  return (
    <nav className="flex w-56 shrink-0 flex-col justify-between border-r border-black/[.1] px-4 py-6 dark:border-white/[.15]">
      <div className="flex flex-col gap-1">
        <p className="px-2 pb-4 text-lg font-semibold">DocuWave</p>
        {NAV_ITEMS.map((item) => {
          const active = pathname === item.href || pathname.startsWith(`${item.href}/`);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`rounded px-3 py-2 text-sm transition-colors ${
                active
                  ? "bg-black/[.06] font-medium dark:bg-white/[.1]"
                  : "hover:bg-black/[.04] dark:hover:bg-white/[.06]"
              }`}
            >
              {item.label}
            </Link>
          );
        })}
      </div>
      <button
        onClick={handleLogout}
        className="rounded px-3 py-2 text-left text-sm transition-colors hover:bg-black/[.04] dark:hover:bg-white/[.06]"
      >
        Log out
      </button>
    </nav>
  );
}
