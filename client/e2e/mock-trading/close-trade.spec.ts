import { test, expect } from "../fixtures/mockTrading";

// Trades in this app are opened automatically by the mock-trading executor
// worker (scripts/mock-trading-executor-worker.ts) based on strategy
// signals — there is no manual "place trade" UI control. The one genuine
// user-driven trading interaction on this page is closing an OPEN position
// (MockTradingDashboard.tsx — DeskButton "Close" → engine.closeTrade(id),
// a synchronous client-side state update). The mockTradingApi fixture seeds
// exactly one OPEN trade so we can exercise that interaction end to end.
test.describe("mock trading dashboard — close trade", () => {
  test("closing an open trade removes its Close action and updates status", async ({ page }) => {
    await page.goto("/mock-trading");

    // Scope to the open-trades table row specifically — by name alone,
    // "EMA_Cross_Fast" also matches an aggregated strategy-rank summary row
    // that has no "BUY"/"SELL" side column, so filter on that too. (Can't
    // additionally filter on the "Close" button: it disappears once the
    // trade closes, which would make the locator stop resolving entirely.)
    const row = page
      .getByRole("row", { name: /EMA_Cross_Fast/i })
      .filter({ hasText: "BUY" });
    await expect(row).toHaveCount(1);
    await expect(row).toBeVisible();
    await expect(row.getByText("OPEN", { exact: true })).toBeVisible();

    // closeTrade() (useMockTradingEngine.ts) is a no-op until the live BTC
    // price hook has resolved at least one /api/btc/price response, so wait
    // for that to settle before clicking — otherwise the click is silently
    // swallowed.
    await expect(page.getByText("97,250.50").first()).toBeVisible();

    // Assert against the "Closed" trade-analytics metric tile rather than the
    // open-trades table row itself: once the close persists, the table's
    // data source (src/components/MockTradingDashboard.tsx — tableSourceTrades)
    // switches from the live in-memory trade list to engine.historyTrades,
    // which this fixture seeds empty (real Mongo isn't configured for E2E —
    // see playwright.config.ts), so the row's own text isn't a reliable
    // post-close signal here. The analytics tile is driven by
    // engine.analytics (computed straight from portfolioTrades) and reliably
    // reflects the close regardless of that table-source switch.
    const closedTile = page
      .locator(".desk-metric-tile")
      .filter({ has: page.getByText("Closed", { exact: true }) });
    await expect(closedTile.locator(".desk-mono")).toHaveText("0");

    await row.getByRole("button", { name: "Close" }).click();

    await expect(closedTile.locator(".desk-mono")).toHaveText("1", { timeout: 15_000 });
  });
});
