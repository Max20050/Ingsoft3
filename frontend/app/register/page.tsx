"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { register } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { AuthForm } from "@/app/ui/auth-form";
import { GoogleSignInButton } from "@/app/ui/google-signin-button";

export default function RegisterPage() {
  const router = useRouter();
  const { setToken } = useAuth();

  async function handleSubmit(email: string, password: string) {
    const { token } = await register(email, password);
    setToken(token);
    router.push("/dashboard");
  }

  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-6 bg-zinc-50 px-6 py-32 dark:bg-black">
      <h1 className="text-2xl font-semibold">Create your account</h1>
      <AuthForm submitLabel="Sign up" onSubmit={handleSubmit} />
      <div className="flex w-full max-w-sm items-center gap-3 text-xs uppercase text-zinc-400">
        <div className="h-px flex-1 bg-black/[.1] dark:bg-white/[.15]" />
        or
        <div className="h-px flex-1 bg-black/[.1] dark:bg-white/[.15]" />
      </div>
      <GoogleSignInButton />
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        Already have an account?{" "}
        <Link href="/login" className="font-medium underline">
          Log in
        </Link>
      </p>
    </div>
  );
}
