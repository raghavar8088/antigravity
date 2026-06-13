/**
 * Pure builders that convert existing diagnostic snapshots into VerificationTrackEvent records.
 * No side effects, no I/O, no trading logic changes.
 */

import type { VerificationTrackEvent, VerificationTrackEventType } from "./types";
import type { EntryFunnelSnapshot } from "@/lib/trading/deskEntryFunnelSnapshot";
import type { StrategySignalTraceRow, SignalTraceSummary } from "@/lib/ai/strategySignalTrace";
import type { NoTradeRootCauseResult } from "@/lib/risk/noTradeRootCause";
import type { BTCFuturesTrade } from "@/lib/trading/btcFuturesTrade.types";

type LegacyOmsOrder = {
  status?: string;
  reject_gate?: string;
};

function summarizeOmsOrders(orders: LegacyOmsOrder[]) {
  const countsByStatus: Record<string, number> = {};
  for (const order of orders) {
    const key = order.status ?? "UNKNOWN";
    countsByStatus[key] = (countsByStatus[key] ?? 0) + 1;
  }
  return {
    total: orders.length,
    countsByStatus,
    topRejectGate: orders.find((o) => o.reject_gate)?.reject_gate ?? null,
    fillRate: orders.length > 0 ? (countsByStatus.POSITION_OPENED ?? 0) / orders.length : 0,
  };
}

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

/**
 * OMS summary events for a tick — one summary per OMS_ORDER_NEW, OMS_ORDER_REJECTED,
 * OMS_ORDER_FILLED, OMS_POSITION_OPENED batches. Emitted once per tick rather than
 * once per order to avoid flooding the event collection.
 */
export function eventsFromOmsTick(
  tickAt: number,
  orders: LegacyOmsOrder[],
  workerOwner: "browser" | "vps" | "cron" | null,
  accountKey?: string | null,
): VerificationTrackEvent[] {
  if (orders.length === 0) return [];
  const summary = summarizeOmsOrders(orders);
  const events: VerificationTrackEvent[] = [];

  const base = {
    created_at: new Date(tickAt).toISOString(),
    module: "btc_future_trading" as const,
    account_key_suffix: suffix(accountKey),
    worker_owner: workerOwner,
  };

  // Overall OMS summary for this tick
  events.push({
    event_id: makeId(tickAt, "OMS_ORDER_NEW"),
    ...base,
    type: "OMS_ORDER_NEW" as const,
    severity: "info" as const,
    summary: `OMS tick: new=${summary.total} rejected=${summary.countsByStatus.REJECTED} filled=${summary.countsByStatus.SIMULATED_FILL + summary.countsByStatus.POSITION_OPENED} closed=${summary.countsByStatus.POSITION_CLOSED} topReject=${summary.topRejectGate ?? "none"}`,
    snapshot: { countsByStatus: summary.countsByStatus, topRejectGate: summary.topRejectGate, fillRate: summary.fillRate },
  });

  if (summary.countsByStatus.REJECTED > 0) {
    events.push({
      event_id: makeId(tickAt, "OMS_ORDER_REJECTED"),
      ...base,
      type: "OMS_ORDER_REJECTED" as const,
      severity: "warning" as const,
      summary: `OMS rejected=${summary.countsByStatus.REJECTED} topGate=${summary.topRejectGate ?? "none"}`,
      gate: summary.topRejectGate ?? undefined,
      snapshot: { rejected: summary.countsByStatus.REJECTED, topRejectGate: summary.topRejectGate },
    });
  }

  const filledCount = summary.countsByStatus.SIMULATED_FILL + summary.countsByStatus.POSITION_OPENED;
  if (filledCount > 0) {
    events.push({
      event_id: makeId(tickAt, "OMS_ORDER_FILLED"),
      ...base,
      type: "OMS_ORDER_FILLED" as const,
      severity: "info" as const,
      summary: `OMS filled=${filledCount} positions`,
      snapshot: { filled: filledCount },
    });
  }

  if (summary.countsByStatus.POSITION_OPENED > 0) {
    events.push({
      event_id: makeId(tickAt, "OMS_POSITION_OPENED"),
      ...base,
      type: "OMS_POSITION_OPENED" as const,
      severity: "info" as const,
      summary: `OMS positions opened=${summary.countsByStatus.POSITION_OPENED}`,
      snapshot: { opened: summary.countsByStatus.POSITION_OPENED },
    });
  }

  if (summary.countsByStatus.POSITION_CLOSED > 0) {
    events.push({
      event_id: makeId(tickAt, "OMS_POSITION_CLOSED"),
      ...base,
      type: "OMS_POSITION_CLOSED" as const,
      severity: "info" as const,
      summary: `OMS positions closed=${summary.countsByStatus.POSITION_CLOSED}`,
      snapshot: { closed: summary.countsByStatus.POSITION_CLOSED },
    });
  }

  return events;
}

/** REPLAY_WALKFORWARD_RUN summary event — emitted by the CLI or API after a replay+rank run. */
export function eventFromReplayWalkForwardRun(
  runAt: number,
  params: {
    days: number;
    barsProcessed: number;
    totalTrades: number;
    promoted: number;
    candlesLoaded?: number;
    coverageDays?: number;
    accountKey?: string | null;
  },
): VerificationTrackEvent {
  const coveragePct =
    params.candlesLoaded != null && params.days > 0
      ? ((params.candlesLoaded / (params.days * 1440)) * 100).toFixed(1)
      : null;

  return {
    event_id: makeId(runAt, "REPLAY_WALKFORWARD_RUN"),
    created_at: new Date(runAt).toISOString(),
    module: "btc_future_trading",
    account_key_suffix: suffix(params.accountKey),
    worker_owner: null,
    type: "REPLAY_WALKFORWARD_RUN",
    severity: "info",
    summary:
      `Replay+WF run: days=${params.days} bars=${params.barsProcessed} ` +
      `candles=${params.candlesLoaded ?? params.barsProcessed} ` +
      `coverage=${params.coverageDays?.toFixed(1) ?? "?"}d (${coveragePct ?? "?"}%) ` +
      `trades=${params.totalTrades} promoted=${params.promoted}`,
    snapshot: {
      days: params.days,
      barsProcessed: params.barsProcessed,
      candlesLoaded: params.candlesLoaded ?? params.barsProcessed,
      coverageDays: params.coverageDays,
      coveragePct: coveragePct != null ? Number(coveragePct) : null,
      totalTrades: params.totalTrades,
      promoted: params.promoted,
    },
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
