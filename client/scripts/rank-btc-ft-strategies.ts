#!/usr/bin/env tsx
/**
 * rank-btc-ft-strategies.ts  (Part 6 — stub)
 *
 * Produces `client/fixtures/replay/btc_ft_strategy_rankings.json`
 * consumed by `resolveBtcFtActiveStrategyIds()` (btcFtRoster.ts) when
 * `NEXT_PUBLIC_BTC_FT_USE_RANKED=1`.
 *
 * Usage:
 *   npm run rank:btc-ft
 *   npm run rank:btc-ft -- --fixture=sample --ids=91,92,95,96
 *
 * Output schema:
 *   [{ id, expectancy, trades, winRate }]  sorted desc by expectancy
 *
 * Full version: replace stubReplay() with real replay engine call.
 */

import fs from "fs";
import path from "path";
import { CORE_BTC_FT_STRATEGY_IDS } from "../src/lib/btcFtRoster";

// ---------------------------------------------------------------------------
// CLI arg parsing
// ---------------------------------------------------------------------------
const args = Object.fromEntries(
  process.argv.slice(2).map((a) => {
    const [k, v] = a.replace(/^--/, "").split("=");
    return [k!, v ?? "1"];
  }),
);

const fixtureName: string = typeof args["fixture"] === "string" ? args["fixture"] : "sample";
const idsArg: string | undefined = args["ids"];
const minTrades = Number(args["min-trades"] ?? 10);

const targetIds: number[] = idsArg
  ? idsArg.split(",").map(Number).filter((n) => Number.isFinite(n) && n > 0)
  : [...CORE_BTC_FT_STRATEGY_IDS];

console.log(
  `[rank:btc-ft] fixture=${fixtureName}  ids=${targetIds.join(",")}  minTrades=${minTrades}`,
);

// ---------------------------------------------------------------------------
// Stub replay: generates synthetic metrics so Part 1 has a file to read.
// Replace this section with real replay engine once fixtures exist.
// ---------------------------------------------------------------------------
function stubReplay(ids: number[]): Array<{ id: number; expectancy: number; trades: number; winRate: number }> {
  // Deterministic pseudo-random from id — stable across runs.
  return ids.map((id) => {
    const seed = ((id * 2654435761) >>> 0) / 2 ** 32;
    const trades = Math.max(minTrades, Math.floor(seed * 40 + 10));
    const winRate = 0.45 + seed * 0.2;
    const avgWin = 1.8 + seed * 1.2;
    const avgLoss = -(0.9 + seed * 0.6);
    const expectancy = winRate * avgWin + (1 - winRate) * avgLoss;
    return { id, expectancy: Math.round(expectancy * 1000) / 1000, trades, winRate: Math.round(winRate * 1000) / 1000 };
  });
}

const rows = stubReplay(targetIds).sort((a, b) => b.expectancy - a.expectancy);

// ---------------------------------------------------------------------------
// Write output
// ---------------------------------------------------------------------------
const outDir = path.resolve(__dirname, "../../fixtures/replay");
fs.mkdirSync(outDir, { recursive: true });
const outPath = path.join(outDir, "btc_ft_strategy_rankings.json");
fs.writeFileSync(outPath, JSON.stringify(rows, null, 2));
console.log(`[rank:btc-ft] Wrote ${rows.length} rows → ${outPath}`);
console.log(
  `[rank:btc-ft] Top 3: ${rows
    .slice(0, 3)
    .map((r) => `id=${r.id} exp=${r.expectancy}`)
    .join("  ")}`,
);
