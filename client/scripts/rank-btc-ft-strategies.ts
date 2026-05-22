#!/usr/bin/env tsx
/**
 * rank-btc-ft-strategies.ts
 *
 * Produces `client/fixtures/replay/btc_ft_strategy_rankings.json`
 * consumed by `resolveBtcFtActiveStrategyIds()` (btcFtRoster.ts) when
 * `NEXT_PUBLIC_BTC_FT_USE_RANKED=1`.
 *
 * Usage:
 *   npm run rank:btc-ft
 *   npm run rank:btc-ft -- --fixture=live --bars=1500 --minTrades=5
 *   npm run rank:btc-ft -- --ids=91,92,95,96 --minExpectancy=0
 *
 * Output schema:
 *   [{ id, expectancy, trades, winRate }]  sorted desc by expectancy
 */

import fs from "fs";
import path from "path";
import { CORE_BTC_FT_STRATEGY_IDS } from "../src/lib/btcFtRoster";
import { runPaperDeskReplay } from "../src/lib/futuresReplayEngine";
import type { ReplayFixtureKind } from "../src/lib/futuresReplayFixtures";
import {
  buildReplayConfigFromCli,
  loadEnvLocal,
  loadReplayCandles,
  parseArg,
} from "./replayCliShared";

loadEnvLocal();

// ---------------------------------------------------------------------------
// CLI arg parsing
// ---------------------------------------------------------------------------
const args = Object.fromEntries(
  process.argv.slice(2).map((a) => {
    const [k, v] = a.replace(/^--/, "").split("=");
    return [k!, v ?? "1"];
  }),
);

const fixtureName = (parseArg("fixture", "live") as ReplayFixtureKind) || "live";
const bars = Math.max(500, Number(parseArg("bars", "1500")));
const minTrades = Math.max(1, Number(parseArg("minTrades", "5")));
const minExpectancy = Number(parseArg("minExpectancy", "-999")); // no floor by default — report all

const idsArg: string | undefined = args["ids"];
const targetIds: number[] = idsArg
  ? idsArg.split(",").map(Number).filter((n) => Number.isFinite(n) && n > 0)
  : [...CORE_BTC_FT_STRATEGY_IDS];

console.log(
  `[rank:btc-ft] fixture=${fixtureName}  bars=${bars}  ids=${targetIds.length} strats  minTrades=${minTrades}`,
);

// ---------------------------------------------------------------------------
// Load candles
// ---------------------------------------------------------------------------
const { candles, fundingRate, fixturePath } = loadReplayCandles({ fixture: fixtureName, bars });
console.log(`[rank:btc-ft] Loaded ${candles.length} candles from ${fixturePath}`);

if (candles.length < 100) {
  console.error(`[rank:btc-ft] Too few candles (${candles.length}). Run replay:fetch first.`);
  process.exit(1);
}

// ---------------------------------------------------------------------------
// Run replay for all strategy IDs together (single pass = fast)
// ---------------------------------------------------------------------------
const config = buildReplayConfigFromCli();
config.fundingRate =
  Number.isFinite(config.fundingRate) && config.fundingRate !== 0
    ? config.fundingRate
    : fundingRate;

// Override relevant config for ranking purposes
config.strategyIds = targetIds;
config.signalThreshold = Number(parseArg("signalThreshold", "28"));
config.maxPositions = 60; // high cap so every strategy gets entries
config.slippageBps = Number(parseArg("slippageBps", "5"));
config.minExpectedMoveSafetyK = 1.1;

console.log(
  `[rank:btc-ft] Running replay: threshold=${config.signalThreshold}  slip=${config.slippageBps}bps  maxPos=${config.maxPositions}`,
);

const result = runPaperDeskReplay(candles, config);
console.log(
  `[rank:btc-ft] Replay done: ${result.trades.length} total trades  finalBalance=$${result.finalBalance.toFixed(2)}`,
);

// ---------------------------------------------------------------------------
// Aggregate per strategy
// ---------------------------------------------------------------------------
const byId = new Map<number, { netPnls: number[] }>();
for (const t of result.trades) {
  if (!byId.has(t.strategyId)) byId.set(t.strategyId, { netPnls: [] });
  byId.get(t.strategyId)!.netPnls.push(t.netPnl);
}

type RankingRow = {
  id: number;
  expectancy: number;
  trades: number;
  winRate: number;
  sumNet: number;
};

const rows: RankingRow[] = targetIds
  .map((id) => {
    const bucket = byId.get(id);
    if (!bucket || bucket.netPnls.length < minTrades) {
      return { id, expectancy: 0, trades: bucket?.netPnls.length ?? 0, winRate: 0, sumNet: 0 };
    }
    const { netPnls } = bucket;
    const sumNet = netPnls.reduce((s, p) => s + p, 0);
    const wins = netPnls.filter((p) => p > 0).length;
    const expectancy = sumNet / netPnls.length;
    const winRate = wins / netPnls.length;
    return {
      id,
      expectancy: Math.round(expectancy * 10_000) / 10_000,
      trades: netPnls.length,
      winRate: Math.round(winRate * 10_000) / 10_000,
      sumNet: Math.round(sumNet * 100) / 100,
    };
  })
  .filter((r) => r.expectancy >= minExpectancy)
  .sort((a, b) => b.expectancy - a.expectancy);

// ---------------------------------------------------------------------------
// Write output
// ---------------------------------------------------------------------------
const outDir = path.resolve(__dirname, "../../fixtures/replay");
fs.mkdirSync(outDir, { recursive: true });
const outPath = path.join(outDir, "btc_ft_strategy_rankings.json");
fs.writeFileSync(outPath, JSON.stringify(rows, null, 2));

console.log(`[rank:btc-ft] Wrote ${rows.length} rows → ${outPath}`);
if (rows.length > 0) {
  console.log(
    `[rank:btc-ft] Top 5:`,
    rows
      .slice(0, 5)
      .map((r) => `id=${r.id} exp=$${r.expectancy.toFixed(4)} trades=${r.trades} wr=${(r.winRate * 100).toFixed(1)}%`)
      .join("  "),
  );
  console.log(
    `[rank:btc-ft] Bottom 3:`,
    rows
      .slice(-3)
      .map((r) => `id=${r.id} exp=$${r.expectancy.toFixed(4)} trades=${r.trades}`)
      .join("  "),
  );
}

const positiveExpectancy = rows.filter((r) => r.trades >= minTrades && r.expectancy > 0);
console.log(
  `[rank:btc-ft] Strategies with positive expectancy + ≥${minTrades} trades: ${positiveExpectancy.length}/${rows.length}`,
);
