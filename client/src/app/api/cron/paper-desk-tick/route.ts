/**
 * GET /api/cron/paper-desk-tick
 *
 * Safety-net backup tick for the headless paper desk.
 * Runs every 1 minute via Vercel cron (vercel.json).
 *
 * Behaviour:
 *   - If the VPS worker's heartbeat is fresh (< 3 min) → skip, return { skipped: true }.
 *   - Otherwise run ONE runPaperDeskPollTick for DESK_WORKER_ACCOUNT_KEY.
 *
 * NOT a replacement for the 4-second VPS worker — Vercel cron fires at most
 * every 60 s and cold-starts add ~2-4 s latency. This is a last-resort guard
 * that keeps positions alive if the VPS host sleeps / crashes.
 *
 * Auth: Bearer CRON_SECRET (same pattern as /api/cron/rank-strategies).
 * Max duration: 25 s (Vercel hobby plan limit — set in route config below).
 */

import { NextResponse } from "next/server";
import {
  isMongoConfigured,
  getAccountState,
  upsertAccountState,
  upsertTradeMongo,
  listTradesMongo,
  insertWorkerEvent,
} from "@/lib/mongoTradesClient";
import { isWorkerHeartbeatFresh } from "@/lib/paperDeskWorker/workerLease";
import { runPaperDeskPollTick, type WorkerPosition } from "@/lib/paperDeskWorker/runPaperDeskPollTick";
import { profitModeFromEnv } from "@/lib/futuresProfitMode";
import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import { resolveAuthoritativeAccountKey, validateAccountKeyAlignment } from "@/lib/authoritativeAccountKey";
import { isEngineExecutionAuthority, ENGINE_AUTHORITY_SKIP_REASON } from "@/lib/engineAuthority";

export const dynamic = "force-dynamic";
export const maxDuration = 25;

const CRON_WORKER_ID = "cron-backup";

