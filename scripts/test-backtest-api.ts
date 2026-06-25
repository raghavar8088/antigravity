#!/usr/bin/env tsx
/**
 * Backtest API smoke test — hits the live engine directly.
 * Usage:  npx tsx scripts/test-backtest-api.ts
 * Env:    ENGINE_URL=http://13.233.8.80  (default)
 */

const ENGINE = process.env.ENGINE_URL ?? "http://13.233.8.80";

// ── helpers ────────────────────────────────────────────────────────────────────

let passed = 0;
let failed = 0;

function ok(label: string, value: unknown) {
  if (value) {
    console.log(`  ✓  ${label}`);
    passed++;
  } else {
    console.error(`  ✗  ${label}`);
    failed++;
  }
}

async function get(path: string) {
  const r = await fetch(`${ENGINE}${path}`, { signal: AbortSignal.timeout(15_000) });
  const text = await r.text();
  try { return { status: r.status, body: JSON.parse(text) }; }
  catch { return { status: r.status, body: text }; }
}

async function post(path: string, payload: unknown) {
  const r = await fetch(`${ENGINE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    signal: AbortSignal.timeout(15_000),
  });
  const text = await r.text();
  try { return { status: r.status, body: JSON.parse(text) }; }
  catch { return { status: r.status, body: text }; }
}

function sleep(ms: number) { return new Promise(r => setTimeout(r, ms)); }

// ── test suites ────────────────────────────────────────────────────────────────

async function testHealth() {
  console.log("\n── Health ──────────────────────────────────────────────");
  const { status, body } = await get("/health");
  ok("GET /health → 200", status === 200);
  ok("status=alive", (body as any)?.status === "alive");
  ok("strategies count > 0", (body as any)?.strategies > 0);
}

async function testStrategies() {
  console.log("\n── GET /api/backtest/strategies ────────────────────────");
  const { status, body } = await get("/api/backtest/strategies");
  ok("200 OK", status === 200);
  ok("returns array", Array.isArray(body));
  const arr = body as any[];
  ok("at least 1 strategy", arr.length >= 1);
  const first = arr[0];
  ok("has .name", typeof first?.name === "string");
  ok("has .regimes array", Array.isArray(first?.regimes));
  ok("has .timeframes array", Array.isArray(first?.timeframes));
  console.log(`     strategies returned: ${arr.length}`);
}

async function testJobList() {
  console.log("\n── GET /api/backtest/jobs ──────────────────────────────");
  const { status, body } = await get("/api/backtest/jobs");
  ok("200 OK", status === 200);
  ok("returns array", Array.isArray(body));
}

async function testLeaderboard() {
  console.log("\n── GET /api/backtest/leaderboard ───────────────────────");
  const { status, body } = await get("/api/backtest/leaderboard?symbol=BTCUSDT");
  ok("200 OK", status === 200);
  ok("returns array", Array.isArray(body));
  const arr = body as any[];
  if (arr.length > 0) {
    const row = arr[0];
    ok("row has strategyName", typeof row?.strategyName === "string");
    ok("row has totalTrades (number)", typeof row?.totalTrades === "number");
    ok("row has sharpe (number)", typeof row?.sharpe === "number");
    ok("row has winRate 0–1", row?.winRate >= 0 && row?.winRate <= 1);
    console.log(`     leaderboard rows: ${arr.length}, top: ${row.strategyName} sharpe=${row.sharpe?.toFixed(2)}`);
  } else {
    console.log("     leaderboard empty (no completed runs yet)");
  }
}

async function testRunAndPoll(): Promise<string | null> {
  console.log("\n── POST /api/backtest/run + poll status ────────────────");

  // Submit a short 30-day backtest with 1 strategy to keep it fast
  const { status: s1, body: b1 } = await post("/api/backtest/run", {
    symbol: "BTCUSDT",
    from:   "2024-11-01",
    to:     "2024-11-30",
    strategies: [],   // empty = all ported strategies, same as UI
  });
  ok("POST /run → 202", s1 === 202);
  const jobId = (b1 as any)?.id;
  const runId = (b1 as any)?.runId;
  ok("response has id", typeof jobId === "string" && jobId.startsWith("job_"));
  ok("response has runId", typeof runId === "string" && runId.startsWith("run_"));
  ok("status=pending or running", ["pending","running"].includes((b1 as any)?.status));
  if (!jobId) return null;
  console.log(`     job=${jobId}  run=${runId}`);

  // Poll /status until done or timeout (120 s)
  console.log("\n── GET /api/backtest/status/:id (polling) ──────────────");
  const deadline = Date.now() + 120_000;
  let finalStatus = "";
  let dots = 0;
  while (Date.now() < deadline) {
    const { status, body } = await get(`/api/backtest/status/${jobId}`);
    ok("status endpoint 200", status === 200);
    const st = (body as any)?.status;
    const prog = (body as any)?.progress ?? 0;
    if (st === "done" || st === "error") {
      finalStatus = st;
      console.log(`\n     final: status=${st}  progress=${prog}%`);
      if (st === "error") console.error(`     error: ${(body as any)?.error}`);
      break;
    }
    process.stdout.write(dots++ % 10 === 0 ? `\r     running… ${prog}% ` : ".");
    await sleep(3_000);
  }
  ok("job completed within 120s", finalStatus === "done");
  if (finalStatus !== "done") return null;
  return runId;
}

async function testResultsForRun(runId: string) {
  console.log("\n── GET /api/backtest/results/:runId ────────────────────");
  const { status, body } = await get(`/api/backtest/results/${runId}?symbol=BTCUSDT`);
  ok("200 OK", status === 200);
  ok("returns array", Array.isArray(body));
  const arr = body as any[];
  if (arr.length > 0) {
    console.log(`     results count: ${arr.length}`);
    ok("row has strategyName", typeof arr[0]?.strategyName === "string");
  }
}

async function testLeaderboardAfterRun(runId: string) {
  console.log("\n── GET /api/backtest/leaderboard?run_id=... ────────────");
  const { status, body } = await get(
    `/api/backtest/leaderboard?run_id=${encodeURIComponent(runId)}&symbol=BTCUSDT`
  );
  ok("200 OK", status === 200);
  ok("returns array", Array.isArray(body));
  const arr = body as any[];
  console.log(`     leaderboard rows for run: ${arr.length}`);
}

async function testBadRequests() {
  console.log("\n── Error handling ──────────────────────────────────────");
  const { status: s1 } = await post("/api/backtest/run", { symbol: "BTCUSDT", from: "bad-date" });
  ok("bad from date → 400", s1 === 400);

  const { status: s2 } = await get("/api/backtest/status/nonexistent-id-xyz");
  ok("unknown job id → 404", s2 === 404);
}

// ── main ───────────────────────────────────────────────────────────────────────

async function main() {
  console.log(`\nBacktest API Test Suite`);
  console.log(`Engine: ${ENGINE}`);
  console.log(`Started: ${new Date().toISOString()}`);

  try {
    await testHealth();
    await testStrategies();
    await testJobList();
    await testLeaderboard();
    await testBadRequests();

    // Full end-to-end run — may take up to 2 minutes
    const runId = await testRunAndPoll();
    if (runId) {
      await testResultsForRun(runId);
      await testLeaderboardAfterRun(runId);
    }
  } catch (err) {
    console.error("\nUnhandled error:", err);
    failed++;
  }

  console.log(`\n${"─".repeat(55)}`);
  console.log(`Results: ${passed} passed, ${failed} failed`);
  if (failed > 0) process.exit(1);
}

main();
