import { chromium } from "playwright";
import crypto from "node:crypto";

const JWT_SECRET = "b8b8c813baa01d5b185b6da6ba5e25a51e31b89c60a38c01dc4e7649daa4a827";
const BASE = "http://localhost:3000";
const OUT = "C:/Users/ragha/AppData/Local/Temp/claude/test-prelive";

function makeJwt() {
  const enc = (o) => Buffer.from(JSON.stringify(o)).toString("base64url");
  const now = Math.floor(Date.now() / 1000);
  const header = enc({ alg: "HS256", typ: "JWT" });
  const payload = enc({ userId: "mock_trading_main", email: "raghava", role: "ADMIN", iat: now, exp: now + 86400 });
  const body = `${header}.${payload}`;
  const sig = crypto.createHmac("sha256", JWT_SECRET).update(body).digest("base64url");
  return `${body}.${sig}`;
}

(async () => {
  const browser = await chromium.launch({ headless: true });

  // ── Desktop (1400×900) ─────────────────────────────────────────────────────
  const dCtx = await browser.newContext({
    viewport: { width: 1400, height: 900 },
    userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
  });
  await dCtx.addCookies([{ name: "raig_session", value: makeJwt(), domain: "localhost", path: "/", httpOnly: true, sameSite: "Lax" }]);
  const dp = await dCtx.newPage();

  // Mock the terminal authority endpoints so the guard passes immediately
  await dp.route("**/api/mock-trading/snapshot**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ balance: 1000000, equity: 1000000, realizedPnl: 0, unrealizedPnl: 0, openPositionCount: 0, totalTrades: 0, winRate: 0 }) }));
  await dp.route("**/api/strategy-intelligence**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ strategies: [] }) }));
  await dp.route("**/api/mock-trading/equity**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify([]) }));
  await dp.route("**/api/btc/price**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ price: 97000, source: "mock" }) }));
  // Mock pre-live engine APIs
  await dp.route("**/api/pre-live/api/positions**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: "[]" }));
  await dp.route("**/api/pre-live/api/trades**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: "[]" }));
  await dp.route("**/api/pre-live/api/stats**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ balance: 100, equity: 100, dailyPnl: 0, openPositions: 0, strategies: 100 }) }));
  await dp.route("**/api/pre-live/api/scalers/stats**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ regime: "NEUTRAL", recentSignals: [] }) }));

  console.log("→ navigating desktop...");
  await dp.goto(`${BASE}/terminal/pre-live-engine`, { waitUntil: "domcontentloaded", timeout: 30000 });
  // Wait for empty states or tables to appear after loading resolves
  await dp.waitForSelector(".google-empty-state:not(.min-h-\\[60vh\\]), .pre-live-scroll-table, .m3-kpi-strip", { timeout: 15000 }).catch(() => {});
  await dp.waitForTimeout(2000);

  // Check for scrollbar elements
  const scrollInfo = await dp.evaluate(() => {
    const containers = document.querySelectorAll(".pre-live-scroll-table");
    return Array.from(containers).map((el) => {
      const cs = window.getComputedStyle(el);
      return {
        overflowX: cs.overflowX,
        scrollWidth: el.scrollWidth,
        clientWidth: el.clientWidth,
        needsScroll: el.scrollWidth > el.clientWidth,
        paddingBottom: cs.paddingBottom,
      };
    });
  });
  console.log("SCROLL CONTAINERS:", JSON.stringify(scrollInfo, null, 2));

  // Check empty state heights
  const emptyInfo = await dp.evaluate(() => {
    const els = document.querySelectorAll(".google-empty-state, .m3-terminal-empty-state");
    return Array.from(els).map((el) => {
      const cs = window.getComputedStyle(el);
      return { class: el.className, computedHeight: Math.round(el.getBoundingClientRect().height), minHeight: cs.minHeight, padding: cs.padding };
    });
  });
  console.log("EMPTY STATES:", JSON.stringify(emptyInfo, null, 2));

  // Check scroll hint text
  const hintCount = await dp.evaluate(() => {
    const hints = document.body.innerText.match(/swipe to scroll/gi);
    return hints ? hints.length : 0;
  });
  console.log(`SCROLL HINTS found: ${hintCount}`);

  await dp.screenshot({ path: `${OUT}-desktop.png`, fullPage: false });
  console.log(`→ desktop screenshot saved`);

  // ── Mobile (390×844) ────────────────────────────────────────────────────────
  const mCtx = await browser.newContext({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15",
  });
  await mCtx.addCookies([{ name: "raig_session", value: makeJwt(), domain: "localhost", path: "/", httpOnly: true, sameSite: "Lax" }]);
  const mp = await mCtx.newPage();
  await mp.route("**/api/mock-trading/snapshot**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ balance: 1000000, equity: 1000000, realizedPnl: 0, unrealizedPnl: 0, openPositionCount: 0, totalTrades: 0, winRate: 0 }) }));
  await mp.route("**/api/strategy-intelligence**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ strategies: [] }) }));
  await mp.route("**/api/mock-trading/equity**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify([]) }));
  await mp.route("**/api/btc/price**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ price: 97000, source: "mock" }) }));
  await mp.route("**/api/pre-live/api/positions**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: "[]" }));
  await mp.route("**/api/pre-live/api/trades**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: "[]" }));
  await mp.route("**/api/pre-live/api/stats**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ balance: 100, equity: 100, dailyPnl: 0, openPositions: 0, strategies: 100 }) }));
  await mp.route("**/api/pre-live/api/scalers/stats**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ regime: "NEUTRAL", recentSignals: [] }) }));
  await mp.goto(`${BASE}/terminal/pre-live-engine`, { waitUntil: "domcontentloaded", timeout: 30000 });
  await mp.waitForSelector(".m3-kpi-strip, .google-empty-state:not(.min-h-\\[60vh\\])", { timeout: 15000 }).catch(() => {});
  await mp.waitForTimeout(2000);
  await mp.screenshot({ path: `${OUT}-mobile.png`, fullPage: true });
  console.log("→ mobile screenshot saved");

  await browser.close();
  console.log("DONE");
})();
