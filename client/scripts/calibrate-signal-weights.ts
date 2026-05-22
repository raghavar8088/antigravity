#!/usr/bin/env tsx
/**
 * calibrate-signal-weights.ts  (P0.1.1)
 *
 * Reads closed trades from MongoDB (with signalContributions attached),
 * computes per-feature win-rate and correlation with positive P&L,
 * and outputs a suggested weight table for `evalBtcFtTemplateSignal`.
 *
 * Usage:
 *   npm run calibrate:weights
 *   npm run calibrate:weights -- --window=30 --minSamples=10
 *
 * Requirements:
 *   - MONGODB_URI must be set in .env.local
 *   - Trades must have been captured with signalContributions attached
 *     (available after running the engine with P0.1.1 changes deployed)
 *
 * Output:
 *   - Console table of feature → current weight | suggested weight | confidence
 *   - Writes `fixtures/calibration/signal_weight_suggestions.json`
 */

import fs from "fs";
import path from "path";
import { MongoClient } from "mongodb";
import { loadEnvLocal, parseArg } from "./replayCliShared";

loadEnvLocal();

const windowDays = Number(parseArg("window", "30"));
const minSamples = Number(parseArg("minSamples", "10"));
const accountKey = parseArg("account_key", "btc_future_trading_20");

const MONGODB_URI = process.env.MONGODB_URI;
const MONGODB_DB = process.env.MONGODB_DB || "loop_trades";

