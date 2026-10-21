import * as Sentry from "@sentry/nextjs";

// The DSN only lets this app SEND events to Sentry, not read them, so it's
// fine to ship in a public repo's client bundle (TP9 §3.5) — unlike the
// backend's SENTRY_DSN, which travels by environment variable.
Sentry.init({
  dsn: "https://d80259298b197662c0adb3bcabee2319@o4512292123705344.ingest.us.sentry.io/4512292137730048",
  environment: typeof window !== "undefined" ? window.location.hostname : "unknown",
  integrations: [Sentry.browserTracingIntegration()],
  tracesSampleRate: 1.0,
});
