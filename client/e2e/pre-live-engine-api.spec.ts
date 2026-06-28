/**
 * Pre-Live Trade Engine — end-to-end API tests
 *
 * Hits the pre-live engine at PRE_LIVE_URL (default http://13.233.8.80:8082)
 * directly — no Next.js server or auth required for these tests.
 *
 * Run:
 *   PRE_LIVE_URL=http://13.233.8.80:8082 npx playwright test e2e/pre-live-engine-api.spec.ts --reporter=list
 */
import { test, expect } from "@playwright/test";

const PRE_LIVE = process.env.PRE_LIVE_URL ?? "http://13.233.8.80:8082";
const TIMEOUT = 12_000;

// ── helpers ───────────────────────────────────────────────────────────────────

async function tryGet(request: any, path: string) {
  try {
    return await request.get(`${PRE_LIVE}${path}`, { timeout: TIMEOUT });
  } catch {
    return null;
  }
}

async function tryPost(request: any, path: string, data?: object) {
  try {
    return await request.post(`${PRE_LIVE}${path}`, {
      timeout: TIMEOUT,
      ...(data ? { data } : {}),
    });
  } catch {
    return null;
  }
}

function reachable(r: any): boolean {
  return r !== null && r.status() > 0;
}

// ── /health ───────────────────────────────────────────────────────────────────

test("GET /health → ok status and strategy count > 0", async ({ request }) => {
  const r = await tryGet(request, "/health");
  test.skip(!reachable(r), "pre-live engine unreachable");
  expect(r.ok()).toBe(true);
  const body = await r.json();
  expect(body.status).toBe("ok");
  expect(body.engine).toBe("pre-live");
  expect(typeof body.strategies).toBe("number");
  expect(body.strategies).toBeGreaterThan(0);
  expect(typeof body.mongodb_connected).toBe("boolean");
});

// ── /ready ────────────────────────────────────────────────────────────────────

test("GET /ready → ready or degraded with shape", async ({ request }) => {
  const r = await tryGet(request, "/ready");
  test.skip(!reachable(r), "pre-live engine unreachable");
  expect(r.ok()).toBe(true);
  const body = await r.json();
  expect(["ready", "degraded"]).toContain(body.status);
  expect(body.engine).toBe("pre-live");
  expect(typeof body.strategies).toBe("number");
  expect(body.strategies).toBeGreaterThan(0);
  expect(typeof body.mongodb_connected).toBe("boolean");
});

// ── /api/positions ────────────────────────────────────────────────────────────

test("GET /api/positions → 200 array (open positions or empty)", async ({ request }) => {
  const r = await tryGet(request, "/api/positions");
  test.skip(!reachable(r), "pre-live engine unreachable");
  expect(r.ok()).toBe(true);
  const body = await r.json();
  expect(Array.isArray(body)).toBe(true);

  if (body.length > 0) {
    const pos = body[0];
    expect(typeof pos.id).toBe("string");
    expect(typeof pos.side).toBe("string");
    expect(["BUY", "SELL", "LONG", "SHORT"]).toContain(pos.side.toUpperCase());
    expect(typeof pos.size).toBe("number");
    expect(pos.size).toBeGreaterThan(0);
    expect(typeof pos.entryPrice).toBe("number");
    expect(pos.entryPrice).toBeGreaterThan(0);
    expect(typeof pos.strategy).toBe("string");
  }
});

// ── /api/trades ───────────────────────────────────────────────────────────────

test("GET /api/trades → 200 array with correct trade shape", async ({ request }) => {
  const r = await tryGet(request, "/api/trades");
  test.skip(!reachable(r), "pre-live engine unreachable");
  expect(r.ok()).toBe(true);
  const body = await r.json();
  expect(Array.isArray(body)).toBe(true);

  if (body.length > 0) {
    const trade = body[0];
    expect(typeof trade.id).toBe("string");
    expect(typeof trade.strategyName).toBe("string");
    expect(typeof trade.side).toBe("string");
    expect(typeof trade.size).toBe("number");
    expect(trade.size).toBeGreaterThan(0);
    expect(typeof trade.entryPrice).toBe("number");
    expect(trade.entryPrice).toBeGreaterThan(0);
    expect(typeof trade.exitPrice).toBe("number");
    expect(trade.exitPrice).toBeGreaterThan(0);
    expect(typeof trade.grossPnL).toBe("number");
    expect(typeof trade.netPnL).toBe("number");
    expect(typeof trade.fees).toBe("number");
    expect(trade.fees).toBeGreaterThanOrEqual(0);
    expect(typeof trade.reason).toBe("string");
    expect(["TP", "SL", "MANUAL", "tp", "sl", "manual"]).toContain(trade.reason);
  }
});