export async function GET(req: Request) {
  // Auth
  const authHeader = req.headers.get("authorization");
  const cronSecret = process.env.CRON_SECRET;
  if (cronSecret && authHeader !== `Bearer ${cronSecret}`) {
    return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
  }

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  if (isEngineExecutionAuthority()) {
    return NextResponse.json({
      ok: true,
      skipped: true,
      skippedReason: ENGINE_AUTHORITY_SKIP_REASON,
    });
  }

  const keyCheck = validateAccountKeyAlignment();
  if (!keyCheck.ok) {
    return NextResponse.json(
      { ok: false, error: keyCheck.errors.join("; ") },
      { status: 503 },
    );
  }

  const accountKey = resolveAuthoritativeAccountKey();

  // Load state first so we can check the stored heartbeat timestamp synchronously.
  const state = await getAccountState(accountKey);

  // Skip when VPS worker is actively heartbeating.
  if (isWorkerHeartbeatFresh(state?.worker_last_poll_at)) {
    void insertWorkerEvent({
      account_key: accountKey,
      type: "cron_skipped_worker_fresh",
      severity: "info",
      message: `Cron skipped — VPS worker fresh (last_poll_at=${state?.worker_last_poll_at})`,
      payload: { workerLastPollAt: state?.worker_last_poll_at },
    });
    return NextResponse.json({
      ok: true,
      tick: false,
      skippedReason: `worker_fresh: last_poll_at=${state?.worker_last_poll_at}`,
    });
  }

  // Load strategy IDs from env
  const strategyIdsRaw = process.env.DESK_WORKER_STRATEGY_IDS ?? "";
  const strategyIdsExplicit = strategyIdsRaw.trim() !== "";
  const strategyIds = strategyIdsRaw
    .split(",")
    .map((s) => parseInt(s.trim(), 10))
    .filter((n) => Number.isFinite(n) && n > 0);

  const initialBalance = Number(process.env.NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD ?? 1000) || 1000;

  // Load trades for context
  const rawTrades = await listTradesMongo({
    accountKey,
    limit: 500,
    moduleKey: process.env.DESK_WORKER_STORAGE_NAMESPACE ?? undefined,
  });

  const trades = rawTrades.map((t) => ({
    clientTradeId: t.client_trade_id ?? "",
    id: t.client_trade_id ?? "",
    symbol: t.symbol ?? "BTCUSD",
    strategyId: Number(t.strategy_id ?? 0),
    strategyName: t.strategy_name ?? "",
    side: (t.side ?? "LONG") as "LONG" | "SHORT",
    entryPrice: Number(t.entry_price ?? 0),
    exitPrice: Number(t.exit_price ?? 0),
    contracts: Number(t.contracts ?? 0),
    notional: Number(t.notional ?? 0),
    marginUsed: Number(t.margin_used ?? 0),
    realizedPnl: Number(t.gross_pnl ?? 0),
    fees: Number(t.fees ?? 0),
    netPnl: Number(t.net_pnl ?? 0),
    netPnlPct: 0,
    priceMovePct: 0,
    fundingCosts: Number(t.funding_costs ?? 0),
    lastFundingAppliedAt: 0,
    openedAt: t.opened_at ?? new Date().toISOString(),
    closedAt: t.closed_at ?? new Date().toISOString(),
    exitReason: (t.exit_reason ?? "TIME") as BTCFuturesTrade["exitReason"],
    liquidationPrice: 0,
    liquidationDistancePct: 0,
    signalContributions: [],
    fundingSinceOpenMs: undefined,
  })) as BTCFuturesTrade[];

  const ctx = {
    accountKey,
    storageNamespace: process.env.DESK_WORKER_STORAGE_NAMESPACE ?? "btc_future_trading_v4",
    strategyIds: strategyIdsExplicit ? strategyIds : undefined,
    strategyIdsExplicit,
    baseUrl: "",
    balance: state?.balance ?? initialBalance,
    positions: (state?.positions ?? []) as WorkerPosition[],
    trades,
    pauseEntries: state?.pause_entries ?? false,
    clearedAt: state?.cleared_at ?? 0,
    peakEquity: state?.balance ?? initialBalance,
    forceProbeOpened: false,
    mountedAt: Date.now(),
    stratCooldowns: {} as Record<string, number>,
    profitModeCfg: profitModeFromEnv(),
    signalThreshold: Number(process.env.NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD ?? 26) || 26,
    initialBalance,
    moduleKey: process.env.DESK_WORKER_STORAGE_NAMESPACE ?? "btc_future_trading_v4",
    relaxConfirm: process.env.NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM === "1",
  };

  void insertWorkerEvent({
    account_key: accountKey,
    type: "cron_backup_tick",
    severity: "info",
    message: "Cron backup tick fired — VPS worker was stale",
    payload: { workerLastPollAt: state?.worker_last_poll_at },
  });

  let result;
  try {
    result = await runPaperDeskPollTick(ctx);
  } catch (err) {
    void insertWorkerEvent({
      account_key: accountKey,
      type: "worker_tick_error",
      severity: "error",
      message: `Cron tick error: ${err instanceof Error ? err.message : String(err)}`,
    });
    return NextResponse.json({
      ok: false,
      error: err instanceof Error ? err.message : "poll_tick_failed",
    }, { status: 500 });
  }

  // Persist closed trades to MongoDB
  for (const trade of result.closedTrades) {
    await upsertTradeMongo({
      client_trade_id: trade.clientTradeId ?? trade.id,
      account_key: accountKey,
      module_key: ctx.moduleKey,
      symbol: trade.symbol,
      strategy_id: trade.strategyId,
      strategy_name: trade.strategyName,
      side: trade.side,
      entry_price: trade.entryPrice,
      exit_price: trade.exitPrice,
      contracts: trade.contracts,
      notional: trade.notional,
      margin_used: trade.marginUsed,
      gross_pnl: trade.realizedPnl,
      fees: trade.fees,
      net_pnl: trade.netPnl,
      funding_costs: trade.fundingCosts,
      exit_reason: trade.exitReason,
      opened_at: trade.openedAt,
      closed_at: trade.closedAt,
      payload: {
        netPnlPct: trade.netPnlPct,
        priceMovePct: trade.priceMovePct,
        liquidationPrice: trade.liquidationPrice,
        liquidationDistancePct: trade.liquidationDistancePct ?? 0,
      },
      exchange: "delta",
    });
  }

  // Persist updated state
  await upsertAccountState({
    account_key: accountKey,
    balance: result.balance,
    positions: result.positions,
    pause_entries: ctx.pauseEntries,
    disabled_strategies: state?.disabled_strategies ?? [],
    last_trade_at: result.closedTrades.length > 0 ? Date.now() : (state?.last_trade_at ?? 0),
    day_start_balance: state?.day_start_balance ?? initialBalance,
    day_start_date: state?.day_start_date ?? 0,
    cleared_at: state?.cleared_at ?? 0,
    updated_at: new Date().toISOString(),
    worker_id: CRON_WORKER_ID,
    worker_last_poll_at: Date.now(),
    worker_owner: "vps",
    entry_funnel_snapshot: result.funnelSnapshot,
    signal_trace_latest: {
      tickAt: result.lastPollAt,
      mode: "cron",
      symbol: result.funnelSnapshot.symbol ?? "BTCUSD",
      summary: result.signalTraceSummary,
      rows: result.signalTraceRows,
    },
  });

  return NextResponse.json({
    ok: true,
    tick: true,
    closed: result.closedTrades.length,
    opened: result.openedPositions.length,
    openPositions: result.positions.length,
    balance: result.balance,
    regime: result.regime,
    error: result.error ?? null,
  });
}
