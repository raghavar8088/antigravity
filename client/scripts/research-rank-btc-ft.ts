#!/usr/bin/env tsx
/**
 * Read Supabase paper_trades, merge optional replay rankings, and write:
 * fixtures/research/btc_ft_verdicts.json
 */

import fs from "fs";
import path from "path";

try {
  const envPath = path.resolve(process.cwd(), ".env.local");
  if (fs.existsSync(envPath)) {
    for (const line of fs.readFileSync(envPath, "utf-8").split("\n")) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith("#")) continue;
      const eqIdx = trimmed.indexOf("=");
      if (eqIdx < 0) continue;
      const key = trimmed.slice(0, eqIdx).trim();
      const val = trimmed.slice(eqIdx + 1).trim();
      if (key && !(key in process.env)) process.env[key] = val;
    }
  }
} catch {
  // ignore local env loading problems
}

const args = Object.fromEntries(
  process.argv.slice(2).map((a) => {
    const [k, v] = a.replace(/^--/, "").split("=");
    return [k!, v ?? "1"];
  }),
);
const windowDays = Number(args["window_days"] ?? "60");
const accountKey = args["account_key"] ?? "";

type ResearchDbRow = {
  strategy_id: number;
  strategy_name?: string;
  net_pnl: number;
  gross_pnl?: number;
  fees?: number;
  opened_at?: string;
  closed_at?: string;
};

type ResearchVerdict = "INSUFFICIENT_DATA" | "CANDIDATE" | "WINNER" | "LOSER";

function computeVerdict(stats: { tradeCount: number; sumNet: number; expectancy: number }): ResearchVerdict {
  const { tradeCount, sumNet, expectancy } = stats;
  if (tradeCount < 10) return "INSUFFICIENT_DATA";
  if (tradeCount >= 15 && (sumNet < -2 || expectancy < -0.1)) return "LOSER";
  if (tradeCount >= 20 && expectancy > 0 && sumNet > 0) return "WINNER";
  return "CANDIDATE";
}

function aggregateStats(rows: ResearchDbRow[]) {
  const map = new Map<number, { name: string; count: number; sumNet: number; wins: number; sumGross: number; sumFees: number; sumHold: number; holdCount: number; lastAt: string | null }>();
  for (const r of rows) {
    if (!Number.isFinite(r.strategy_id) || !Number.isFinite(r.net_pnl)) continue;
    const cur = map.get(r.strategy_id) ?? { name: r.strategy_name ?? `Strat ${r.strategy_id}`, count: 0, sumNet: 0, wins: 0, sumGross: 0, sumFees: 0, sumHold: 0, holdCount: 0, lastAt: null };
    cur.count += 1;
    cur.sumNet += r.net_pnl;
    if (r.net_pnl > 0) cur.wins += 1;
    if (typeof r.gross_pnl === "number") cur.sumGross += Math.abs(r.gross_pnl);
    if (typeof r.fees === "number") cur.sumFees += r.fees;
    if (r.strategy_name?.trim()) cur.name = r.strategy_name.trim();
    if (r.opened_at && r.closed_at) {
      const d = Date.parse(r.closed_at) - Date.parse(r.opened_at);
      if (d > 0) {
        cur.sumHold += d / 60_000;
        cur.holdCount += 1;
      }
    }
    if (r.closed_at && (!cur.lastAt || r.closed_at > cur.lastAt)) cur.lastAt = r.closed_at;
    map.set(r.strategy_id, cur);
  }
  return [...map.entries()].map(([id, s]) => {
    const expectancy = s.count > 0 ? s.sumNet / s.count : 0;
    return {
      strategyId: id,
      strategyName: s.name,
      tradeCount: s.count,
      sumNet: Math.round(s.sumNet * 100) / 100,
      expectancy: Math.round(expectancy * 1000) / 1000,
      winRate: s.count > 0 ? Math.round((s.wins / s.count) * 1000) / 1000 : 0,
      feePctOfGross: s.sumGross > 0 ? Math.round((s.sumFees / s.sumGross) * 10000) / 100 : null,
      avgHoldMin: s.holdCount > 0 ? Math.round((s.sumHold / s.holdCount) * 10) / 10 : null,
      lastTradeAt: s.lastAt,
      verdict: computeVerdict({ tradeCount: s.count, sumNet: s.sumNet, expectancy }),
    };
  }).sort((a, b) => b.sumNet - a.sumNet);
}

async function fetchFromSupabase(key: string, days: number): Promise<ResearchDbRow[]> {
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL;
  const svcKey = process.env.SUPABASE_SERVICE_ROLE_KEY;
  if (!url || !svcKey) {
    console.warn("[research:rank] No Supabase credentials; using empty dataset.");
    return [];
  }
  const cutoff = new Date(Date.now() - days * 24 * 3600_000).toISOString();
  const filter = key ? `account_key=eq.${encodeURIComponent(key)}&` : "";
  const apiUrl = `${url}/rest/v1/paper_trades?${filter}closed_at=gte.${cutoff}&select=strategy_id,strategy_name,net_pnl,gross_pnl,fees,opened_at,closed_at`;
  const res = await fetch(apiUrl, {
    headers: { apikey: svcKey, Authorization: `Bearer ${svcKey}`, "Content-Type": "application/json" },
  });
  if (!res.ok) {
    console.error(`[research:rank] Supabase error ${res.status}: ${await res.text()}`);
    return [];
  }
  return (await res.json()) as ResearchDbRow[];
}

function loadReplayRows(): Map<number, unknown> {
  const replayPath = path.resolve(process.cwd(), "fixtures/replay/btc_ft_strategy_rankings.json");
  try {
    if (!fs.existsSync(replayPath)) return new Map();
    const raw = JSON.parse(fs.readFileSync(replayPath, "utf-8")) as Array<{
      id?: unknown;
      expectancy?: unknown;
      trades?: unknown;
      winRate?: unknown;
    }>;
    return new Map(
      raw
        .filter((r) => typeof r.id === "number")
        .map((r) => [
          r.id as number,
          {
            replayExpectancy: typeof r.expectancy === "number" ? r.expectancy : null,
            replayTrades: typeof r.trades === "number" ? r.trades : null,
            replayWinRate: typeof r.winRate === "number" ? r.winRate : null,
          },
        ]),
    );
  } catch {
    return new Map();
  }
}

async function main() {
  console.log(`[research:rank] window=${windowDays}d account=${accountKey || "(all)"}`);
  const rows = await fetchFromSupabase(accountKey, windowDays);
  const replayRows = loadReplayRows();
  const stats = aggregateStats(rows).map((s) => ({
    ...s,
    ...(replayRows.get(s.strategyId) ?? {}),
  }));

  const outDir = path.resolve(process.cwd(), "fixtures/research");
  fs.mkdirSync(outDir, { recursive: true });
  const outPath = path.join(outDir, "btc_ft_verdicts.json");
  fs.writeFileSync(outPath, JSON.stringify(stats, null, 2));

  const winners = stats.filter((s) => s.verdict === "WINNER").length;
  const losers = stats.filter((s) => s.verdict === "LOSER").length;
  const candidates = stats.filter((s) => s.verdict === "CANDIDATE").length;
  const insufficient = stats.filter((s) => s.verdict === "INSUFFICIENT_DATA").length;
  console.log(`[research:rank] fetched=${rows.length} wrote=${stats.length} -> ${outPath}`);
  console.log(`[research:rank] winners=${winners} losers=${losers} candidates=${candidates} insufficient=${insufficient}`);
}

main().catch((e) => {
  console.error("[research:rank] failed", e);
  process.exitCode = 1;
});
