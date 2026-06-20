import { test, expect } from "../fixtures/marketData";

// /mock-trading is public (no auth, no Go engine) per PUBLIC_PATHS in
// src/middleware.ts — it's a browser-side trade simulator backed by Mongo.
// This spec runs WITHOUT the mock-trading API mocks (mockTradingApi fixture)
// on purpose: Mongo is intentionally left unconfigured for the E2E webServer
// (see playwright.config.ts), so every /api/mock-trading/* call returns its
// existing graceful 503 MONGO_NOT_CONFIGURED fallback — this proves the page
// degrades cleanly with zero open trades rather than crashing.
test.describe("mock trading dashboard — smoke", () => {
  test("loads without auth and renders core panels in an empty/offline state", async ({ page }) => {
    const pageErrors: Error[] = [];
    page.on("pageerror", (err) => pageErrors.push(err));

    await page.goto("/mock-trading");

    await expect(page.getByText("Equity, margin and exposure")).toBeVisible();
    expect(pageErrors, `Uncaught page errors: ${pageErrors.map((e) => e.message).join("; ")}`).toHaveLength(0);
  });
});
