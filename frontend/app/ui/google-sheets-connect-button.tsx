import { googleSheetsConnectUrl } from "@/lib/api";

export function GoogleSheetsConnectButton({ token }: { token: string }) {
  return (
    <a
      href={googleSheetsConnectUrl(token)}
      className="flex w-full max-w-md items-center justify-center gap-2 rounded-full border border-black/[.1] px-5 py-2 transition-colors hover:bg-black/[.03] dark:border-white/[.15] dark:hover:bg-white/[.06]"
    >
      Connect Google Sheets
    </a>
  );
}
