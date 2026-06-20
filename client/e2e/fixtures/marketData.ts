import { test as base, type Page } from "@playwright/test";
import btcPrice from "./data/btc-price.json";

const EMPTY_CANDLES = { ok: true, candles: [] as unknown[] };

/**
 * Intercepts the browser-side BTC price/candle fetches (useLiveBTCPrice and
 * chart components) so specs never depend on live Coinbase/Binance network
 * calls. These are client `fetch()` calls, so Playwright's page.route can
 * catch them before they ever reach the Next.js server.
 */
async function mockMarketData(page: Page): Promise<void> {
  await page.route("**/api/btc/price", (route) =>
    route.fulfill({ json: btcPrice }),
  );
  await page.route(
    /\/api\/btc\/(spot-klines|futures-klines|trade-candles|spot-state|option-chain)(\?.*)?$/,
    (route) => route.fulfill({ json: EMPTY_CANDLES }),
  );
}

export const test = base.extend<{ marketData: void }>({
  marketData: [
    async ({ page }, use) => {
      await mockMarketData(page);
      await use();
    },
    { auto: true },
  ],
});

export { expect } from "@playwright/test";