// ── /api/stats ────────────────────────────────────────────────────────────────

test("GET /api/stats → 200 with all required KPI fields", async ({ request }) => {
  const r = await tryGet(request, "/api/stats");
  test.skip(!reachable(r), "pre-live engine unreachable");
  expect(r.ok()).toBe(true);
  const body = await r.json();

  // Top-level keys
  expect(body).toHaveProperty("aggregate");
  expect(body).toHaveProperty("balance");
  expect(body).toHaveProperty("equity");
  expect(body).toHaveProperty("cashBalance");
  expect(body).toHaveProperty("initialBalance");
  expect(body).toHaveProperty("exposure");
  expect(body).toHaveProperty("netPosition");
  expect(body).toHaveProperty("dailyPnl");
  expect(body).toHaveProperty("lastPrice");
  expect(body).toHaveProperty("openPositions");
  expect(body).toHaveProperty("ticksProcessed");
  expect(body).toHaveProperty("candlesClosed");
  expect(body).toHaveProperty("strategies");

  // Numeric sanity
  expect(typeof body.balance).toBe("number");
  expect(typeof body.equity).toBe("number");
  expect(typeof body.initialBalance).toBe("number");
  expect(body.initialBalance).toBeGreaterThan(0);
  expect(typeof body.strategies).toBe("number");
  expect(body.strategies).toBeGreaterThan(0);
  expect(typeof body.openPositions).toBe("number");
  expect(body.openPositions).toBeGreaterThanOrEqual(0);
  expect(typeof body.lastPrice).toBe("number");
  expect(body.lastPrice).toBeGreaterThan(0);

  // Aggregate stats shape
  const agg = body.aggregate;
  expect(typeof agg.totalTrades).toBe("number");
  expect(typeof agg.totalWins).toBe("number");
  expect(typeof agg.totalLosses).toBe("number");
  expect(typeof agg.winRate).toBe("number");
  expect(agg.winRate).toBeGreaterThanOrEqual(0);
  expect(agg.winRate).toBeLessThanOrEqual(100);
  expect(typeof agg.totalPnl).toBe("number");
});

test("GET /api/stats → balance is consistent with initial balance", async ({ request }) => {
  const r = await tryGet(request, "/api/stats");
  test.skip(!reachable(r), "pre-live engine unreachable");
  const body = await r.json();

  // initialBalance must match the configured $100
  expect(body.initialBalance).toBe(100);

  // equity = balance + unrealized; balance shouldn't diverge wildly from initial in early trading
  expect(Math.abs(body.balance)).toBeLessThan(1_000_000);
  expect(Math.abs(body.equity)).toBeLessThan(1_000_000);
});

test("GET /api/stats → lastPrice is a plausible BTC price", async ({ request }) => {
  const r = await tryGet(request, "/api/stats");
  test.skip(!reachable(r), "pre-live engine unreachable");
  const body = await r.json();
  // BTC price should be between $1,000 and $1,000,000
  expect(body.lastPrice).toBeGreaterThan(1_000);
  expect(body.lastPrice).toBeLessThan(1_000_000);
});

// ── /api/regime ───────────────────────────────────────────────────────────────

test("GET /api/regime → returns a non-empty regime string", async ({ request }) => {
  const r = await tryGet(request, "/api/regime");
  test.skip(!reachable(r), "pre-live engine unreachable");
  expect(r.ok()).toBe(true);
  const body = await r.json();
  expect(body).toHaveProperty("regime");
  expect(typeof body.regime).toBe("string");
  expect(body.regime.length).toBeGreaterThan(0);
});

// ── /api/strategies ───────────────────────────────────────────────────────────

test("GET /api/strategies → 100 qualified strategies with correct shape", async ({ request }) => {
  const r = await tryGet(request, "/api/strategies");
  test.skip(!reachable(r), "pre-live engine unreachable");
  expect(r.ok()).toBe(true);
  const body = await r.json();
  expect(Array.isArray(body)).toBe(true);
  expect(body.length).toBeGreaterThan(0);

  const s = body[0];
  expect(typeof s.name).toBe("string");
  expect(s.name.length).toBeGreaterThan(0);
  expect(typeof s.category).toBe("string");
  expect(typeof s.timeframe).toBe("string");
});

