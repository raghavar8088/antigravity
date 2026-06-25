/**
 * Playwright config for engine API tests — no Next.js server, no auth.
 * Run: ENGINE_URL=http://13.233.8.80 npx playwright test --config=playwright.engine-api.config.ts
 */
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  testMatch: "**/backtest-engine-api.spec.ts",
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: [["list"]],
  timeout: 150_000,
  use: {
    ignoreHTTPSErrors: true,
  },
  // No globalSetup, no webServer — engine is external
});
