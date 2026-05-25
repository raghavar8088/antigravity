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
 *   NEXT_PUBLIC_VERCEL_URL    — deployed app URL for kline fetch
 *   (+ any NEXT_PUBLIC_DESK_* policy vars you use in the browser)
 *
 * See .env.local.example for the full list.
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
} from "../src/lib/mongoTradesClient";
import {
  runPaperDeskPollTick,
  type PaperDeskWorkerContext,
  type WorkerPosition,
} from "../src/lib/paperDeskWorker/runPaperDeskPollTick";
import { isWorkerHeartbeatFresh, WORKER_LEASE_TTL_MS } from "../src/lib/paperDeskWorker/workerLease";
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

const BASE_URL = process.env.NEXT_PUBLIC_VERCEL_URL
  ? `https://${process.env.NEXT_PUBLIC_VERCEL_URL}`
  : process.env.VERCEL_URL
    ? `https://${process.env.VERCEL_URL}`
    : "http://localhost:3000";

// Stable worker ID for this process (survives re-polls, restarted by pm2 = new ID → clears stale lease)
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
      disabled_strategies: [],
      last_trade_at: 0,
      day_start_balance: INITIAL_BALANCE,
      day_start_date: 0,
      cleared_at: 0,
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

// ── Helpers ───────────────────────────────────────────────────────────────────

function sleep(ms: number) {
  return new Promise<void>((resolve) => setTimeout(resolve, ms));
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60_000)}m${Math.floor((ms % 60_000) / 1000)}s`;
}

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
    stratCooldowns = {}; // cooldowns don't survive across restarts
  }

  const rawTrades = await listTradesMongo({ accountKey: ACCOUNT_KEY, limit: 500 });
  lastTrades = rawTrades.map((t) => ({
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

  console.log(`[worker] Hydrated: balance=$${lastBalance.toFixed(2)} open=${lastPositions.length} trades=${lastTrades.length}`);

  while (running) {
    const tickStart = Date.now();
    tickCount++;

    // Acquire or renew lease
    const leaseGranted = await acquireWorkerLease(ACCOUNT_KEY, WORKER_ID, WORKER_LEASE_TTL_MS);
    if (!leaseGranted) {
      console.warn(`[worker] tick#${tickCount} lease denied — another worker may be active. Waiting…`);
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
      pauseEntries: initState?.pause_entries ?? false,
      clearedAt: initState?.cleared_at ?? 0,
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
          template_family: undefined,
        });
      } catch (err) {
        console.error("[worker] trade persist failed:", err);
      }
    }

    // Persist account state + heartbeat
    try {
      await upsertAccountState({
        account_key: ACCOUNT_KEY,
        balance: lastBalance,
        positions: lastPositions,
        pause_entries: false,
        disabled_strategies: [],
        last_trade_at: result.closedTrades.length > 0 ? Date.now() : (initState?.last_trade_at ?? 0),
        day_start_balance: initState?.day_start_balance ?? INITIAL_BALANCE,
        day_start_date: initState?.day_start_date ?? 0,
        cleared_at: initState?.cleared_at ?? 0,
        updated_at: new Date().toISOString(),
        worker_id: WORKER_ID,
        worker_last_poll_at: Date.now(),
        worker_owner: "vps",
      });
    } catch (err) {
      console.error("[worker] state save failed:", err);
      // Try heartbeat-only as fallback
      await workerHeartbeat(ACCOUNT_KEY, WORKER_ID).catch(() => {});
    }

    const tickMs = Date.now() - tickStart;
    const balanceDrift = Math.abs(lastBalance - INITIAL_BALANCE - lastTrades.filter(t => !t.strategyName?.toUpperCase().includes("BOOTSTRAP") && !t.strategyName?.toUpperCase().includes("PROBE")).reduce((s, t) => s + t.netPnl, 0));

    // One-line tick summary
    console.log(
      `[worker] tick#${tickCount} ${formatDuration(tickMs).padStart(5)} ` +
      `open=${result.positions.length} closed=${result.closedTrades.length} opened=${result.openedPositions.length} ` +
      `bal=$${lastBalance.toFixed(2)} regime=${result.regime} blocker=${result.error ?? "none"} drift=$${balanceDrift.toFixed(2)}`,
    );

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