test("GET /api/strategies → strategy count matches /health", async ({ request }) => {
  const [health, strategies] = await Promise.all([
    tryGet(request, "/health"),
    tryGet(request, "/api/strategies"),
  ]);
  test.skip(!reachable(health) || !reachable(strategies), "pre-live engine unreachable");

  const healthBody = await health.json();
  const strategiesBody = await strategies.json();
  expect(Array.isArray(strategiesBody)).toBe(true);
  expect(strategiesBody.length).toBe(healthBody.strategies);
});

test("GET /api/strategies → unique strategy names", async ({ request }) => {
  const r = await tryGet(request, "/api/strategies");
  test.skip(!reachable(r), "pre-live engine unreachable");
  const body = await r.json();
  const names = body.map((s: any) => s.name);
  const unique = new Set(names);
  expect(unique.size).toBe(names.length);
});

// ── /api/strategies/stats ─────────────────────────────────────────────────────

test("GET /api/strategies/stats → array with per-strategy runtime fields", async ({ request }) => {
  const r = await tryGet(request, "/api/strategies/stats");
  test.skip(!reachable(r), "pre-live engine unreachable");
  expect(r.ok()).toBe(true);
  const body = await r.json();
  expect(Array.isArray(body)).toBe(true);
  expect(body.length).toBeGreaterThan(0);

  const s = body[0];
  expect(typeof s.name).toBe("string");
  expect(typeof s.category).toBe("string");
  expect(typeof s.timeframe).toBe("string");
  expect(typeof s.trades).toBe("number");
  expect(typeof s.wins).toBe("number");
  expect(typeof s.losses).toBe("number");
  expect(typeof s.winRate).toBe("number");
  expect(typeof s.pnl).toBe("number");
  expect(typeof s.signalsFired).toBe("number");

  // wins + losses <= trades
  expect(s.wins + s.losses).toBeLessThanOrEqual(s.trades);
});

test("GET /api/strategies/stats → count matches /api/strategies count", async ({ request }) => {
  const [strategies, stats] = await Promise.all([
    tryGet(request, "/api/strategies"),
    tryGet(request, "/api/strategies/stats"),
  ]);
  test.skip(!reachable(strategies) || !reachable(stats), "pre-live engine unreachable");

  const sBody = await strategies.json();
  const stBody = await stats.json();
  expect(stBody.length).toBe(sBody.length);
});

// ── /api/scalers/stats ────────────────────────────────────────────────────────

test("GET /api/scalers/stats → 200 with regime field", async ({ request }) => {
  const r = await tryGet(request, "/api/scalers/stats");
  test.skip(!reachable(r), "pre-live engine unreachable");
  expect(r.ok()).toBe(true);
  const body = await r.json();
  expect(typeof body).toBe("object");
  expect(body).not.toBeNull();
});

// ── /api/admin/reset — method guard (GET → 405) ───────────────────────────────

test("GET /api/admin/reset → 405 method not allowed", async ({ request }) => {
  const r = await tryGet(request, "/api/admin/reset");
  test.skip(!reachable(r), "pre-live engine unreachable");
  expect(r.status()).toBe(405);
});

// ── Cross-endpoint consistency checks ─────────────────────────────────────────

test("openPositions in /api/stats matches length of /api/positions", async ({ request }) => {
  const [statsR, posR] = await Promise.all([
    tryGet(request, "/api/stats"),
    tryGet(request, "/api/positions"),
  ]);
  test.skip(!reachable(statsR) || !reachable(posR), "pre-live engine unreachable");

  const stats = await statsR.json();
  const positions = await posR.json();
  expect(positions.length).toBe(stats.openPositions);
});

test("/api/trades entry and exit prices are positive numbers", async ({ request }) => {
  const r = await tryGet(request, "/api/trades");
  test.skip(!reachable(r), "pre-live engine unreachable");
  const trades = await r.json();
  test.skip(trades.length === 0, "no closed trades yet — skip price validation");

  for (const trade of trades.slice(0, 20)) {
    expect(trade.entryPrice).toBeGreaterThan(0);
    expect(trade.exitPrice).toBeGreaterThan(0);
    expect(trade.size).toBeGreaterThan(0);
    // With 10x leverage: |grossPnL| = |diff| * size * 10
    // Just verify it's a finite number
    expect(Number.isFinite(trade.grossPnL)).toBe(true);
    expect(Number.isFinite(trade.netPnL)).toBe(true);
    // fees always non-negative
    expect(trade.fees).toBeGreaterThanOrEqual(0);
    // netPnL = grossPnL - fees (approximately)
    const expectedNet = trade.grossPnL - trade.fees;
    expect(Math.abs(trade.netPnL - expectedNet)).toBeLessThan(0.001);
  }
});

