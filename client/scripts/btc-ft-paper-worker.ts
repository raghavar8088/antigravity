/**
 * BTC Futures paper desk — headless 24/7 VPS worker.
 *
 * Usage:
 *   cd client
 *   npx tsx scripts/btc-ft-paper-worker.ts
 *
 * Or via pm2 (recommended for 24/7):
 *   pm2 start ecosystem.worker.config.cjs --name btc-ft-worker
 *
 * Required env (copy from .env.local and add to VPS):
 *   DESK_WORKER_ACCOUNT_KEY   — same UUID the signed-in browser uses
 *   MONGODB_URI               — Atlas connection string
 *   MONGODB_DB                — database name (default: loop_trades)
 *   NEXT_PUBLIC_VERCEL_URL    — deployed app URL (with or without https://)
 *   (+ any NEXT_PUBLIC_DESK_* policy vars you use in the browser)
 *
 * See ecosystem.worker.config.cjs for the full env var list.
 */

import * as crypto from "crypto";

// ── Env: on AWS/pm2 pass vars via ecosystem config; locally use tsx --env-file=.env.local ──

import {
  isMongoConfigured,
  getAccountState,
  upsertAccountState,
  upsertTradeMongo,
  listTradesMongo,
  acquireWorkerLease,
  workerHeartbeat,
  releaseWorkerLease,
  insertWorkerEvent,
  type PaperStateDoc,
} from "../src/lib/mongoTradesClient";
import {
  runPaperDeskPollTick,
  type PaperDeskWorkerContext,
  type WorkerPosition,
} from "../src/lib/paperDeskWorker/runPaperDeskPollTick";
import { isWorkerHeartbeatFresh, WORKER_LEASE_TTL_MS } from "../src/lib/paperDeskWorker/workerLease";
import { normalizeBaseUrl } from "../src/lib/paperDeskWorker/workerHelpers";
import { profitModeFromEnv } from "../src/lib/futuresProfitMode";
import type { BTCFuturesTrade } from "../src/lib/btcFuturesTrade.types";

// ── Config ────────────────────────────────────────────────────────────────────

const ACCOUNT_KEY = process.env.DESK_WORKER_ACCOUNT_KEY?.trim() ?? "";
const STORAGE_NAMESPACE = process.env.DESK_WORKER_STORAGE_NAMESPACE ?? "btc_future_trading_v4";
const POLL_MS = Math.max(2000, Number(process.env.POLL_MS ?? 4000) || 4000);
const INITIAL_BALANCE = Math.max(100, Number(process.env.NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD ?? 1000) || 1000);
const SIGNAL_THRESHOLD = Math.max(18, Number(process.env.NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD ?? 26) || 26);
const RELAX_CONFIRM = process.env.NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM === "1";

const STRATEGY_IDS: number[] = (process.env.DESK_WORKER_STRATEGY_IDS ?? "")
  .split(",")
  .map((s) => parseInt(s.trim(), 10))
  .filter((n) => Number.isFinite(n) && n > 0);

// Handles both `app.vercel.app` and `https://app.vercel.app` without double-protocol
const BASE_URL = normalizeBaseUrl(
  process.env.NEXT_PUBLIC_VERCEL_URL ?? process.env.VERCEL_URL,
);

// Stable worker ID for this process (survives re-polls; restarted by pm2 = new ID → clears stale lease)
const WORKER_ID = `vps-${crypto.randomUUID().slice(0, 8)}`;

// ── Runtime state ──────────────────────────────────────────────────────────────

let running = true;
let tickCount = 0;
let lastPositions: WorkerPosition[] = [];
let lastBalance = INITIAL_BALANCE;
let lastTrades: BTCFuturesTrade[] = [];
let stratCooldowns: Record<string, number> = {};
let peakEquity = INITIAL_BALANCE;
let forceProbeOpened = false;
const mountedAt = Date.now();

/** Consecutive ticks where opened===0 (resets on any open). */
let consecutiveZeroOpenTicks = 0;
const NO_OPEN_WATCH_THRESHOLD = 30;

/** The `cleared_at` epoch we last saw — used to detect remote ledger resets. */
let lastClearedAt = 0;
/** Browser-managed disabled strategies; preserved across saves so resets don't clear them. */
let lastKnownDisabledStrategies: number[] = [];

// ── Helpers ───────────────────────────────────────────────────────────────────

