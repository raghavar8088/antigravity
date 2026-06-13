/**
 * futuresAttribution.ts
 * Pure functions. No side effects. Fully testable.
 *
 * Attributes wins and losses to specific factors:
 * regime, time of day, hold duration, signal strength.
 */

import { isProbeOrBootstrapTrade } from "./futuresSessionMetrics";
import type { PaperTradeDbRow } from "../portfolio/paperTradesTypes";

export interface AttributionBucket {
  label: string;
  trades: number;
  wins: number;
  losses: number;
  winRate: number;
  totalNetPnl: number;
  avgNetPnl: number;
  totalFees: number;
}

export interface AttributionReport {
  byHoldDuration: AttributionBucket[];
  byHourOfDay: AttributionBucket[];
  byExitReason: AttributionBucket[];
  byTemplateFamily: AttributionBucket[];
  bySide: AttributionBucket[];
  bestHour: number | null;
  worstHour: number | null;
  bestHoldBucket: string | null;
  worstHoldBucket: string | null;
  totalAnalyzed: number;
  computedAt: number;
}

function holdMinutes(t: PaperTradeDbRow): number {
  if (!t.closed_at || !t.opened_at) return 0;
  return (new Date(t.closed_at).getTime() - new Date(t.opened_at).getTime()) / 60_000;
}

function holdBucket(mins: number): string {
  if (mins < 5) return "<5m";
  if (mins < 15) return "5-15m";
  if (mins < 60) return "15-60m";
  return ">60m";
}

function buildBuckets(groups: Map<string, PaperTradeDbRow[]>): AttributionBucket[] {
  const buckets: AttributionBucket[] = [];

  for (const [label, groupTrades] of groups) {
    const wins = groupTrades.filter((t) => (t.net_pnl ?? 0) > 0);
    const losses = groupTrades.filter((t) => (t.net_pnl ?? 0) <= 0);
    const totalNet = groupTrades.reduce((s, t) => s + (t.net_pnl ?? 0), 0);
    const totalFees = groupTrades.reduce((s, t) => s + (t.fees ?? 0), 0);

    buckets.push({
      label,
      trades: groupTrades.length,
      wins: wins.length,
      losses: losses.length,
      winRate: groupTrades.length ? wins.length / groupTrades.length : 0,
      totalNetPnl: totalNet,
      avgNetPnl: groupTrades.length ? totalNet / groupTrades.length : 0,
      totalFees,
    });
  }

  return buckets.sort((a, b) => b.avgNetPnl - a.avgNetPnl);
}

export function computeAttribution(trades: PaperTradeDbRow[]): AttributionReport {
  const prod = trades.filter(
    (t) => !isProbeOrBootstrapTrade({ strategy_name: t.strategy_name }) && t.closed_at,
  );

  const holdGroups = new Map<string, PaperTradeDbRow[]>();
  for (const bucket of ["<5m", "5-15m", "15-60m", ">60m"]) {
    holdGroups.set(bucket, []);
  }
  for (const t of prod) {
    const b = holdBucket(holdMinutes(t));
    holdGroups.get(b)!.push(t);
  }
  const byHoldDuration = buildBuckets(holdGroups);

  const hourGroups = new Map<string, PaperTradeDbRow[]>();
  for (let h = 0; h < 24; h++) hourGroups.set(String(h), []);
  for (const t of prod) {
    if (!t.opened_at) continue;
    const h = new Date(t.opened_at).getUTCHours();
    hourGroups.get(String(h))!.push(t);
  }
  const byHourOfDay = buildBuckets(hourGroups).filter((b) => b.trades > 0);

  const exitGroups = new Map<string, PaperTradeDbRow[]>();
  for (const t of prod) {
    const r = t.exit_reason ?? "UNKNOWN";
    if (!exitGroups.has(r)) exitGroups.set(r, []);
    exitGroups.get(r)!.push(t);
  }
  const byExitReason = buildBuckets(exitGroups);

  const familyGroups = new Map<string, PaperTradeDbRow[]>();
  for (const t of prod) {
    const f = t.template_family ?? "unknown";
    if (!familyGroups.has(f)) familyGroups.set(f, []);
    familyGroups.get(f)!.push(t);
  }
  const byTemplateFamily = buildBuckets(familyGroups);

  const sideGroups = new Map<string, PaperTradeDbRow[]>([
    ["LONG", prod.filter((t) => t.side === "LONG")],
    ["SHORT", prod.filter((t) => t.side === "SHORT")],
  ]);
  const bySide = buildBuckets(sideGroups);

  const hourWithData = byHourOfDay.filter((b) => b.trades >= 3);
  const bestHour = hourWithData.length > 0 ? parseInt(hourWithData[0].label, 10) : null;
  const worstHour =
    hourWithData.length > 0 ? parseInt(hourWithData[hourWithData.length - 1].label, 10) : null;

  const holdWithData = byHoldDuration.filter((b) => b.trades >= 3);
  const bestHoldBucket = holdWithData[0]?.label ?? null;
  const worstHoldBucket = holdWithData[holdWithData.length - 1]?.label ?? null;

  return {
    byHoldDuration,
    byHourOfDay,
    byExitReason,
    byTemplateFamily,
    bySide,
    bestHour,
    worstHour,
    bestHoldBucket,
    worstHoldBucket,
    totalAnalyzed: prod.length,
    computedAt: Date.now(),
  };
}