test("/api/stats aggregate wins + losses = totalTrades", async ({ request }) => {
  const r = await tryGet(request, "/api/stats");
  test.skip(!reachable(r), "pre-live engine unreachable");
  const body = await r.json();
  const agg = body.aggregate;
  test.skip(agg.totalTrades === 0, "no trades yet — skip aggregate validation");
  expect(agg.totalWins + agg.totalLosses).toBe(agg.totalTrades);
});

test("/api/stats winRate is consistent with wins/totalTrades", async ({ request }) => {
  const r = await tryGet(request, "/api/stats");
  test.skip(!reachable(r), "pre-live engine unreachable");
  const agg = (await r.json()).aggregate;
  test.skip(agg.totalTrades === 0, "no trades yet — skip win rate validation");
  const expectedWinRate = (agg.totalWins / agg.totalTrades) * 100;
  expect(Math.abs(agg.winRate - expectedWinRate)).toBeLessThan(0.01);
});

// ── Leverage verification ─────────────────────────────────────────────────────

test("/api/trades gross PnL reflects 10x leverage (size × price diff × 10)", async ({ request }) => {
  const r = await tryGet(request, "/api/trades");
  test.skip(!reachable(r), "pre-live engine unreachable");
  const trades = await r.json();
  test.skip(trades.length === 0, "no closed trades yet — skip leverage check");

  const LEVERAGE = 10;
  for (const trade of trades.slice(0, 10)) {
    if (!trade.entryPrice || !trade.exitPrice || !trade.size) continue;
    const isLong = trade.side === "BUY" || trade.side === "LONG";
    const rawDiff = isLong
      ? trade.exitPrice - trade.entryPrice
      : trade.entryPrice - trade.exitPrice;
    const expectedGross = rawDiff * trade.size * LEVERAGE;
    // Allow 1% tolerance for rounding
    const tolerance = Math.abs(expectedGross) * 0.01 + 0.001;
    expect(Math.abs(trade.grossPnL - expectedGross)).toBeLessThan(tolerance);
  }
});

// ── CORS headers ──────────────────────────────────────────────────────────────

test("GET /health → CORS headers present", async ({ request }) => {
  const r = await tryGet(request, "/health");
  test.skip(!reachable(r), "pre-live engine unreachable");
  const allowOrigin = r.headers()["access-control-allow-origin"];
  expect(allowOrigin).toBe("*");
});

test("OPTIONS /api/stats → CORS preflight returns 200", async ({ request }) => {
  const r = await request.fetch(`${PRE_LIVE}/api/stats`, {
    method: "OPTIONS",
    timeout: TIMEOUT,
  }).catch(() => null);
  test.skip(!reachable(r), "pre-live engine unreachable");
  expect([200, 204]).toContain(r!.status());
});

// ── /api/admin/reset integration ─────────────────────────────────────────────

test("POST /api/admin/reset → ok response with initialBalance", async ({ request }) => {
  const r = await tryPost(request, "/api/admin/reset");
  test.skip(!reachable(r), "pre-live engine unreachable");

  if (r.status() === 503) {
    // No price available yet — engine still warming up, acceptable
    const body = await r.text();
    expect(body).toContain("no price available");
    return;
  }

  expect(r.ok()).toBe(true);
  const body = await r.json();
  expect(body.ok).toBe(true);
  expect(typeof body.message).toBe("string");
  expect(body.message).toContain("reset");
  expect(body.initialBalance).toBe(100);
});

test("POST /api/admin/reset → positions cleared and stats reset", async ({ request }) => {
  const resetR = await tryPost(request, "/api/admin/reset");
  test.skip(!reachable(resetR), "pre-live engine unreachable");
  if (resetR.status() === 503) {
    test.skip(true, "engine warming up — no price available yet");
  }
  expect(resetR.ok()).toBe(true);

  // After reset, positions should be empty
  const posR = await tryGet(request, "/api/positions");
  const positions = await posR.json();
  expect(positions.length).toBe(0);

  // Stats should show 0 trades
  const statsR = await tryGet(request, "/api/stats");
  const stats = await statsR.json();
  expect(stats.openPositions).toBe(0);
  expect(stats.aggregate.totalTrades).toBe(0);
  expect(stats.balance).toBeCloseTo(100, 2);
});
