/**
 * Pure builders that convert existing diagnostic snapshots into VerificationTrackEvent records.
 * No side effects, no I/O, no trading logic changes.
 */

import type { VerificationTrackEvent, VerificationTrackEventType } from "./types";
import type { EntryFunnelSnapshot } from "@/lib/deskEntryFunnelSnapshot";
import type { StrategySignalTraceRow, SignalTraceSummary } from "@/lib/strategySignalTrace";
import type { NoTradeRootCauseResult } from "@/lib/noTradeRootCause";
import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";

function makeId(tickAt: number, type: VerificationTrackEventType, extra = ""): string {
  return `${tickAt}-${type}${extra ? "-" + extra : ""}-${Math.random().toString(36).slice(2, 9)}`;
}

function suffix(key: string | null | undefined): string | null {
  if (!key) return null;
  const s = key.trim();
  return s.length > 8 ? s.slice(-8) : s;
}

/** ENTRY_FUNNEL snapshot event (emitted on blocker change or every 15 min). */
export function eventFromEntryFunnel(
  tickAt: number,
  funnel: EntryFunnelSnapshot,
  workerOwner: "browser" | "vps" | "cron" | null,
  accountKey?: string | null,
  buildSha?: string | null,
): VerificationTrackEvent {
  return {
    event_id: makeId(tickAt, "ENTRY_FUNNEL"),
    created_at: new Date(tickAt).toISOString(),
    module: "btc_future_trading",
    account_key_suffix: suffix(accountKey),
    worker_owner: workerOwner,
    build_sha: buildSha ?? null,
    type: "ENTRY_FUNNEL",
    severity: funnel.dominantBlocker === "none" ? "info" : "warning",
    summary: `Funnel blocker=${funnel.dominantBlocker} active=${funnel.activeStrategies} evaluated=${funnel.evaluatedStrategies} opened=${funnel.opened}`,
    dominant_blocker: funnel.dominantBlocker,
    snapshot: {
      active: funnel.activeStrategies,
      evaluated: funnel.evaluatedStrategies,
      signalPassed: funnel.signalPassed,
      candidates: funnel.candidateCount,
      opened: funnel.opened,
      blockerCounts: funnel.blockerCounts,
    },
  };
}

/** SIGNAL_TRACE — only the interesting rows (opened + candidates + top rejected + closest). */
export function eventsFromSignalTraceRows(
  tickAt: number,
  rows: StrategySignalTraceRow[],
  summary: SignalTraceSummary,
  workerOwner: "browser" | "vps" | "cron" | null,
  accountKey?: string | null,
): VerificationTrackEvent[] {
  if (rows.length === 0) return [];

  const events: VerificationTrackEvent[] = [];

  // One summary event per tick
  events.push({
    event_id: makeId(tickAt, "SIGNAL_TRACE"),
    created_at: new Date(tickAt).toISOString(),
    module: "btc_future_trading",
    account_key_suffix: suffix(accountKey),
    worker_owner: workerOwner,
    type: "SIGNAL_TRACE",
    severity: summary.fired > 0 ? "info" : "warning",
    summary: `Trace evaluated=${summary.totalEvaluated} fired=${summary.fired} candidates=${summary.candidates} opened=${summary.opened} topReject=${summary.topRejectedGate ?? "none"}`,
    snapshot: { summary, topRejectedGate: summary.topRejectedGate },
  });

  // Individual interesting rows (opened, candidates, closest 10 by ratio)
  const interesting = [
    ...rows.filter(r => r.status === "OPENED" || r.status === "CANDIDATE"),
    ...[...rows].sort((a, b) => (b.signalScore / (b.requiredThreshold || 1)) - (a.signalScore / (a.requiredThreshold || 1))).slice(0, 10),
  ].slice(0, 25); // hard cap per tick

  for (const r of interesting) {
    events.push({
      event_id: makeId(tickAt, "STRATEGY_EVALUATED", String(r.strategyId)),
      created_at: new Date(tickAt).toISOString(),
      module: "btc_future_trading",
      account_key_suffix: suffix(accountKey),
      worker_owner: workerOwner,
      type: r.status === "OPENED" ? "POSITION_OPENED" : r.status === "CANDIDATE" ? "CANDIDATE_BUILT" : "SIGNAL_REJECTED",
      severity: r.status === "OPENED" ? "info" : r.status === "CANDIDATE" ? "info" : "warning",
      summary: `${r.strategyName} ${r.side ?? ""} score=${r.signalScore.toFixed(1)}/${r.requiredThreshold} gate=${r.gate}`,
      strategy_id: r.strategyId,
      strategy_name: r.strategyName,
      side: r.side,
      signal_score: r.signalScore,
      required_threshold: r.requiredThreshold,
      gate: r.gate,
      reject_reason: r.reason,
    });
  }

  return events;
}

