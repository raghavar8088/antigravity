import type { Page } from "@playwright/test";
import { test as base, expect } from "./marketData";
import openTrade from "./data/mock-open-trade.json";
import accountSnapshot from "./data/mock-account.json";
import { DEFAULT_MOCK_TRADING_CONFIG } from "../../src/lib/trading/mockTradingEngine";

/**
 * Mocks the /api/mock-trading/* endpoints the dashboard fetches on load.
 * Real Mongo is intentionally NOT configured for the E2E webServer (see
 * playwright.config.ts), so without these mocks every call returns 503
 * MONGO_NOT_CONFIGURED — fine for a pure smoke test, but not enough to
 * exercise an actual close-trade interaction. This fixture seeds one OPEN
 * trade so specs can click "Close" and assert the UI reacts.
 */
async function mockMockTradingApi(page: Page): Promise<void> {
  await page.route(/\/api\/mock-trading\/trades\?.*status=OPEN.*/, (route) =>
    route.fulfill({
      json: { ok: true, trades: [openTrade], total: 1, page: 1, limit: 200, totalPages: 1, source: "e2e-fixture" },
    }),
  );
  await page.route(/\/api\/mock-trading\/trades\?.*status=CLOSED.*/, (route) =>
    route.fulfill({
      json: { ok: true, trades: [], total: 0, page: 1, limit: 200, totalPages: 1, source: "e2e-fixture" },
    }),
  );
  // Fallback for any other /trades query (e.g. unfiltered "full" fetch).
  await page.route(/\/api\/mock-trading\/trades\?(?!.*status=).*/, (route) =>
    route.fulfill({
      json: { ok: true, trades: [openTrade], total: 1, page: 1, limit: 200, totalPages: 1, source: "e2e-fixture" },
    }),
  );
  await page.route("**/api/mock-trading/trades/*/close", (route) =>
    route.fulfill({ json: { ok: true } }),
  );
  await page.route(/\/api\/mock-trading\/account\/latest.*/, (route) =>
    route.fulfill({
      json: { ok: true, snapshot: accountSnapshot, config: DEFAULT_MOCK_TRADING_CONFIG, source: "e2e-fixture" },
    }),
  );
  await page.route(/\/api\/mock-trading\/logs.*/, (route) =>
    route.fulfill({ json: { ok: true, logs: [], total: 0, page: 1, limit: 200, totalPages: 1 } }),
  );
  await page.route(/\/api\/mock-trading\/executor-status.*/, (route) =>
    route.fulfill({
      json: { ok: true, health: "HEALTHY", lastCycleAt: Date.now(), issues: [] },
    }),
  );
  await page.route(/\/api\/mock-trading\/signal-tick.*/, (route) =>
    route.fulfill({ json: { ok: true, signals: [] } }),
  );
}

export const test = base.extend<{ mockTradingApi: void }>({
  mockTradingApi: [
    async ({ page }, use) => {
      await mockMockTradingApi(page);
      await use();
    },
    { auto: true },
  ],
});

export { expect };
