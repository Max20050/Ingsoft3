"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { login } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { AuthForm } from "@/app/ui/auth-form";
import { GoogleSignInButton } from "@/app/ui/google-signin-button";

export default function LoginPage() {
  const router = useRouter();
  const { setToken } = useAuth();

  async function handleSubmit(email: string, password: string) {
    const { token } = await login(email, password);
    setToken(token);
    router.push("/dashboard");
  }

  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-6 bg-zinc-50 px-6 py-32 dark:bg-black">
      <h1 className="text-2xl font-semibold">Log in</h1>
      <AuthForm submitLabel="Log in" onSubmit={handleSubmit} />
      <div className="flex w-full max-w-sm items-center gap-3 text-xs uppercase text-zinc-400">
        <div className="h-px flex-1 bg-black/[.1] dark:bg-white/[.15]" />
        or
        <div className="h-px flex-1 bg-black/[.1] dark:bg-white/[.15]" />
      </div>
      <GoogleSignInButton />
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        Don&apos;t have an account?{" "}
        <Link href="/register" className="font-medium underline">
          Sign up
        </Link>
      </p>
    </div>
  );
}
