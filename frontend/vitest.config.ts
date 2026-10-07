import { defineConfig } from "vitest/config";
import path from "node:path";

export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "."),
    },
  },
  test: {
    environment: "node",
    coverage: {
      provider: "v8",
      reporter: ["text", "html", "lcov", "json-summary"],
      // Only the pure logic counts: lib/api.ts is a thin CRUD client with no
      // branches of its own to speak of, and auth-context.tsx is React
      // Context wiring. Neither has a business rule a test could verify.
      include: ["lib/report-builder-helpers.ts"],
      thresholds: {
        lines: 90,
        branches: 90,
      },
    },
  },
});
