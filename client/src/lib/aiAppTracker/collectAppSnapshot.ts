/**
 * Collect a live AiTrackerSnapshot from MongoDB paper_state + trade counts.
 * Server-side only — imports MongoDB helpers.
 *
 * Security: never stores the full account key. Only the last 4 chars are kept.
 */

import type { AiTrackerSnapshot, AiTrackerWarning, AiTrackerRotationSummary } from "./types";
import {
  STALE_WORKER_THRESHOLD_MS,
  NO_TRADE_WINDOW_MS,
  BALANCE_DRIFT_WARN_USD,
  STALE_FUNNEL_THRESHOLD_MS,
} from "./trackerConstants";
import type { EntryFunnelSnapshot } from "@/lib/deskEntryFunnelSnapshot";

export type CollectSnapshotInput = {
  accountKey: string;
  buildSha: string | null;
  balance: number | null;
  dayStartBalance: number | null;
  workerLastPollAt: number | null;
  workerOwner: "browser" | "vps" | null;
  openPositionsCount: number;
  closedTradesCount: number;
  lastTradeAt: number | null;
  funnelSnapshot: EntryFunnelSnapshot | null;
  rotationReport: unknown | null;
  probeDominant: boolean;
};

function safeKeySuffix(key: string): string {
  if (!key || key.length < 4) return "****";
  return "…" + key.slice(-4);
}

function buildRotationSummary(report: unknown): AiTrackerRotationSummary | null {
  if (!report || typeof report !== "object") return null;
  const r = report as Record<string, unknown>;
  const scores = Array.isArray(r.scores) ? r.scores as Array<Record<string, unknown>> : [];
  const suspended = Array.isArray(r.suspended) ? (r.suspended as unknown[]).length : 0;
  const promoted = Array.isArray(r.promoted) ? (r.promoted as unknown[]).length : 0;
  const active = scores.filter(
    (s) => s.status !== "SUSPENDED" && s.status !== "INSUFFICIENT_DATA",
  ).length;
  const top = scores.sort((a, b) => ((b.score as number) ?? 0) - ((a.score as number) ?? 0))[0];
  return {
    active,
    suspended,
    promoted,
    topStrategyId: top ? ((top.strategyId as number) ?? null) : null,
  };
}

export function collectAppSnapshot(input: CollectSnapshotInput): AiTrackerSnapshot {
  const now = Date.now();
  const capturedAt = new Date().toISOString();

  const accountKeySuffix = input.accountKey ? safeKeySuffix(input.accountKey) : null;

  // Worker age
  const workerLastPollAgeSeconds =
    input.workerLastPollAt != null
      ? Math.floor((now - input.workerLastPollAt) / 1000)
      : null;

  // Funnel
  const snap = input.funnelSnapshot;
  const funnelTickAt = snap?.tickAt ?? null;
  const funnelTickAgeSeconds =
    funnelTickAt != null ? Math.floor((now - funnelTickAt) / 1000) : null;

  const dominantBlocker = snap?.dominantBlocker ?? "unknown";
  const recommendation = snap?.recommendation ?? "";
  const activeStrategies = snap?.activeStrategies ?? 0;
  const evaluatedStrategies = snap?.evaluatedStrategies ?? 0;
  const signalPassed = snap?.signalPassed ?? 0;
  const opened = snap?.opened ?? 0;

  // Balance drift
  const balanceDriftUsd =
    input.balance != null && input.dayStartBalance != null
      ? input.balance - input.dayStartBalance
      : null;

  // Last trade date
  const lastTradeAt =
    input.lastTradeAt && input.lastTradeAt > 0
      ? new Date(input.lastTradeAt).toISOString()
      : null;

  // Rotation
  const rotationSummary = buildRotationSummary(input.rotationReport);

  // Env flags
  const envFlags = {
    profitMode: process.env.NEXT_PUBLIC_BTC_FT_PROFIT_MODE === "1",
    winnersOnly: process.env.NEXT_PUBLIC_BTC_FT_WINNERS_ONLY === "1",
    workerEnabled: process.env.NEXT_PUBLIC_DESK_WORKER_ENABLED === "1",
    researchMode: process.env.NEXT_PUBLIC_RESEARCH_MODE === "1",
    entryCanary: process.env.NEXT_PUBLIC_DESK_ENTRY_CANARY === "1",
  };

  // Warnings
  const warnings: AiTrackerWarning[] = [];

  if (
    workerLastPollAgeSeconds != null &&
    workerLastPollAgeSeconds * 1000 > STALE_WORKER_THRESHOLD_MS
  ) {
    warnings.push({
      code: "stale_worker",
      message: `Worker heartbeat is ${workerLastPollAgeSeconds}s old (threshold ${STALE_WORKER_THRESHOLD_MS / 1000}s).`,
    });
  }

  if (funnelTickAgeSeconds != null && funnelTickAgeSeconds * 1000 > STALE_FUNNEL_THRESHOLD_MS) {
    warnings.push({
      code: "data_gap",
      message: `Funnel snapshot is ${funnelTickAgeSeconds}s old — worker or browser may not be polling.`,
    });
  }

  const noTradeForLong =
    input.lastTradeAt != null
      ? now - input.lastTradeAt > NO_TRADE_WINDOW_MS
      : false;
  if (noTradeForLong) {
    const minAgo = Math.floor((now - (input.lastTradeAt ?? 0)) / 60_000);
    warnings.push({
      code: "no_trades_30min",
      message: `No new trade in ${minAgo} minutes. Dominant blocker: ${dominantBlocker}.`,
    });
  }

  if (balanceDriftUsd != null && Math.abs(balanceDriftUsd) >= BALANCE_DRIFT_WARN_USD) {
    warnings.push({
      code: "balance_drift",
      message: `Balance drifted ${balanceDriftUsd >= 0 ? "+" : ""}$${balanceDriftUsd.toFixed(2)} from day-start.`,
    });
  }

  if (input.probeDominant) {
    warnings.push({
      code: "probe_dominant",
      message: "Last 5+ closed trades are probe/bootstrap — production signal edge not visible yet.",
    });
  }

  if (
    rotationSummary != null &&
    rotationSummary.active === 0 &&
    rotationSummary.suspended > 0
  ) {
    warnings.push({
      code: "no_strategy_candidates",
      message: `All ${rotationSummary.suspended} strategies suspended by rotation. No candidates available.`,
    });
  }

  return {
    capturedAt,
    buildSha: input.buildSha,
    accountKeySuffix,
    workerLastPollAgeSeconds,
    workerOwner: input.workerOwner,
    funnelTickAt,
    funnelTickAgeSeconds,
    dominantBlocker,
    recommendation,
    activeStrategies,
    evaluatedStrategies,
    signalPassed,
    opened,
    balance: input.balance,
    balanceDriftUsd,
    openPositionsCount: input.openPositionsCount,
    closedTradesCount: input.closedTradesCount,
    lastTradeAt,
    rotationSummary,
    envFlags,
    warnings,
  };
}