if (!MONGODB_URI) {
  console.error("[calibrate] MONGODB_URI not set. Add it to .env.local");
  process.exit(1);
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
type Contribution = { reason: string; pts: number };
type TradeSample = {
  strategyId: number;
  netPnl: number;
  signalContributions?: Contribution[];
};

type FeatureStats = {
  reason: string;
  currentPts: number | null; // null if seen with multiple different pts
  samples: number;
  wins: number;
  sumPnl: number;
  winRate: number;
  avgPnlWhenPresent: number;
  suggestedPts: number;
  confidence: "high" | "medium" | "low";
};

// ---------------------------------------------------------------------------
// Fetch trades from Mongo
// ---------------------------------------------------------------------------
async function fetchTrades(): Promise<TradeSample[]> {
  const client = new MongoClient(MONGODB_URI!, { serverSelectionTimeoutMS: 10_000 });
  try {
    await client.connect();
    const db = client.db(MONGODB_DB);
    const cutoff = new Date(Date.now() - windowDays * 24 * 60 * 60 * 1000).toISOString();
    const docs = await db
      .collection("paper_trades")
      .find({ account_key: accountKey, closed_at: { $gte: cutoff } })
      .project({ strategy_id: 1, net_pnl: 1, signal_contributions: 1, _id: 0 })
      .toArray();

    return docs.map((d) => ({
      strategyId: Number(d.strategy_id),
      netPnl: Number(d.net_pnl),
      signalContributions: Array.isArray(d.signal_contributions) ? d.signal_contributions : undefined,
    }));
  } finally {
    await client.close();
  }
}

// ---------------------------------------------------------------------------
// Aggregate feature stats
// ---------------------------------------------------------------------------
function aggregateFeatureStats(trades: TradeSample[]): FeatureStats[] {
  const tradesWithContribs = trades.filter((t) => t.signalContributions && t.signalContributions.length > 0);
  console.log(`[calibrate] ${trades.length} total trades, ${tradesWithContribs.length} with signal contributions`);

  if (tradesWithContribs.length === 0) {
    console.log(
      "[calibrate] No trades have signalContributions yet.\n" +
        "  → Run the engine with the P0.1.1 build for at least a few days to collect data.",
    );
    return [];
  }

  // Map: reason → list of { pts, netPnl }
  const featureMap = new Map<string, { pts: number[]; netPnls: number[] }>();
  for (const trade of tradesWithContribs) {
    for (const c of trade.signalContributions!) {
      if (!featureMap.has(c.reason)) featureMap.set(c.reason, { pts: [], netPnls: [] });
      const bucket = featureMap.get(c.reason)!;
      bucket.pts.push(c.pts);
      bucket.netPnls.push(trade.netPnl);
    }
  }

  // Compute per-feature stats
  const rows: FeatureStats[] = [];
  for (const [reason, { pts, netPnls }] of featureMap.entries()) {
    if (netPnls.length < minSamples) continue;

    const uniquePts = [...new Set(pts)];
    const currentPts = uniquePts.length === 1 ? uniquePts[0]! : null;

    const wins = netPnls.filter((p) => p > 0).length;
    const sumPnl = netPnls.reduce((s, p) => s + p, 0);
    const winRate = wins / netPnls.length;
    const avgPnlWhenPresent = sumPnl / netPnls.length;

    // Suggested weight: scale current pts proportional to how much win-rate
    // exceeds base (50%). Features with < 40% win-rate get reduced.
    const basePts = currentPts ?? (uniquePts.reduce((s, p) => s + p, 0) / uniquePts.length);
    let suggestedPts: number;
    if (winRate >= 0.55) {
      suggestedPts = Math.round(basePts * 1.2); // boost high-performing features
    } else if (winRate >= 0.45) {
      suggestedPts = Math.round(basePts); // neutral — keep
    } else if (winRate >= 0.35) {
      suggestedPts = Math.round(basePts * 0.7); // reduce underperforming
    } else {
      suggestedPts = Math.round(basePts * 0.4); // halve weak features
    }

    const confidence: FeatureStats["confidence"] =
      netPnls.length >= 50 ? "high" : netPnls.length >= 20 ? "medium" : "low";

    rows.push({
      reason,
      currentPts,
      samples: netPnls.length,
      wins,
      sumPnl: Math.round(sumPnl * 100) / 100,
      winRate: Math.round(winRate * 1000) / 1000,
      avgPnlWhenPresent: Math.round(avgPnlWhenPresent * 10000) / 10000,
      suggestedPts,
      confidence,
    });
  }

  return rows.sort((a, b) => b.samples - a.samples);
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------
async function main() {
  console.log(`[calibrate] Fetching trades: account=${accountKey}  window=${windowDays}d  minSamples=${minSamples}`);

  let trades: TradeSample[];
  try {
    trades = await fetchTrades();
  } catch (err) {
    console.error("[calibrate] Mongo fetch failed:", err instanceof Error ? err.message : err);
    process.exit(1);
  }

  if (trades.length === 0) {
    console.log("[calibrate] No trades found for the given window. Nothing to calibrate.");
    return;
  }

  const rows = aggregateFeatureStats(trades);
  if (rows.length === 0) return;

  // Print table
  console.log("\n╔══════════════════════════════════════════════════════════════════════════════╗");
  console.log("║ Feature weight calibration suggestions (NOT auto-applied — review manually) ║");
  console.log("╚══════════════════════════════════════════════════════════════════════════════╝\n");

  const header = ["Feature", "Curr pts", "Sugg pts", "Samples", "WinRate", "AvgPnL", "Confidence"].map((h) =>
    h.padEnd(14),
  );
  console.log(header.join(" │ "));
  console.log("─".repeat(110));

  for (const r of rows) {
    const delta = r.currentPts !== null ? (r.suggestedPts > r.currentPts ? "▲" : r.suggestedPts < r.currentPts ? "▼" : "=") : "?";
    const cols = [
      r.reason.slice(0, 22).padEnd(22),
      String(r.currentPts ?? "mixed").padEnd(8),
      `${r.suggestedPts} ${delta}`.padEnd(8),
      String(r.samples).padEnd(7),
      `${(r.winRate * 100).toFixed(1)}%`.padEnd(7),
      `$${r.avgPnlWhenPresent.toFixed(4)}`.padEnd(10),
      r.confidence.padEnd(10),
    ];
    console.log(cols.join(" │ "));
  }

  console.log("\n⚠  These suggestions require manual review and a new replay validation before applying.");
  console.log("   Minimum recommended: high-confidence rows with ≥50 samples and clear win-rate signal.\n");

  // Write output
  const outDir = path.resolve(__dirname, "../../fixtures/calibration");
  fs.mkdirSync(outDir, { recursive: true });
  const outPath = path.join(outDir, "signal_weight_suggestions.json");
  fs.writeFileSync(outPath, JSON.stringify({ generatedAt: new Date().toISOString(), windowDays, minSamples, suggestions: rows }, null, 2));
  console.log(`[calibrate] Wrote suggestions → ${outPath}`);
}

main().catch((e) => {
  console.error(e instanceof Error ? e.message : e);
  process.exit(1);
});
