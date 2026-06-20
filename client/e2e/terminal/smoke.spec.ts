import { test, expect } from "../fixtures/authedTest";

// Every /terminal/* route, authenticated, with no Go engine or MongoDB
// configured for the E2E webServer (playwright.config.ts). All engine/Mongo
// -backed API routes here already have graceful "offline"/"not configured"
// fallbacks (verified in killswitch/status, risk-ribbon, scalers/stats, and
// the mock-trading routes), so this is a true smoke test: each page must
// render its layout chrome without throwing, even with zero live data.
const TERMINAL_ROUTES = [
  "/terminal",
  "/terminal/analytics",
  "/terminal/backtest",
  "/terminal/design-system",
  "/terminal/diagnostics",
  "/terminal/events",
  "/terminal/execution",
  "/terminal/grade-1",
  "/terminal/grade-2",
  "/terminal/grade-3",
  "/terminal/grade-4",
  "/terminal/grade-5",
  "/terminal/health",
  "/terminal/journal",
  "/terminal/main-engine",
  "/terminal/mock-engine",
  "/terminal/observability",
  "/terminal/portfolio",
  "/terminal/portfolio-intelligence",
  "/terminal/research",
  "/terminal/risk",
  "/terminal/settings",
  "/terminal/strategies",
  "/terminal/trade-engine",
  "/terminal/trading",
];

test.describe("terminal smoke", () => {
  for (const route of TERMINAL_ROUTES) {
    test(`${route} loads without crashing`, async ({ page }) => {
      const pageErrors: Error[] = [];
      page.on("pageerror", (err) => pageErrors.push(err));

      const response = await page.goto(route);
      expect(response?.ok(), `${route} responded with ${response?.status()}`).toBeTruthy();
      await expect(page).toHaveURL(new RegExp(route.replace(/\//g, "\\/") + "$"));

      // Body should have rendered real content, not an empty/error shell.
      await expect(page.locator("body")).not.toBeEmpty();
      expect(pageErrors, `${route} threw: ${pageErrors.map((e) => e.message).join("; ")}`).toHaveLength(0);
    });
  }
});
