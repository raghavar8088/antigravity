/**
 * Collect a live AiAppTrackerSnapshot from MongoDB paper_state.
 * Server-side only — imports MongoDB helpers.
 *
 * Security: never stores the full account key. Only the last 4 chars.
 *
 * Two entry points:
 *   collectAppSnapshot({ accountKey?, nowMs? })   — async, fetches MongoDB
 *   buildSnapshotFromRaw(raw, nowMs?)              — pure, for unit tests
 */

import type { AiAppTrackerSnapshot } from "./types";
import {
  STALE_WORKER_THRESHOLD_MS,
  BALANCE_DRIFT_WARN_USD,
  STALE_FUNNEL_THRESHOLD_MS,
  TRACKER_MODULE,
} from "./trackerConstants";
import type { EntryFunnelSnapshot } from "@/lib/trading/deskEntryFunnelSnapshot";

// ── helpers ───────────────────────────────────────────────────────────────────

function safeKeySuffix(key: string): string {
  if (!key || key.length < 4) return "****";
  return "…" + key.slice(-4);
}

// ── raw input shape (for the pure builder) ────────────────────────────────────

export type SnapshotRawInput = {
  accountKey: string | null;
  nowMs: number;
  buildSha: string | null;
  workerEnabled: boolean;
  workerLastPollAt: number | null;
  workerOwner: string | null;
  funnelSnapshot: EntryFunnelSnapshot | null;
  balance: number | null;
  dayStartBalance: number | null;
  openPositionsCount: number;
  pauseEntries: boolean;
  clearedAt: number | null;
  hasPaperState: boolean;
  signalTraceRaw?: Record<string, unknown> | null;
  verificationSummary?: any;
};

/**
 * Pure builder — no I/O. Used directly by unit tests and called internally
 * by the async collectAppSnapshot after data is fetched.
 */
export function buildSnapshotFromRaw(raw: SnapshotRawInput): AiAppTrackerSnapshot {
  const {
    accountKey, nowMs, buildSha,
    workerEnabled, workerLastPollAt, workerOwner,
    funnelSnapshot,
    balance, dayStartBalance, openPositionsCount, pauseEntries, clearedAt,
    hasPaperState, signalTraceRaw, verificationSummary,
  } = raw;

  const warnings: string[] = [];

  // Missing account key
  if (!accountKey) {
    warnings.push(
      "DESK_WORKER_ACCOUNT_KEY is not set — cannot read desk state. Set the env var or sign in.",
    );
  }

  // No paper state
  if (accountKey && !hasPaperState) {
    warnings.push(
      "No paper_state found in MongoDB for this account. Worker has not run yet or the account key is wrong.",
    );
  }

  // Worker stale
  const workerAgeMs =
    workerLastPollAt != null ? nowMs - workerLastPollAt : null;
  const workerAgeSeconds =
    workerAgeMs != null ? Math.floor(workerAgeMs / 1000) : null;
  const workerStale =
    workerAgeMs != null && workerAgeMs > STALE_WORKER_THRESHOLD_MS;

  if (workerAgeMs != null && workerStale) {
    warnings.push(
      `Worker heartbeat is stale (${workerAgeSeconds}s old, threshold ${STALE_WORKER_THRESHOLD_MS / 1000}s). Restart: pm2 restart btc-ft-paper-worker on VPS.`,
    );
  }

  // Entry funnel stale
  const funnelTickAt = funnelSnapshot?.tickAt ?? null;
  const funnelAgeMs =
    funnelTickAt != null ? nowMs - funnelTickAt : null;
  const funnelAgeSeconds =
    funnelAgeMs != null ? Math.floor(funnelAgeMs / 1000) : null;

  if (funnelAgeMs != null && funnelAgeMs > STALE_FUNNEL_THRESHOLD_MS) {
    warnings.push(
      `Entry funnel snapshot is stale (${funnelAgeSeconds}s old). Worker or browser may have stopped polling.`,
    );
  }

  // Balance drift
  const balanceDriftUsd =
    balance != null && dayStartBalance != null
      ? balance - dayStartBalance
      : null;

  if (balanceDriftUsd != null && Math.abs(balanceDriftUsd) >= BALANCE_DRIFT_WARN_USD) {
    warnings.push(
      `Balance drifted ${balanceDriftUsd >= 0 ? "+" : ""}$${balanceDriftUsd.toFixed(2)} from day start. Run computeSessionEquityFromProduction() to verify.`,
    );
  }

  // Pause entries flag
  if (pauseEntries) {
    warnings.push("Entries are paused (pauseEntries=true in paper_state). No new positions will open.");
  }

  // No candidates / no opened this tick
  const candidates = funnelSnapshot?.candidateCount ?? null;
  const opened = funnelSnapshot?.opened ?? null;
  if (hasPaperState && candidates === 0 && opened === 0 && !workerStale) {
    warnings.push(
      `No entry candidates or trades opened last tick. Dominant blocker: ${funnelSnapshot?.dominantBlocker ?? "unknown"}.`,
    );
  }

  // Env flags
  const env = {
    profitMode: process.env.NEXT_PUBLIC_BTC_FT_PROFIT_MODE === "1",
    winnersOnly: process.env.NEXT_PUBLIC_BTC_FT_WINNERS_ONLY === "1",
    researchMode: process.env.NEXT_PUBLIC_RESEARCH_MODE === "1",
    workerEnabled,
  };

  return {
    createdAt: new Date(nowMs).toISOString(),
    appBuildSha: buildSha,
    module: TRACKER_MODULE,
    accountKeySuffix: accountKey ? safeKeySuffix(accountKey) : null,

    worker: {
      enabled: workerEnabled,
      stale: workerStale,
      lastPollAt: workerLastPollAt,
      ageSeconds: workerAgeSeconds,
      owner: workerOwner,
    },

    entryFunnel: {
      ageSeconds: funnelAgeSeconds,
      dominantBlocker: funnelSnapshot?.dominantBlocker ?? null,
      recommendation: funnelSnapshot?.recommendation ?? null,
      activeStrategies: funnelSnapshot?.activeStrategies ?? null,
      evaluatedStrategies: funnelSnapshot?.evaluatedStrategies ?? null,
      candidates,
      opened,
    },

    paperState: {
      balance,
      openPositions: openPositionsCount,
      workerLastPollAt,
      balanceDriftUsd,
      pauseEntries,
      clearedAt,
    },

    env,
    warnings,
    signalTrace: signalTraceRaw
      ? {
          totalEvaluated: Number(signalTraceRaw.summary && (signalTraceRaw.summary as Record<string, unknown>).totalEvaluated) || 0,
          fired: Number(signalTraceRaw.summary && (signalTraceRaw.summary as Record<string, unknown>).fired) || 0,
          candidates: Number(signalTraceRaw.summary && (signalTraceRaw.summary as Record<string, unknown>).candidates) || 0,
          opened: Number(signalTraceRaw.summary && (signalTraceRaw.summary as Record<string, unknown>).opened) || 0,
          topRejectedGate: typeof ((signalTraceRaw.summary as Record<string, unknown> | null)?.topRejectedGate) === "string" ? (signalTraceRaw.summary as Record<string, unknown>).topRejectedGate as string : null,
          ageSeconds: signalTraceRaw.tickAt ? Math.floor((nowMs - Number(signalTraceRaw.tickAt)) / 1000) : null,
          mode: typeof signalTraceRaw.mode === "string" ? signalTraceRaw.mode : null,
        }
      : null,
  };
}

