#!/usr/bin/env tsx
/**
 * replay-walkforward.ts
 *
 * Runs a Replay + Walk-Forward analysis on the stored fixture candles and
 * prints per-strategy recommendations to stdout.
 *
 * Usage:
 *   npm run replay:walkforward
 *   npm run replay:walkforward -- --days=30 --ids=91,92,95,96
 *   npm run replay:walkforward -- --days=60 --threshold=26 --slip=5
 *
 * Hard constraints:
 *   - Paper/research only. No live Delta order placement.
 *   - No threshold lowering below 26 by default.
 *   - Recommendations only — operator decides whether to apply.
 *   - Refuses to run if candle coverage is < 80% of the requested window.
 */

import { loadEnvLocal, loadReplayCandlesForDays, parseArg } from "./replayCliShared";
import { runPaperDeskReplay, summarizeReplayTrades } from "../src/lib/futuresReplayEngine";
import { rankReplayStrategies } from "../src/lib/replayWalkForwardRanker";
import { eventFromReplayWalkForwardRun } from "../src/lib/verificationTrack/buildVerificationEvents";
import { insertVerificationEvents } from "../src/lib/verificationTrack/verificationTrackMongo";
import { isMongoConfigured } from "../src/lib/mongoTradesClient";

loadEnvLocal();

// ─── Parse args ───────────────────────────────────────────────────────────────

const days = Math.min(90, Math.max(1, Number(parseArg("days", "30"))));
const idsArg = parseArg("ids", "");
const strategyIds: number[] | undefined = idsArg
  ? idsArg.split(",").map(Number).filter((n) => Number.isFinite(n) && n > 0)
  : undefined;

const threshold = Number(parseArg("threshold", "26"));
const slippageBps = Number(parseArg("slip", "5"));

console.log(
  `[replay:walkforward] days=${days}  threshold=${threshold}  slip=${slippageBps}bps  ids=${strategyIds?.join(",") ?? "all"}`,
);

// ─── Load candles (with coverage guard) ──────────────────────────────────────

const { candles, fundingRate, fixturePath, coverageDays, sufficient, fetchCommand } =
  loadReplayCandlesForDays(days);

console.log(
  `[replay:walkforward] Loaded ${candles.length.toLocaleString()} candles from ${fixturePath || "(none)"}`,
);
console.log(
  `[replay:walkforward] Coverage: ${coverageDays.toFixed(2)} days of ${days} requested`,
);

if (!sufficient) {
  console.error(
    `\n[replay:walkforward] ✗ Insufficient replay data: ${candles.length} candles = ` +
    `${coverageDays.toFixed(1)}d coverage (need ≥80% of ${days}d = ${Math.ceil(days * 1440 * 0.8).toLocaleString()} candles).`,
  );
  console.error(
    `[replay:walkforward] Run first: ${fetchCommand}`,
  );
  console.error(
    `[replay:walkforward] Refusing to produce fake ${days}d rankings from ${coverageDays.toFixed(1)}d of data.`,
  );
  process.exit(1);
}

if (candles.length < 100) {
  console.error(`[replay:walkforward] Too few candles (${candles.length}). Run: ${fetchCommand}`);
  process.exit(1);
}

// ─── Run replay ───────────────────────────────────────────────────────────────

const result = runPaperDeskReplay(candles, {
  initialBalance: 1000,
  leverage: 25,
  slippageBps,
  maxPositions: 60,
  barMs: 60_000,
  symbol: "BTCUSD",
  signalThreshold: threshold,
  minExpectedMoveSafetyK: 1.1,
  maxSameDirFracOfEquity: 0.35,
  minTpSlRatio: 2,
  fundingRate,
  strategyIds,
});

const summary = summarizeReplayTrades(result.trades);
console.log(
  `[replay:walkforward] Replay done: ${summary.count} trades  ` +
  `sumNet=$${summary.sumNet.toFixed(2)}  expectancy=$${summary.expectancy.toFixed(2)}/trade  ` +
  `finalBalance=$${result.finalBalance.toFixed(2)}`,
);

// ─── Rank strategies ──────────────────────────────────────────────────────────

const rankings = rankReplayStrategies(result.trades);

console.log("\n[replay:walkforward] Strategy Rankings:\n");
console.log(
  "  " +
    ["ID", "Name", "Trades", "Expectancy", "Win%", "Fee%", "WF", "WFE", "Rec"]
      .map((h) => h.padEnd(14))
      .join(""),
);
console.log("  " + "-".repeat(14 * 9));

for (const r of rankings) {
  console.log(
    "  " +
      [
        String(r.strategyId).padEnd(14),
        r.strategyName.slice(0, 13).padEnd(14),
        String(r.replayTrades).padEnd(14),
        `$${r.replayExpectancy.toFixed(2)}`.padEnd(14),
        `${(r.replayWinRate * 100).toFixed(1)}%`.padEnd(14),
        `${r.replayFeePctOfAbsGross.toFixed(0)}%`.padEnd(14),
        r.walkForward.status.padEnd(14),
        r.walkForward.aggregateWFE.toFixed(2).padEnd(14),
        r.recommendation,
      ].join(""),
  );
}

const promoted = rankings.filter((r) => r.recommendation === "PROMOTE");

console.log(`\n[replay:walkforward] Promoted: ${promoted.length} / ${rankings.length} strategies`);
if (promoted.length > 0) {
  console.log(`[replay:walkforward] Recommended IDs: ${promoted.map((r) => r.strategyId).join(",")}`);
  console.log(
    `[replay:walkforward] Env: NEXT_PUBLIC_BTC_FT_STRATEGY_IDS=${promoted.map((r) => r.strategyId).join(",")}`,
  );
  console.log("[replay:walkforward] NOTE: Recommendations only — operator decides whether to apply.");
}

// ─── Optional: write verification event to MongoDB ────────────────────────────

if (isMongoConfigured()) {
  const nowMs = Date.now();
  const event = eventFromReplayWalkForwardRun(nowMs, {
    days,
    barsProcessed: result.barsProcessed,
    totalTrades: summary.count,
    promoted: promoted.length,
    candlesLoaded: candles.length,
    coverageDays,
  });
  void insertVerificationEvents([event]).catch(() => {
    // Non-fatal
  });
}

console.log("\n[replay:walkforward] Done.");
