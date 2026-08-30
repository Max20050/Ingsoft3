import { API_URL } from "@/lib/api";

export function GoogleSignInButton() {
  return (
    <a
      href={`${API_URL}/api/auth/google/login`}
      className="flex w-full max-w-sm items-center justify-center gap-2 rounded-full border border-black/[.1] px-5 py-2 transition-colors hover:bg-black/[.03] dark:border-white/[.15] dark:hover:bg-white/[.06]"
    >
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
        <path
          fill="#4285F4"
          d="M23.49 12.27c0-.79-.07-1.54-.19-2.27H12v4.51h6.44c-.29 1.48-1.14 2.73-2.42 3.58v2.98h3.93c2.3-2.12 3.54-5.24 3.54-8.8z"
        />
        <path
          fill="#34A853"
          d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.93-2.98c-1.08.72-2.47 1.16-4 1.16-3.08 0-5.68-2.08-6.62-4.87H1.32v3.06C3.29 21.3 7.32 24 12 24z"
        />
        <path
          fill="#FBBC05"
          d="M5.38 14.4c-.24-.72-.38-1.49-.38-2.4s.14-1.68.38-2.4V6.54H1.32C.48 8.2 0 10.05 0 12s.48 3.8 1.32 5.46z"
        />
        <path
          fill="#EA4335"
          d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.44-3.44C17.95 1.19 15.24 0 12 0 7.32 0 3.29 2.7 1.32 6.54l4.06 3.06C6.32 6.83 8.92 4.75 12 4.75z"
        />
      </svg>
      Continue with Google
    </a>
  );
}
