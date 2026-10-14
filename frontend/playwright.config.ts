import { defineConfig } from "@playwright/test";

// En docker-compose (TP2) el backend no publica puerto propio: el único
// publicado es el 80 del frontend, cuyo nginx hace proxy de /api/ hacia el
// backend (frontend/nginx/default.conf.template). Por eso, local, la api y
// el front comparten host. En QA/PROD (Render) son dos servicios con dos
// URLs públicas distintas (TP6): ahí sí difieren.
export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: process.env.E2E_BASE_URL || "http://localhost",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  retries: 1,
  reporter: [["html", { open: "never" }], ["list"]],
});