function sleep(ms: number) {
  return new Promise<void>((resolve) => setTimeout(resolve, ms));
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60_000)}m${Math.floor((ms % 60_000) / 1000)}s`;
}

function mapRawTrade(t: Awaited<ReturnType<typeof listTradesMongo>>[number]): BTCFuturesTrade {
  return {
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
  } as BTCFuturesTrade;
}

/**
 * Apply a remote ledger reset: reload balance, positions, trades, and clear
 * in-process tracking so the next tick starts from the operator's chosen state.
 */
async function applyRemoteReset(liveState: PaperStateDoc, newClearedAt: number): Promise<void> {
  console.log(`[worker] repair/clear detected; local state reset to paper_state (cleared_at=${newClearedAt})`);
  void insertWorkerEvent({
    account_key: ACCOUNT_KEY,
    type: "repair_detected",
    severity: "info",
    message: `Repair/clear detected; local state reset (cleared_at=${newClearedAt})`,
    payload: { newClearedAt, workerId: WORKER_ID },
  });
  lastClearedAt = newClearedAt;
  lastPositions = (liveState.positions ?? []) as WorkerPosition[];
  lastBalance = liveState.balance ?? INITIAL_BALANCE;
  stratCooldowns = {};
  peakEquity = lastBalance;
  forceProbeOpened = false;

  // Reload trades — only keep those closed after the clear point
  try {
    const rawTrades = await listTradesMongo({ accountKey: ACCOUNT_KEY, limit: 500 });
    lastTrades = rawTrades
      .filter((t) => {
        const closedMs = t.closed_at ? new Date(t.closed_at).getTime() : 0;
        return closedMs >= newClearedAt;
      })
      .map(mapRawTrade);
    console.log(`[worker] reset: balance=$${lastBalance.toFixed(2)} open=${lastPositions.length} trades=${lastTrades.length}`);
  } catch (err) {
    console.error("[worker] reset trade reload failed:", err);
    lastTrades = [];
  }
}

// ── Graceful shutdown ─────────────────────────────────────────────────────────

async function shutdown(signal: string) {
  running = false;
  console.log(`\n[worker] ${signal} received — releasing lease and flushing state…`);
  try {
    await releaseWorkerLease(ACCOUNT_KEY, WORKER_ID);
    await upsertAccountState({
      account_key: ACCOUNT_KEY,
      balance: lastBalance,
      positions: lastPositions,
      pause_entries: false,
      disabled_strategies: lastKnownDisabledStrategies,
      last_trade_at: 0,
      day_start_balance: INITIAL_BALANCE,
      day_start_date: 0,
      cleared_at: lastClearedAt,
      updated_at: new Date().toISOString(),
      worker_id: null,
      worker_owner: null,
      worker_last_poll_at: null,
    });
    console.log("[worker] State flushed. Goodbye.");
  } catch (err) {
    console.error("[worker] Shutdown flush failed:", err);
  }
  process.exit(0);
}

process.on("SIGINT", () => void shutdown("SIGINT"));
process.on("SIGTERM", () => void shutdown("SIGTERM"));

// ── Main loop ─────────────────────────────────────────────────────────────────

async function main() {
  console.log("[worker] BTC futures paper worker starting");
  console.log(`[worker] account_key=${ACCOUNT_KEY || "(missing)"}`);
  console.log(`[worker] worker_id=${WORKER_ID}`);
  console.log(`[worker] base_url=${BASE_URL}`);
  console.log(`[worker] poll_ms=${POLL_MS}`);
  console.log(`[worker] strategy_ids=${STRATEGY_IDS.length > 0 ? STRATEGY_IDS.join(",") : "(all)"}`);

  if (!ACCOUNT_KEY) {
    console.error("[worker] FATAL: DESK_WORKER_ACCOUNT_KEY is required");
    process.exit(1);
  }

  if (!isMongoConfigured()) {
    console.error("[worker] FATAL: MONGODB_URI not set or invalid");
    process.exit(1);
  }

  // Initial hydration from MongoDB
  console.log("[worker] Hydrating state from MongoDB…");
  const initState = await getAccountState(ACCOUNT_KEY);
  if (initState) {
    lastBalance = initState.balance ?? INITIAL_BALANCE;
    lastPositions = (initState.positions ?? []) as WorkerPosition[];
    lastClearedAt = initState.cleared_at ?? 0;
    lastKnownDisabledStrategies = initState.disabled_strategies ?? [];
    stratCooldowns = {}; // cooldowns don't survive across restarts
  }

  const rawTrades = await listTradesMongo({ accountKey: ACCOUNT_KEY, limit: 500 });
  lastTrades = rawTrades.map(mapRawTrade);

  console.log(`[worker] Hydrated: balance=$${lastBalance.toFixed(2)} open=${lastPositions.length} trades=${lastTrades.length}`);

  // Emit worker_start event (non-fatal)
  void insertWorkerEvent({
    account_key: ACCOUNT_KEY,
    type: "worker_start",
    severity: "info",
    message: `Worker ${WORKER_ID} started. balance=$${lastBalance.toFixed(2)} open=${lastPositions.length} trades=${lastTrades.length}`,
    payload: { workerId: WORKER_ID, balance: lastBalance, openPositions: lastPositions.length },
  });

  while (running) {
    const tickStart = Date.now();
    tickCount++;

    // ── Read fresh operator state at the start of every tick ─────────────────
    // This lets the worker respect pause/clear/disable changes made in the browser
    // without requiring a restart.
    let liveState: PaperStateDoc | null = null;
    try {
      liveState = await getAccountState(ACCOUNT_KEY);
    } catch (err) {
      console.error(`[worker] tick#${tickCount} state read failed:`, err);
    }

    // Sync disabled strategies so we never overwrite browser-managed blocklist
    if (liveState?.disabled_strategies) {
      lastKnownDisabledStrategies = liveState.disabled_strategies;
    }

    // Detect remote ledger reset (operator clicked "Clear trades" in the UI)
    const remClearedAt = liveState?.cleared_at ?? 0;
    if (remClearedAt > lastClearedAt) {
      await applyRemoteReset(liveState!, remClearedAt);
    }

    // Acquire or renew lease
    const leaseGranted = await acquireWorkerLease(ACCOUNT_KEY, WORKER_ID, WORKER_LEASE_TTL_MS);
    if (!leaseGranted) {
      console.warn(`[worker] tick#${tickCount} lease denied — another worker may be active. Waiting…`);
      void insertWorkerEvent({
        account_key: ACCOUNT_KEY,
        type: "lease_denied",
        severity: "warning",
        message: `Tick #${tickCount}: lease denied — another worker may be active`,
        payload: { tickCount, workerId: WORKER_ID },
      });
      await sleep(POLL_MS);
      continue;
    }

    const profitModeCfg = profitModeFromEnv();
    const ctx: PaperDeskWorkerContext = {
      accountKey: ACCOUNT_KEY,
      storageNamespace: STORAGE_NAMESPACE,
      strategyIds: STRATEGY_IDS,
      baseUrl: BASE_URL,
      balance: lastBalance,
      positions: [...lastPositions],
      trades: [...lastTrades],
      // Always use the live value so a browser pause/resume takes effect immediately
      pauseEntries: liveState?.pause_entries ?? false,
      clearedAt: lastClearedAt,
      peakEquity,
      forceProbeOpened,
      mountedAt,
      stratCooldowns,
      profitModeCfg,
      signalThreshold: SIGNAL_THRESHOLD,
      initialBalance: INITIAL_BALANCE,
      moduleKey: STORAGE_NAMESPACE,
      relaxConfirm: RELAX_CONFIRM,
    };

    let result;
    try {
      result = await runPaperDeskPollTick(ctx);
    } catch (err) {
      console.error(`[worker] tick#${tickCount} poll error:`, err);
      void insertWorkerEvent({
        account_key: ACCOUNT_KEY,
        type: "worker_tick_error",
        severity: "error",
        message: `Tick #${tickCount} poll error: ${err instanceof Error ? err.message : String(err)}`,
        payload: { tickCount, workerId: WORKER_ID },
      });
      await sleep(POLL_MS);
      continue;
    }

    // Update local state
    lastBalance = result.balance;
    lastPositions = result.positions;
    forceProbeOpened = ctx.forceProbeOpened;
    peakEquity = ctx.peakEquity;
    stratCooldowns = ctx.stratCooldowns;

    // Persist closed trades
    for (const trade of result.closedTrades) {
      lastTrades.push(trade);
      try {
        await upsertTradeMongo({
          client_trade_id: trade.clientTradeId ?? trade.id,
          account_key: ACCOUNT_KEY,
          module_key: STORAGE_NAMESPACE,
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
      } catch (err) {
        console.error("[worker] trade persist failed:", err);
      }
    }

    // Persist account state + heartbeat
    // Preserve operator-managed fields (disabled_strategies, day_start_*) from liveState.
    try {
      await upsertAccountState({
        account_key: ACCOUNT_KEY,
        balance: lastBalance,
        positions: lastPositions,
        pause_entries: liveState?.pause_entries ?? false,
        disabled_strategies: lastKnownDisabledStrategies,
        last_trade_at: result.closedTrades.length > 0 ? Date.now() : (liveState?.last_trade_at ?? 0),
        day_start_balance: liveState?.day_start_balance ?? INITIAL_BALANCE,
        day_start_date: liveState?.day_start_date ?? 0,
        cleared_at: lastClearedAt,
        updated_at: new Date().toISOString(),
        worker_id: WORKER_ID,
        worker_last_poll_at: Date.now(),
        worker_owner: "vps",
        entry_funnel_snapshot: result.funnelSnapshot ?? undefined,
      });
    } catch (err) {
      console.error("[worker] state save failed:", err);
      // Try heartbeat-only as fallback
      await workerHeartbeat(ACCOUNT_KEY, WORKER_ID).catch(() => {});
    }

    // ── Funnel snapshot: track no-open-watch counter ─────────────────────────
    const funnel = result.funnelSnapshot;
    if (result.openedPositions.length > 0) {
      consecutiveZeroOpenTicks = 0;
    } else {
      consecutiveZeroOpenTicks++;
    }

    const tickMs = Date.now() - tickStart;
    const balanceDrift = Math.abs(
      lastBalance -
      INITIAL_BALANCE -
      lastTrades
        .filter((t) => !t.strategyName?.toUpperCase().includes("BOOTSTRAP") && !t.strategyName?.toUpperCase().includes("PROBE"))
        .reduce((s, t) => s + t.netPnl, 0),
    );

    // ── Funnel log line (every tick) ─────────────────────────────────────────
    const funnelLine = funnel
      ? ` funnel=[strats=${funnel.activeStrategies} sig=${funnel.signalPassed} cand=${funnel.candidateCount} open=${funnel.opened} blocker=${funnel.dominantBlocker}]`
      : "";

    console.log(
      `[worker] tick#${tickCount} ${formatDuration(tickMs).padStart(5)} ` +
      `open=${result.positions.length} closed=${result.closedTrades.length} opened=${result.openedPositions.length} ` +
      `bal=$${lastBalance.toFixed(2)} regime=${result.regime} blocker=${result.error ?? "none"} drift=$${balanceDrift.toFixed(2)}` +
      funnelLine +
      (liveState?.pause_entries ? " [PAUSED]" : ""),
    );

    // ── No-open-watch alert ───────────────────────────────────────────────────
    if (consecutiveZeroOpenTicks === NO_OPEN_WATCH_THRESHOLD) {
      const watchMsg = `[worker] NO-OPEN-WATCH: ${consecutiveZeroOpenTicks} consecutive ticks with 0 opens. ` +
        `dominantBlocker=${funnel?.dominantBlocker ?? "unknown"} ` +
        `recommendation="${funnel?.recommendation ?? "run with NEXT_PUBLIC_DESK_ENTRY_DEBUG=1 to diagnose"}"`;
      console.warn(watchMsg);
      void insertWorkerEvent({
        account_key: ACCOUNT_KEY,
        type: "worker_tick_error",
        severity: "warning",
        message: watchMsg,
        payload: {
          consecutiveZeroOpenTicks,
          dominantBlocker: funnel?.dominantBlocker,
          activeStrategies: funnel?.activeStrategies,
          regime: result.regime,
          tickCount,
        },
      });
    }

    if (result.error && result.error !== "none") {
      console.warn(`[worker] poll error: ${result.error}`);
    }

    // Sleep for remaining poll interval
    const elapsed = Date.now() - tickStart;
    const remaining = Math.max(0, POLL_MS - elapsed);
    if (remaining > 0) await sleep(remaining);
  }
}

main().catch((err) => {
  console.error("[worker] Fatal error:", err);
  process.exit(1);
});