// ── async entry point (server-side) ──────────────────────────────────────────

export async function collectAppSnapshot(opts: {
  accountKey?: string;
  nowMs?: number;
}): Promise<AiAppTrackerSnapshot> {
  const nowMs = opts.nowMs ?? Date.now();
  const accountKey = opts.accountKey?.trim() || null;

  const buildSha =
    process.env.NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA?.slice(0, 7) ||
    process.env.NEXT_PUBLIC_BTC_FT_DESK_BUILD?.trim() ||
    null;

  const workerEnabled = process.env.NEXT_PUBLIC_DESK_WORKER_ENABLED === "1";

  // No account key — can't query MongoDB. Build a minimal warning snapshot.
  if (!accountKey) {
    return buildSnapshotFromRaw({
      accountKey: null,
      nowMs,
      buildSha,
      workerEnabled,
      workerLastPollAt: null,
      workerOwner: null,
      funnelSnapshot: null,
      balance: null,
      dayStartBalance: null,
      openPositionsCount: 0,
      pauseEntries: false,
      clearedAt: null,
      hasPaperState: false,
    });
  }

  // Dynamic import keeps this server-side only (mongoTradesClient uses Node.js APIs)
  const { isMongoConfigured, getAccountState } = await import("@/lib/broker/mongoTradesClient");

  if (!isMongoConfigured()) {
    const snap = buildSnapshotFromRaw({
      accountKey,
      nowMs,
      buildSha,
      workerEnabled,
      workerLastPollAt: null,
      workerOwner: null,
      funnelSnapshot: null,
      balance: null,
      dayStartBalance: null,
      openPositionsCount: 0,
      pauseEntries: false,
      clearedAt: null,
      hasPaperState: false,
    });
    snap.warnings.unshift("MongoDB is not configured (MONGODB_URI missing or invalid).");
    return snap;
  }

  const state = await getAccountState(accountKey);
  const hasPaperState = state != null;

  const funnelSnapshot =
    (state?.entry_funnel_snapshot as EntryFunnelSnapshot | null) ?? null;

  const openPositions = Array.isArray(state?.positions)
    ? (state!.positions as unknown[]).length
    : 0;

  const signalTraceRaw =
    (state?.signal_trace_latest as Record<string, unknown> | null) ?? null;

  // Verification track summary (best-effort, non-blocking)
  let verificationSummary: any = null;
  try {
    const vRes = await fetch(`/api/verification-track/summary?window_minutes=60`);
    if (vRes.ok) verificationSummary = await vRes.json();
  } catch { /* ignore */ }

  return buildSnapshotFromRaw({
    accountKey,
    nowMs,
    buildSha,
    workerEnabled,
    workerLastPollAt: state?.worker_last_poll_at ?? null,
    workerOwner: state?.worker_owner ?? null,
    funnelSnapshot,
    balance: state?.balance ?? null,
    dayStartBalance: state?.day_start_balance ?? null,
    openPositionsCount: openPositions,
    pauseEntries: state?.pause_entries ?? false,
    clearedAt: state?.cleared_at ?? null,
    hasPaperState,
    signalTraceRaw,
    verificationSummary,
  });
}
