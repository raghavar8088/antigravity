import { test, expect } from "../fixtures/marketData";

// /paper-desk, /paperdesk, and /btc-future-trading are kept public for
// backward compat (src/middleware.ts PUBLIC_PATHS comment) — no auth, no
// engine required. Smoke-only: confirm they render without auth redirects
// or uncaught errors.
const LEGACY_PUBLIC_ROUTES = ["/paper-desk", "/paperdesk", "/btc-future-trading"];

test.describe("legacy public routes", () => {
  for (const route of LEGACY_PUBLIC_ROUTES) {
    test(`${route} loads without redirecting to /login`, async ({ page }) => {
      const pageErrors: Error[] = [];
      page.on("pageerror", (err) => pageErrors.push(err));

      const response = await page.goto(route);
      expect(response?.ok(), `${route} responded with ${response?.status()}`).toBeTruthy();
      await expect(page).not.toHaveURL(/\/login/);
      expect(pageErrors, `${route} threw: ${pageErrors.map((e) => e.message).join("; ")}`).toHaveLength(0);
    });
  }
});