/** POSITION_OPENED from worker/browser open. */
export function eventFromOpenedPosition(
  tickAt: number,
  pos: { id: string; strategyId: number; strategyName: string; side: "LONG" | "SHORT"; notional: number },
  workerOwner: "browser" | "vps" | "cron" | null,
  accountKey?: string | null,
): VerificationTrackEvent {
  return {
    event_id: makeId(tickAt, "POSITION_OPENED", pos.id),
    created_at: new Date(tickAt).toISOString(),
    module: "btc_future_trading",
    account_key_suffix: suffix(accountKey),
    worker_owner: workerOwner,
    type: "POSITION_OPENED",
    severity: "info",
    summary: `Opened ${pos.side} ${pos.strategyName} notional=${pos.notional}`,
    strategy_id: pos.strategyId,
    strategy_name: pos.strategyName,
    side: pos.side,
    position_id: pos.id,
    snapshot: { notional: pos.notional },
  };
}

/** POSITION_CLOSED from closed trade. */
export function eventFromClosedTrade(
  tickAt: number,
  trade: BTCFuturesTrade,
  workerOwner: "browser" | "vps" | "cron" | null,
  accountKey?: string | null,
): VerificationTrackEvent {
  return {
    event_id: makeId(tickAt, "POSITION_CLOSED", trade.id),
    created_at: new Date(tickAt).toISOString(),
    module: "btc_future_trading",
    account_key_suffix: suffix(accountKey),
    worker_owner: workerOwner,
    type: "POSITION_CLOSED",
    severity: trade.netPnl >= 0 ? "info" : "warning",
    summary: `Closed ${trade.side} ${trade.strategyName} PnL=${trade.netPnl.toFixed(2)} reason=${trade.exitReason}`,
    strategy_id: trade.strategyId,
    strategy_name: trade.strategyName,
    side: trade.side,
    trade_id: trade.id,
    net_pnl: trade.netPnl,
    exit_reason: trade.exitReason,
  };
}

/** NO_TRADE_ROOT_CAUSE event. */
export function eventFromNoTradeRootCause(
  tickAt: number,
  result: NoTradeRootCauseResult,
  workerOwner: "browser" | "vps" | "cron" | null,
  accountKey?: string | null,
): VerificationTrackEvent {
  return {
    event_id: makeId(tickAt, "NO_TRADE_ROOT_CAUSE"),
    created_at: new Date(tickAt).toISOString(),
    module: "btc_future_trading",
    account_key_suffix: suffix(accountKey),
    worker_owner: workerOwner,
    type: "NO_TRADE_ROOT_CAUSE",
    severity: result.rootCause.includes("WORKER") || result.rootCause.includes("STATE") ? "danger" : "warning",
    summary: `Root cause: ${result.rootCause} — ${result.safeFix}`,
    dominant_blocker: result.rootCause,
    snapshot: { evidence: result.evidence, canOpenIfSignalQualifies: result.canOpenIfSignalQualifies },
  };
}

/** WORKER_HEALTH / CRON_BACKUP simple event. */
export function eventFromWorkerHealth(
  tickAt: number,
  stale: boolean,
  owner: string | null,
  workerOwner: "browser" | "vps" | "cron" | null,
  accountKey?: string | null,
): VerificationTrackEvent {
  return {
    event_id: makeId(tickAt, "WORKER_HEALTH"),
    created_at: new Date(tickAt).toISOString(),
    module: "btc_future_trading",
    account_key_suffix: suffix(accountKey),
    worker_owner: workerOwner,
    type: "WORKER_HEALTH",
    severity: stale ? "danger" : "info",
    summary: `Worker ${owner ?? "unknown"} ${stale ? "STALE" : "LIVE"}`,
    snapshot: { stale, owner },
  };
}

/** Generic error event. */
export function eventFromError(
  tickAt: number,
  message: string,
  workerOwner: "browser" | "vps" | "cron" | null,
  accountKey?: string | null,
): VerificationTrackEvent {
  return {
    event_id: makeId(tickAt, "ERROR"),
    created_at: new Date(tickAt).toISOString(),
    module: "btc_future_trading",
    account_key_suffix: suffix(accountKey),
    worker_owner: workerOwner,
    type: "ERROR",
    severity: "danger",
    summary: message.slice(0, 200),
  };
}
