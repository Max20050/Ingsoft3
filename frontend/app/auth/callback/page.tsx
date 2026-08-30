"use client";

import { Suspense, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth-context";

function GoogleCallbackContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { setToken } = useAuth();

  useEffect(() => {
    const token = searchParams.get("token");
    if (!token) {
      router.replace("/login");
      return;
    }
    setToken(token);
    router.replace("/dashboard");
  }, [searchParams, setToken, router]);

  return (
    <div className="flex flex-1 items-center justify-center py-32">
      <p>Signing you in…</p>
    </div>
  );
}

export default function GoogleCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="flex flex-1 items-center justify-center py-32">
          <p>Signing you in…</p>
        </div>
      }
    >
      <GoogleCallbackContent />
    </Suspense>
  );
}
