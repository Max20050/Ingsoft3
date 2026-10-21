import * as Sentry from "@sentry/nextjs";

export function register() {
  // Server-side rendering/route-handler errors. Reads from the environment,
  // same as the backend, since this half of the SDK never ships to the
  // browser.
  Sentry.init({
    dsn: process.env.SENTRY_DSN ?? "",
    tracesSampleRate: 1.0,
  });
}

export const onRequestError = Sentry.captureRequestError;
