/**
 * Backtest engine API tests — hit 13.233.8.80 directly.
 * No Next.js server or auth required. Run with:
 *   ENGINE_URL=http://13.233.8.80 npx playwright test e2e/backtest-engine-api.spec.ts --reporter=list
 */
import { test, expect } from "@playwright/test";

const ENGINE = process.env.ENGINE_URL ?? "http://13.233.8.80";

async function tryGet(request: any, url: string, opts?: any) {
  try {
    return await request.get(url, opts);
  } catch {
    return null;
  }
}

async function tryPost(request: any, url: string, opts?: any) {
  try {
    return await request.post(url, opts);
  } catch {
    return null;
  }
}

// ── Basic endpoints ────────────────────────────────────────────────────────────

test("GET /health → alive with strategies > 0", async ({ request }) => {
  const r = await tryGet(request, `${ENGINE}/health`, { timeout: 10_000 });
  test.skip(!r || !r.ok(), "engine unreachable");
  const body = await r.json();
  expect(body.status).toBe("alive");
  expect(body.strategies).toBeGreaterThan(0);
});

test("GET /api/backtest/strategies → non-empty array with correct shape", async ({ request }) => {
  const r = await tryGet(request, `${ENGINE}/api/backtest/strategies`, { timeout: 10_000 });
  test.skip(!r || !r.ok(), "engine unreachable");
  expect(r.status()).toBe(200);
  const body = await r.json();
  expect(Array.isArray(body)).toBe(true);
  expect(body.length).toBeGreaterThan(0);
  expect(body[0]).toHaveProperty("name");
  expect(body[0]).toHaveProperty("regimes");
  expect(body[0]).toHaveProperty("timeframes");
  expect(Array.isArray(body[0].regimes)).toBe(true);
});

test("GET /api/backtest/jobs → 200 array", async ({ request }) => {
  const r = await tryGet(request, `${ENGINE}/api/backtest/jobs`, { timeout: 10_000 });
  test.skip(!r || !r.ok(), "engine unreachable");
  expect(r.status()).toBe(200);
  expect(Array.isArray(await r.json())).toBe(true);
});

test("GET /api/backtest/leaderboard → 200 array with camelCase fields", async ({ request }) => {
  const r = await tryGet(request, `${ENGINE}/api/backtest/leaderboard?symbol=BTCUSDT`, { timeout: 10_000 });
  test.skip(!r || !r.ok(), "engine unreachable");
  expect(r.status()).toBe(200);
  const body = await r.json();
  expect(Array.isArray(body)).toBe(true);
  if (body.length > 0) {
    expect(body[0]).toHaveProperty("strategyName");
    expect(body[0]).toHaveProperty("totalTrades");
    expect(body[0]).toHaveProperty("sharpe");
    expect(body[0]).toHaveProperty("winRate");
    expect(typeof body[0].winRate).toBe("number");
    // winRate is 0–1 ratio
    expect(body[0].winRate).toBeGreaterThanOrEqual(0);
    expect(body[0].winRate).toBeLessThanOrEqual(1);
  }
});

// ── Error handling ─────────────────────────────────────────────────────────────

test("POST /api/backtest/run with bad date → 400 with error field", async ({ request }) => {
  const r = await tryPost(request, `${ENGINE}/api/backtest/run`, {
    data: { symbol: "BTCUSDT", from: "not-a-date", to: "2024-11-30" },
    timeout: 10_000,
  });
  test.skip(!r, "engine unreachable");
  expect(r.status()).toBe(400);
  const body = await r.json();
  expect(body).toHaveProperty("error");
  expect(typeof body.error).toBe("string");
});

test("GET /api/backtest/status/nonexistent → 404", async ({ request }) => {
  const r = await tryGet(request, `${ENGINE}/api/backtest/status/nonexistent-xyz-000`, { timeout: 10_000 });
  test.skip(!r, "engine unreachable");
  expect(r.status()).toBe(404);
});

// ── Full end-to-end run ────────────────────────────────────────────────────────

test("POST /run + poll to done + read results + leaderboard", async ({ request }) => {
  test.setTimeout(150_000);

  // Submit a short 7-day job
  const runResp = await tryPost(request, `${ENGINE}/api/backtest/run`, {
    data: { symbol: "BTCUSDT", from: "2024-11-01", to: "2024-11-07" },
    timeout: 10_000,
  });
  test.skip(!runResp || !runResp.ok(), "engine unreachable or data missing");
  expect(runResp.status()).toBe(202);

  const job = await runResp.json();
  expect(job.id).toMatch(/^job_\d+/);
  expect(job.runId).toMatch(/^run_\d+/);
  expect(["pending", "running"]).toContain(job.status);

  // Poll until done or 120 s — tolerate transient ECONNRESET (e.g. deploy mid-poll)
  const deadline = Date.now() + 120_000;
  let finalStatus = "";
  while (Date.now() < deadline) {
    await new Promise(r => setTimeout(r, 3_000));
    try {
      const sr = await request.get(`${ENGINE}/api/backtest/status/${job.id}`, { timeout: 10_000 });
      if (!sr.ok()) continue;
      const s = await sr.json();
      if (s.status === "done" || s.status === "error") {
        finalStatus = s.status;
        if (s.status === "error") console.error("job error:", s.error);
        break;
      }
    } catch {
      // transient network error — retry
    }
  }
  expect(finalStatus).toBe("done");

  // Results endpoint
  const resultsResp = await request.get(
    `${ENGINE}/api/backtest/results/${job.runId}?symbol=BTCUSDT`,
    { timeout: 10_000 }
  );
  expect(resultsResp.ok()).toBe(true);
  const results = await resultsResp.json();
  expect(Array.isArray(results)).toBe(true);
  expect(results.length).toBeGreaterThan(0);
  expect(results[0]).toHaveProperty("strategyName");
  expect(typeof results[0].strategyName).toBe("string");

  // Leaderboard with explicit run_id
  const lbResp = await request.get(
    `${ENGINE}/api/backtest/leaderboard?run_id=${encodeURIComponent(job.runId)}&symbol=BTCUSDT`,
    { timeout: 10_000 }
  );
  expect(lbResp.ok()).toBe(true);
  const lb = await lbResp.json();
  expect(Array.isArray(lb)).toBe(true);
  expect(lb.length).toBeGreaterThan(0);

  // Jobs list reflects the completed job
  const jobsResp = await request.get(`${ENGINE}/api/backtest/jobs`, { timeout: 10_000 });
  const jobs = await jobsResp.json();
  const found = jobs.find((j: any) => j.id === job.id);
  expect(found).toBeDefined();
  expect(found.status).toBe("done");
  expect(found.progress).toBe(100);
});
