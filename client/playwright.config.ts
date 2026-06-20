import { defineConfig, devices } from "@playwright/test";

// ── E2E test-only credentials ─────────────────────────────────────────────────
// Auth (ownerAuth.ts) only reads ADMIN_USERNAME / ADMIN_PASSWORD_HASH / AUTH_JWT_SECRET
// from process.env — no MongoDB or Go engine required. These are injected directly
// into the spawned `next dev` process via webServer.env below (not via a .env file,
// since Next dev only auto-loads .env.development[.local], not .env.test).
// Override any of these from the environment (e.g. CI secrets) if needed.
export const E2E_ADMIN_USERNAME = process.env.E2E_ADMIN_USERNAME ?? "e2e_test_owner";
export const E2E_ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? "E2E-Test-Pass-2026!";
// scrypt hash of E2E_ADMIN_PASSWORD above, in this repo's "scrypt:<salt>:<hash>" format.
export const E2E_ADMIN_PASSWORD_HASH =
  process.env.E2E_ADMIN_PASSWORD_HASH ??
  "scrypt:c247596b52e671213b619f68cfce25a0:01e84dd287a53a0cd3a483dc84a1475e7dcfc47695f116096d47b6b2374fec216ea48a51f4985d3c5e8ea16885f1cafbabc580cb6545fa2ad90612ebe1598fd3";
export const E2E_AUTH_JWT_SECRET =
  process.env.E2E_AUTH_JWT_SECRET ??
  "4338f11f9d9a09d24b1fad140fc5d0a1da78d6c0215551bb41db5cc929d8e19b";

const PORT = 3000;
const BASE_URL = `http://localhost:${PORT}`;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : [["html", { open: "never" }]],
  globalSetup: "./e2e/global-setup.ts",
  timeout: 30_000,
  use: {
    baseURL: BASE_URL,
    trace: "on-first-retry",
    video: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium-desktop",
      use: { ...devices["Desktop Chrome"] },
      testIgnore: /mobile\//,
    },
    {
      name: "chromium-mobile",
      use: { ...devices["Pixel 7"] },
      testMatch: /mobile\//,
    },
  ],
  webServer: {
    // CI builds once and serves the production bundle: `next dev`'s
    // on-demand per-route compilation caused real, repeatable flakiness once
    // many specs hit different routes concurrently against one shared dev
    // server (confirmed by isolated runs being 100% reliable while full-suite
    // parallel runs occasionally weren't). Locally, `next dev` is kept for
    // fast iteration — reuseExistingServer means a dev already running
    // `npm run dev` is reused as-is.
    command: process.env.CI
      ? `npm run build && npm run start -- --port ${PORT}`
      : `npm run dev -- --port ${PORT}`,
    url: BASE_URL,
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
    env: {
      ADMIN_USERNAME: E2E_ADMIN_USERNAME,
      ADMIN_PASSWORD_HASH: E2E_ADMIN_PASSWORD_HASH,
      AUTH_JWT_SECRET: E2E_AUTH_JWT_SECRET,
    },
  },
});
