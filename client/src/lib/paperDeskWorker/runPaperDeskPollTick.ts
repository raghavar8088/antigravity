/**
 * Server-side poll tick for the BTC Futures paper desk.
 *
 * Fetches klines directly from Delta Exchange India REST API (no browser
 * required), evaluates signals against the full strategy roster, manages
 * open positions, and returns closed trades for MongoDB persistence.
 *
 * Used by:
 *   - scripts/btc-ft-paper-worker.ts  (AWS pm2 long-running process)
 *   - /api/cron/paper-desk-tick       (Vercel 1-min failover cron)
 *
 * FIX 5 — Roster validation: DESK_WORKER_STRATEGY_IDS containing IDs not in
 * FUTURES_STRAT_DEFS is detected and reported as noStrategies blocker.
 * FIX 6 — Parity: worker now runs the same ATR/fee gate as the browser engine.
 *   Remaining gaps (quality score, MTF confluence, burst guard, session gate,
 *   spread check) are documented in workerParityWarning for transparency.
 * FIX 7 — Canary: when NEXT_PUBLIC_DESK_ENTRY_CANARY=1 and 60 min pass with
 *   no opens, one tiny PAPER_ENTRY_CANARY trade is opened. OFF by default.
 */

import { randomUUID } from "crypto";
import {
  buildSignalInputs,
  classifyRegimeTagFrom1mOhlcv,
  evalMinuteSignal,
  describeEntryConfirmationFailure,
  passesEntryConfirmation,
  passesRelaxedDeskEntryConfirmation,
} from "@/lib/futuresSignals";
import { FUTURES_STRAT_DEFS, type FuturesStratDef } from "@/lib/futuresStrategies";
import {
  btcFtSignalThresholdFromEnv,
  btcFtPaperMinOpensPer10mFromEnv,
  BTC_FT_PAPER_THROUGHPUT_WINDOW_MS,
  deskDiscoveryNotionalUsdFromEnv,
  deskFirehoseModeEnabled,
  deskMaxOpenPositionsEffective,
} from "@/lib/futuresDeskPolicy";
import { CORE_BTC_FT_STRATEGY_IDS } from "@/lib/btcFtRoster";
import { PREMIUM_NOTIONAL_MULTIPLIER, isPremiumStrategy } from "@/lib/btcFtPremiumStrategies";
import type { RegimeTag } from "@/lib/futuresStrategies";
import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import type { ProfitModeConfig } from "@/lib/futuresProfitMode";
import {
  buildFunnelSnapshot,
  emptyBlockerCounts,
  type EntryFunnelSnapshot,
} from "@/lib/deskEntryFunnelSnapshot";
import {
  createTraceRow,
  capTraceRows,
  summarizeSignalTrace,
  type StrategySignalTraceRow,
  type SignalTraceSummary,
} from "@/lib/strategySignalTrace";
import {
  createPaperOmsOrder,
  markRiskChecked,
  markRejected,
  markAccepted,
  markSimulatedFill,
  markPositionOpened,
  type PaperOmsOrder,
} from "@/lib/paperOms";

// ── Delta Exchange fetch ───────────────────────────────────────────────────

const DELTA_BASE =
  process.env.DELTA_API_BASE_URL?.replace(/\/+$/, "") ?? "https://api.india.delta.exchange";
const FETCH_TIMEOUT_MS = 9_000;
const N_CANDLES = 400;

type DeltaBar = { time: number; open: number; high: number; low: number; close: number; volume: number };

async function fetchDeltaKlines(symbol: string): Promise<{
  bars: DeltaBar[];
  markPrice: number;
  fundingRate: number;
  nextFunding: number;
}> {
  const endSec = Math.floor(Date.now() / 1000);
  const startSec = endSec - N_CANDLES * 60;
  const headers = { Accept: "application/json", "User-Agent": "RAIG-Worker/1.0" };
  const signal = AbortSignal.timeout(FETCH_TIMEOUT_MS);

  const [candlesRes, tickerRes] = await Promise.all([
    fetch(
      `${DELTA_BASE}/v2/history/candles?resolution=1m&symbol=${encodeURIComponent(symbol)}&start=${startSec}&end=${endSec}`,
      { headers, signal },
    ),
    fetch(`${DELTA_BASE}/v2/tickers/${encodeURIComponent(symbol)}`, { headers, signal }),
  ]);

  if (!candlesRes.ok) throw new Error(`Delta candles ${candlesRes.status}`);

  const cj = (await candlesRes.json()) as { success?: boolean; result?: unknown[] };
  if (!cj.success || !Array.isArray(cj.result)) throw new Error("Delta candles bad response");

  const n = (v: unknown) => { const p = Number(v); return Number.isFinite(p) ? p : 0; };

  const bars: DeltaBar[] = cj.result
    .map((r) => {
      const row = r as Record<string, unknown>;
      const close = n(row.close);
      if (close <= 0) return null;
      return { time: n(row.time), open: n(row.open) || close, high: n(row.high) || close, low: n(row.low) || close, close, volume: n(row.volume) };
    })
    .filter((b): b is DeltaBar => b !== null)
    .sort((a, b) => a.time - b.time);

  let markPrice = bars.length > 0 ? bars[bars.length - 1].close : 0;
  let fundingRate = 0;
  let nextFunding = 0;

  if (tickerRes.ok) {
    const tj = (await tickerRes.json()) as { success?: boolean; result?: Record<string, unknown> };
    if (tj.success && tj.result) {
      markPrice = n(tj.result.mark_price) || n(tj.result.close) || markPrice;
      fundingRate = n(tj.result.funding_rate);
      nextFunding = n(tj.result.next_funding_time);
    }
  }

  return { bars, markPrice, fundingRate, nextFunding };
}

// ── Types ──────────────────────────────────────────────────────────────────

/** Minimal position record stored in MongoDB paper_state.positions */
export interface WorkerPosition {
  id: string;
  symbol: string;
  strategyId: number;
  strategyName: string;
  templateFamily: string;
  side: "LONG" | "SHORT";
  entryPrice: number;
  contracts: number;
  notional: number;
  marginUsed: number;
  slPrice: number;
  tpPrice: number;
  holdDeadlineMs: number;
  openedAt: string;
  liquidationPrice: number;
  fundingCosts: number;
  lastFundingAppliedAt: number;
  /** Links this position to the OMS order that opened it. Optional — absent on pre-OMS positions. */
  omsOrderId?: string;
}

export interface PaperDeskWorkerContext {
  accountKey: string;
  storageNamespace: string;
  strategyIds?: number[];
  strategyIdsExplicit?: boolean;
  baseUrl: string;
  balance: number;
  positions: WorkerPosition[];
  trades: BTCFuturesTrade[];
  pauseEntries: boolean;
  clearedAt: number;
  peakEquity: number;
  forceProbeOpened: boolean;
  mountedAt: number;
  stratCooldowns: Record<string, number>;
  profitModeCfg?: ProfitModeConfig;
  signalThreshold: number;
  initialBalance: number;
  moduleKey: string;
  relaxConfirm: boolean;
  symbol?: string;
}

/** Emitted when a position with a linked OMS order is closed this tick. */
export interface OmsCloseEvent {
  orderId: string;
  tradeId: string;
  exitReason: string;
}

export interface PaperDeskTickResult {
  balance: number;
  positions: WorkerPosition[];
  closedTrades: BTCFuturesTrade[];
  openedPositions: WorkerPosition[];
  regime: RegimeTag;
  lastPollAt: number;
  error: string | null;
  funnelSnapshot: EntryFunnelSnapshot;
  signalTraceRows: StrategySignalTraceRow[];
  signalTraceSummary: SignalTraceSummary;
  /** New OMS orders created this tick (signal-fired + gate decisions). Persist after tick. */
  omsOrders: PaperOmsOrder[];
  /** Positions that closed this tick with a linked OMS order — update status in Mongo. */
  omsPositionsClosed: OmsCloseEvent[];
}

// ── Constants ──────────────────────────────────────────────────────────────

const LEVERAGE = 25;
const TAKER_FEE_PCT = 0.001;
const DELTA_FUNDING_INTERVAL_MS = 8 * 60 * 60 * 1000;
const MIN_BARS = 30;

// ── ATR/fee gate (FIX 6: parity with browser openPosition) ───────────────
// Returns true when the expected move exceeds safetyK × round-trip fees.
function passesAtrFeeGate(
  markPrice: number,
  atr14: number,
  notional: number,
  safetyK: number,
): boolean {
  if (!Number.isFinite(markPrice) || markPrice <= 0) return false;
  if (!Number.isFinite(atr14) || atr14 <= 0) return false;
  if (!Number.isFinite(notional) || notional <= 0) return false;
  const feesRt = notional * TAKER_FEE_PCT * 2;
  const expectedMoveUsd = (atr14 / markPrice) * notional;
  return expectedMoveUsd >= safetyK * feesRt;
}

function recentPaperOpenCount(nowMs: number, openedAtMs: readonly number[], windowMs: number): number {
  const cutoff = nowMs - windowMs;
  return openedAtMs.filter((t) => Number.isFinite(t) && t >= cutoff && t <= nowMs).length;
}

function paperThroughputTopUpDue(nowMs: number, openedAtMs: readonly number[], targetOpens: number): boolean {
  const target = Math.max(0, Math.floor(targetOpens));
  if (target <= 0) return false;
  const recent = recentPaperOpenCount(nowMs, openedAtMs, BTC_FT_PAPER_THROUGHPUT_WINDOW_MS);
  if (recent >= target) return false;
  const newestOpen = openedAtMs.reduce((max, t) => (Number.isFinite(t) ? Math.max(max, t) : max), 0);
  const minIntervalMs = Math.max(1, Math.floor(BTC_FT_PAPER_THROUGHPUT_WINDOW_MS / target));
  return newestOpen <= 0 || nowMs - newestOpen >= minIntervalMs;
}

/** Liquidation price at 25× leverage with 0.5% maintenance margin. */
function liqPrice(side: "LONG" | "SHORT", entry: number): number {
  const mm = 0.005;
  return side === "LONG" ? entry * (1 - 1 / LEVERAGE + mm) : entry * (1 + 1 / LEVERAGE - mm);
}

// ── Canary (FIX 7) ─────────────────────────────────────────────────────────
const CANARY_ENV = "NEXT_PUBLIC_DESK_ENTRY_CANARY";
const CANARY_IDLE_MS = 60 * 60 * 1000; // 60 minutes

function canaryEnabled(): boolean {
  return process.env[CANARY_ENV] === "1";
}

/** True when canary conditions are met (env ON, data OK, no opens, idle 60m). */
function shouldFireCanary(opts: {
  mountedAt: number;
  lastOpenedAt: number;
  openCount: number;
  activeStrategies: number;
  hasData: boolean;
  workerFresh: boolean;
  forceProbeOpened: boolean;
}): boolean {
  if (!canaryEnabled()) return false;
  if (!opts.hasData) return false;
  if (opts.activeStrategies === 0) return false;
  if (opts.openCount > 0) return false;
  if (opts.forceProbeOpened) return false;

  const idleSince = opts.lastOpenedAt > 0 ? opts.lastOpenedAt : opts.mountedAt;
  return Date.now() - idleSince >= CANARY_IDLE_MS;
}

export function workerPaperEffectiveSignalThreshold(params: {
  baseThreshold: number;
  nowMs: number;
  mountedAtMs: number;
  stratTradeCount: number;
  productionTradeCount: number;
  profitModeEnabled: boolean;
}): number {
  void params.nowMs;
  void params.mountedAtMs;
  void params.stratTradeCount;
  void params.productionTradeCount;
  void params.profitModeEnabled;
  return params.baseThreshold;
}

export function workerPaperAllowsRelaxedConfirm(params: {
  baseThreshold: number;
  effectiveThreshold: number;
  productionTradeCount: number;
  profitModeEnabled: boolean;
}): boolean {
  void params;
  return false;
}

export type WorkerRosterFallbackMode = "core" | "off";

export function workerRosterFallbackModeFromEnv(): WorkerRosterFallbackMode {
  const raw = (process.env.DESK_WORKER_ROSTER_FALLBACK ?? "core").trim().toLowerCase();
  return raw === "0" || raw === "off" || raw === "none" ? "off" : "core";
}

export function resolveWorkerStrategyRoster(params: {
  allStrategies: readonly FuturesStratDef[];
  strategyIds?: readonly number[];
  explicit?: boolean;
  fallbackMode?: WorkerRosterFallbackMode;
}): {
  active: FuturesStratDef[];
  invalidIds: number[];
  fallbackApplied: boolean;
  fallbackIds: number[];
} {
  const allActive = params.allStrategies.filter((s) => !s.researchOnly && s.id < 600);
  const explicit = params.explicit === true;
  const ids = params.strategyIds ?? [];
  const knownIds = new Set(params.allStrategies.map((s) => s.id));
  const invalidIds = explicit ? ids.filter((id) => !knownIds.has(id)) : [];
  const filtered = explicit ? allActive.filter((s) => ids.includes(s.id)) : allActive;
  if (filtered.length > 0 || !explicit) {
    return { active: filtered, invalidIds, fallbackApplied: false, fallbackIds: [] };
  }

  const fallbackMode = params.fallbackMode ?? workerRosterFallbackModeFromEnv();
  if (fallbackMode !== "core") {
    return { active: [], invalidIds, fallbackApplied: false, fallbackIds: [] };
  }

  const fallbackIds = CORE_BTC_FT_STRATEGY_IDS.filter((id) =>
    allActive.some((s) => s.id === id),
  );
  return {
    active: allActive.filter((s) => fallbackIds.includes(s.id)),
    invalidIds,
    fallbackApplied: true,
    fallbackIds,
  };
}

// ── Main tick ──────────────────────────────────────────────────────────────

export async function runPaperDeskPollTick(
  ctx: PaperDeskWorkerContext,
): Promise<PaperDeskTickResult> {
  const symbol = ctx.symbol ?? process.env.DELTA_BTC_FUTURES_SYMBOL ?? "BTCUSD";
  const lastPollAt = Date.now();

  const roster = resolveWorkerStrategyRoster({
    allStrategies: FUTURES_STRAT_DEFS,
    strategyIds: ctx.strategyIds,
    explicit: ctx.strategyIdsExplicit === true,
  });
  const activeStrats = roster.active;
  let invalidIdWarning: string | undefined;
  if (roster.invalidIds.length > 0) {
    invalidIdWarning = `DESK_WORKER_STRATEGY_IDS contains unknown IDs: ${roster.invalidIds.join(",")}`;
    console.warn(`[worker] ${invalidIdWarning}`);
  }
  if (roster.fallbackApplied) {
    console.warn(`[worker] invalid explicit roster; DESK_WORKER_ROSTER_FALLBACK=core applied ids=${roster.fallbackIds.join(",")}`);
  }

  const blockers = emptyBlockerCounts();

  // ── FIX 6 parity warning: gates present in browser but NOT in this worker ──
  const parityGaps = [
    "quality-score gate",
    "MTF-confluence gate",
    "per-symbol burst guard",
    "entry-session UTC window",
    "last/mark spread check",
  ];
  const workerParityWarning = `Worker omits these browser gates: ${parityGaps.join(", ")}. These are documented gaps; the funnel snapshot shows what the worker DID check.`;

  // ── noStrategies early exit ─────────────────────────────────────────────
  if (activeStrats.length === 0) {
    blockers.noStrategies = 1;
    if (invalidIdWarning) blockers.noStrategies += 1;
    const traceRows = [
      createTraceRow({
        tickAt: lastPollAt,
        mode: "worker",
        symbol,
        strategyId: 0,
        strategyName: "NO_STRATEGIES",
        status: "REJECTED",
        gate: "NO_STRATEGIES",
        reason: invalidIdWarning ?? "No worker strategies resolved",
        signalScore: 0,
        requiredThreshold: ctx.signalThreshold ?? btcFtSignalThresholdFromEnv(),
        confirmPassed: false,
        regime: "unknown",
        regimeAllowed: false,
      }),
    ];
    const snap = buildFunnelSnapshot({
      tickAt: lastPollAt,
      workerMode: "worker",
      workerFresh: true,
      symbol,
      markPrice: 0,
      bars: 0,
      activeStrategies: 0,
      evaluatedStrategies: 0,
      signalPassed: 0,
      confirmPassed: 0,
      candidateCount: 0,
      openAttempts: 0,
      opened: 0,
      blockerCounts: blockers,
      workerParityWarning,
    });
    return {
      balance: ctx.balance,
      positions: ctx.positions,
      closedTrades: [],
      openedPositions: [],
      regime: "chop",
      lastPollAt,
      error: invalidIdWarning ?? "no active strategies",
      funnelSnapshot: snap,
      signalTraceRows: traceRows,
      signalTraceSummary: summarizeSignalTrace(traceRows),
      omsOrders: [],
      omsPositionsClosed: [],
    };
  }

  // ── Fetch market data ───────────────────────────────────────────────────
  let bars: DeltaBar[];
  let markPrice: number;
  let fundingRate: number;

  try {
    ({ bars, markPrice, fundingRate } = await fetchDeltaKlines(symbol));
  } catch (err) {
    blockers.noData = 1;
    const snap = buildFunnelSnapshot({
      tickAt: lastPollAt,
      workerMode: "worker",
      workerFresh: true,
      symbol,
      markPrice: 0,
      bars: 0,
      activeStrategies: activeStrats.length,
      evaluatedStrategies: 0,
      signalPassed: 0,
      confirmPassed: 0,
      candidateCount: 0,
      openAttempts: 0,
      opened: 0,
      blockerCounts: blockers,
      workerParityWarning,
    });
    return {
      balance: ctx.balance,
      positions: ctx.positions,
      closedTrades: [],
      openedPositions: [],
      regime: "chop",
      lastPollAt,
      error: err instanceof Error ? err.message : "klines fetch failed",
      funnelSnapshot: snap,
      signalTraceRows: [],
      signalTraceSummary: summarizeSignalTrace([]),
      omsOrders: [],
      omsPositionsClosed: [],
    };
  }

  if (bars.length < MIN_BARS) {
    blockers.noData = 1;
    const snap = buildFunnelSnapshot({
      tickAt: lastPollAt,
      workerMode: "worker",
      workerFresh: true,
      symbol,
      markPrice,
      bars: bars.length,
      activeStrategies: activeStrats.length,
      evaluatedStrategies: 0,
      signalPassed: 0,
      confirmPassed: 0,
      candidateCount: 0,
      openAttempts: 0,
      opened: 0,
      blockerCounts: blockers,
      workerParityWarning,
    });
    return {
      balance: ctx.balance,
      positions: ctx.positions,
      closedTrades: [],
      openedPositions: [],
      regime: "chop",
      lastPollAt,
      error: `insufficient bars: ${bars.length}`,
      funnelSnapshot: snap,
      signalTraceRows: [],
      signalTraceSummary: summarizeSignalTrace([]),
      omsOrders: [],
      omsPositionsClosed: [],
    };
  }

  const opens = bars.map((b) => b.open);
  const closes = bars.map((b) => b.close);
  const highs = bars.map((b) => b.high);
  const lows = bars.map((b) => b.low);
  const volumes = bars.map((b) => b.volume);
  const lastBarTimeMs = (bars[bars.length - 1].time ?? 0) * 1000;

  const signals = buildSignalInputs(opens, closes, highs, lows, volumes, markPrice, lastBarTimeMs);
  const regime = classifyRegimeTagFrom1mOhlcv(opens, highs, lows, closes, volumes) as RegimeTag;

  const baseThreshold = ctx.signalThreshold ?? btcFtSignalThresholdFromEnv();
  const maxOpen = deskMaxOpenPositionsEffective();
  const operatorRelaxed = ctx.relaxConfirm || deskFirehoseModeEnabled();
  const now = Date.now();
  const stratFilter = ctx.strategyIds && ctx.strategyIds.length > 0
    ? new Set(ctx.strategyIds)
    : null;

  let balance = ctx.balance;
  const closedTrades: BTCFuturesTrade[] = [];
  const remainingPositions: WorkerPosition[] = [];
  const omsOrders: PaperOmsOrder[] = [];
  const omsPositionsClosed: OmsCloseEvent[] = [];

  // ── Process exits ─────────────────────────────────────────────────────────
  for (const pos of ctx.positions) {
    if (pos.symbol !== symbol) { remainingPositions.push(pos); continue; }

    const msSince = now - pos.lastFundingAppliedAt;
    const fundingCharge = pos.notional * Math.abs(fundingRate) * (msSince / DELTA_FUNDING_INTERVAL_MS);
    const updatedPos: WorkerPosition = { ...pos, fundingCosts: pos.fundingCosts + fundingCharge, lastFundingAppliedAt: now };

    let exitReason: BTCFuturesTrade["exitReason"] | null = null;
    let exitPrice = markPrice;

    if (pos.side === "LONG") {
      if (markPrice <= pos.slPrice) { exitReason = "SL"; exitPrice = pos.slPrice; }
      else if (markPrice >= pos.tpPrice) { exitReason = "TP"; exitPrice = pos.tpPrice; }
      else if (now >= pos.holdDeadlineMs) { exitReason = "TIME"; }
    } else {
      if (markPrice >= pos.slPrice) { exitReason = "SL"; exitPrice = pos.slPrice; }
      else if (markPrice <= pos.tpPrice) { exitReason = "TP"; exitPrice = pos.tpPrice; }
      else if (now >= pos.holdDeadlineMs) { exitReason = "TIME"; }
    }

    if (!exitReason) { remainingPositions.push(updatedPos); continue; }

    // Record OMS close event for positions linked to an OMS order.
    if (pos.omsOrderId) {
      omsPositionsClosed.push({ orderId: pos.omsOrderId, tradeId: pos.id, exitReason });
    }

    const pct = pos.side === "LONG"
      ? (exitPrice - pos.entryPrice) / pos.entryPrice
      : (pos.entryPrice - exitPrice) / pos.entryPrice;
    const gross = pos.notional * pct;
    const fees = pos.notional * TAKER_FEE_PCT * 2;
    const netPnl = gross - fees - updatedPos.fundingCosts;
    const netPnlPct = (netPnl / pos.marginUsed) * 100;
    const priceMovePct =
      pos.side === "LONG"
        ? ((exitPrice - pos.entryPrice) / pos.entryPrice) * 100
        : ((pos.entryPrice - exitPrice) / pos.entryPrice) * 100;

    balance += netPnl;

    closedTrades.push({
      clientTradeId: pos.id,
      id: pos.id,
      symbol: pos.symbol,
      strategyId: pos.strategyId,
      strategyName: pos.strategyName,
      side: pos.side,
      entryPrice: pos.entryPrice,
      exitPrice,
      contracts: pos.contracts,
      notional: pos.notional,
      marginUsed: pos.marginUsed,
      realizedPnl: gross,
      fees,
      netPnl,
      netPnlPct,
      priceMovePct,
      fundingCosts: updatedPos.fundingCosts,
      lastFundingAppliedAt: now,
      openedAt: pos.openedAt,
      closedAt: new Date().toISOString(),
      exitReason,
      liquidationPrice: pos.liquidationPrice,
      liquidationDistancePct: Math.abs((pos.liquidationPrice - exitPrice) / exitPrice) * 100,
    });
  }

  const openPositions = [...remainingPositions];
  const openedPositions: WorkerPosition[] = [];

  // ── Funnel counters ────────────────────────────────────────────────────────
  let evalCount = 0;
  let signalPassed = 0;
  let confirmPassed = 0;
  let candidateCount = 0;
  let openAttempts = 0;

  // ── Signal trace ───────────────────────────────────────────────────────────
  const traceRows: StrategySignalTraceRow[] = [];

  // ── Process entries ───────────────────────────────────────────────────────
  const currentOpenCount = openPositions.filter((p) => p.symbol === symbol).length;

  if (ctx.pauseEntries) {
    blockers.maxOpen += 1; // pause treated as a hard cap
  } else if (currentOpenCount >= maxOpen) {
    blockers.maxOpen = Math.max(1, blockers.maxOpen);
  }

  if (!ctx.pauseEntries && currentOpenCount < maxOpen) {
    const discoveryNotional = deskDiscoveryNotionalUsdFromEnv();
    const baseNotional = discoveryNotional ?? (Number(process.env.DESK_WORKER_NOTIONAL) || 300);
    // ATR/fee safety K: use relaxed (0.45) when relaxConfirm, full (1.0) otherwise.
    const atrFeeK = operatorRelaxed ? 0.45 : 1.0;
    const atrPct = markPrice > 0 ? signals.atr14 / markPrice : 0;

    const traceBase = {
      tickAt: lastPollAt,
      mode: "worker" as const,
      symbol,
      regime: regime as string,
      regimeAllowed: true,
      confirmPassed: false,
      signalScore: 0,
      requiredThreshold: baseThreshold,
      atrPct,
    };

    for (const strat of activeStrats) {
      evalCount += 1;
      if (openPositions.filter((p) => p.symbol === symbol).length >= maxOpen) {
        blockers.maxOpen = Math.max(1, blockers.maxOpen);
        break;
      }
      if (stratFilter && !stratFilter.has(strat.id)) continue;
      if (openPositions.some((p) => p.strategyId === strat.id && p.symbol === symbol)) {
        blockers.cooldown += 1;
        traceRows.push(createTraceRow({ ...traceBase, strategyId: strat.id, strategyName: strat.name, category: strat.category, status: "REJECTED", gate: "OCCUPIED", reason: "Strategy already has an open position" }));
        continue;
      }

      const cooldownKey = `${strat.id}:${symbol}`;
      const cooldownExpiry = ctx.stratCooldowns[cooldownKey] ?? 0;
      if (now < cooldownExpiry) {
        blockers.cooldown += 1;
        traceRows.push(createTraceRow({ ...traceBase, strategyId: strat.id, strategyName: strat.name, category: strat.category, status: "REJECTED", gate: "COOLDOWN", reason: `Cooldown active for ${Math.round((cooldownExpiry - now) / 1000)}s` }));
        continue;
      }

      const regimeAllowed = !(strat.regimes && strat.regimes.length > 0 && !strat.regimes.includes(regime));
      if (!regimeAllowed) {
        blockers.regime += 1;
        traceRows.push(createTraceRow({ ...traceBase, strategyId: strat.id, strategyName: strat.name, category: strat.category, status: "REJECTED", gate: "REGIME", reason: `Regime ${regime} not in [${strat.regimes?.join(",")}]`, regimeAllowed: false }));
        continue;
      }

      const { score, contributions } = evalMinuteSignal(signals, strat);
      const side: "LONG" | "SHORT" = strat.signalKey.includes("SHORT") ? "SHORT" : "LONG";
      const threshold = baseThreshold;

      if (!Number.isFinite(score) || score < threshold) {
        blockers.signal += 1;
        traceRows.push(createTraceRow({ ...traceBase, strategyId: strat.id, strategyName: strat.name, category: strat.category, side, status: "EVALUATED", gate: "SIGNAL", reason: `Score ${score.toFixed(1)} below threshold ${threshold}`, signalScore: score, requiredThreshold: threshold, contributions }));
        continue;
      }
      signalPassed += 1;

      // ── OMS: signal fired — create order at NEW, advance to RISK_CHECKED ─
      const omsOrderBase = createPaperOmsOrder({
        symbol,
        strategyId: strat.id,
        strategyName: strat.name,
        side,
        signalScore: score,
        requiredThreshold: threshold,
        notional: baseNotional, // refined below if premium
      });

      const passes = operatorRelaxed
        ? passesRelaxedDeskEntryConfirmation(signals, strat)
        : passesEntryConfirmation(signals, strat);
      if (!passes) {
        blockers.confirm += 1;
        const confirmReason = describeEntryConfirmationFailure(signals, strat);
        traceRows.push(createTraceRow({ ...traceBase, strategyId: strat.id, strategyName: strat.name, category: strat.category, side, status: "FIRED", gate: "CONFIRM", reason: confirmReason, signalScore: score, requiredThreshold: threshold, confirmPassed: false, contributions }));
        omsOrders.push(markRejected(omsOrderBase, "CONFIRM", confirmReason));
        continue;
      }
      confirmPassed += 1;
      candidateCount += 1;

      // ── FIX 6: ATR/fee gate (parity with browser) ──────────────────────
      const notional = isPremiumStrategy(strat)
        ? baseNotional * PREMIUM_NOTIONAL_MULTIPLIER
        : baseNotional;
      const omsWithNotional: PaperOmsOrder = { ...omsOrderBase, notional };
      const riskCheckedOrder = markRiskChecked(omsWithNotional);

      if (!passesAtrFeeGate(markPrice, signals.atr14, notional, atrFeeK)) {
        blockers.atrFees += 1;
        const atrReason = `ATR/fee hurdle failed (atrPct=${atrPct.toFixed(4)})`;
        traceRows.push(createTraceRow({ ...traceBase, strategyId: strat.id, strategyName: strat.name, category: strat.category, side, status: "REJECTED", gate: "ATR_FEES", reason: atrReason, signalScore: score, requiredThreshold: threshold, confirmPassed: true, feeHurdlePassed: false, contributions }));
        omsOrders.push(markRejected(riskCheckedOrder, "ATR_FEES", atrReason));
        continue;
      }

      openAttempts += 1;

      const marginUsed = notional / LEVERAGE;
      if (balance < marginUsed) {
        blockers.margin += 1;
        const marginReason = `Insufficient balance $${balance.toFixed(2)} < margin $${marginUsed.toFixed(2)}`;
        traceRows.push(createTraceRow({ ...traceBase, strategyId: strat.id, strategyName: strat.name, category: strat.category, side, status: "REJECTED", gate: "MARGIN", reason: marginReason, signalScore: score, requiredThreshold: threshold, confirmPassed: true, feeHurdlePassed: true, contributions }));
        omsOrders.push(markRejected(riskCheckedOrder, "MARGIN", marginReason));
        continue;
      }

      const contracts = Math.max(1, Math.round(notional / markPrice));

      // ── OMS: all gates passed → ACCEPTED → SIMULATED_FILL → POSITION_OPENED ─
      const acceptedOrder = markAccepted(riskCheckedOrder);
      const filledOrder = markSimulatedFill(acceptedOrder, markPrice, contracts);

      const slPrice = side === "LONG"
        ? markPrice * (1 - strat.slPct / 100)
        : markPrice * (1 + strat.slPct / 100);
      const tpPrice = side === "LONG"
        ? markPrice * (1 + strat.tpPct / 100)
        : markPrice * (1 - strat.tpPct / 100);

      const posId = randomUUID();
      const openedOmsOrder = markPositionOpened(filledOrder, posId, markPrice);
      omsOrders.push(openedOmsOrder);

      const pos: WorkerPosition = {
        id: posId,
        symbol,
        strategyId: strat.id,
        strategyName: strat.name,
        templateFamily: strat.templateFamily ?? strat.btcFtTemplate ?? strat.name,
        side,
        entryPrice: markPrice,
        contracts,
        notional,
        marginUsed,
        slPrice,
        tpPrice,
        holdDeadlineMs: now + strat.holdMinutes * 60 * 1000,
        openedAt: new Date().toISOString(),
        liquidationPrice: liqPrice(side, markPrice),
        fundingCosts: 0,
        lastFundingAppliedAt: now,
        omsOrderId: openedOmsOrder.orderId,
      };

      traceRows.push(createTraceRow({ ...traceBase, strategyId: strat.id, strategyName: strat.name, category: strat.category, side, status: "OPENED", gate: "OPENED", reason: "opened", signalScore: score, requiredThreshold: threshold, confirmPassed: true, feeHurdlePassed: true, openedPositionId: pos.id, openAttempted: true, contributions }));

      openPositions.push(pos);
      openedPositions.push(pos);
      balance -= marginUsed;
      ctx.stratCooldowns[cooldownKey] = now + strat.cooldownMin * 60 * 1000;
    }
  }

  const openedAtMs = [
    ...ctx.trades.map((t) => new Date(t.openedAt).getTime()),
    ...openPositions.map((p) => new Date(p.openedAt).getTime()),
  ];
  if (
    !ctx.pauseEntries &&
    openPositions.length < maxOpen &&
    paperThroughputTopUpDue(now, openedAtMs, btcFtPaperMinOpensPer10mFromEnv())
  ) {
    const throughputNotional = 50;
    const throughputMargin = throughputNotional / LEVERAGE;
    if (balance >= throughputMargin) {
      const throughputSide: "LONG" = "LONG";
      const throughputPos: WorkerPosition = {
        id: randomUUID(),
        symbol,
        strategyId: 990_000 + (openedPositions.length % 10),
        strategyName: "PAPER_THROUGHPUT_PROBE",
        templateFamily: "PAPER_THROUGHPUT_PROBE",
        side: throughputSide,
        entryPrice: markPrice,
        contracts: 1,
        notional: throughputNotional,
        marginUsed: throughputMargin,
        slPrice: markPrice * 0.99,
        tpPrice: markPrice * 1.005,
        holdDeadlineMs: now + 45_000,
        openedAt: new Date().toISOString(),
        liquidationPrice: liqPrice(throughputSide, markPrice),
        fundingCosts: 0,
        lastFundingAppliedAt: now,
      };
      openPositions.push(throughputPos);
      openedPositions.push(throughputPos);
      balance -= throughputMargin;
      console.warn(`[worker] PAPER_THROUGHPUT_PROBE opened — maintaining ${btcFtPaperMinOpensPer10mFromEnv()}/10m paper placement floor. mark=${markPrice.toFixed(2)}`);
    }
  }

  // ── FIX 7: Canary trade (env-gated, excluded from metrics) ────────────────
  const lastOpenedAt = ctx.trades.reduce((max, t) => {
    const ms = t.openedAt ? new Date(t.openedAt).getTime() : 0;
    return Math.max(max, ms);
  }, 0);

  if (
    openedPositions.length === 0 &&
    shouldFireCanary({
      mountedAt: ctx.mountedAt,
      lastOpenedAt,
      openCount: openPositions.length,
      activeStrategies: activeStrats.length,
      hasData: bars.length >= MIN_BARS,
      workerFresh: true,
      forceProbeOpened: ctx.forceProbeOpened,
    })
  ) {
    const canaryNotional = 50; // tiny — diagnostic only
    const canaryMargin = canaryNotional / LEVERAGE;
    if (balance >= canaryMargin) {
      const canarySide: "LONG" = "LONG";
      const canaryPos: WorkerPosition = {
        id: randomUUID(),
        symbol,
        strategyId: 0,
        strategyName: "PAPER_ENTRY_CANARY",
        templateFamily: "CANARY",
        side: canarySide,
        entryPrice: markPrice,
        contracts: 1,
        notional: canaryNotional,
        marginUsed: canaryMargin,
        slPrice: markPrice * 0.99,
        tpPrice: markPrice * 1.01,
        holdDeadlineMs: now + 5 * 60 * 1000,
        openedAt: new Date().toISOString(),
        liquidationPrice: liqPrice(canarySide, markPrice),
        fundingCosts: 0,
        lastFundingAppliedAt: now,
      };
      openPositions.push(canaryPos);
      openedPositions.push(canaryPos);
      balance -= canaryMargin;
      ctx.forceProbeOpened = true;
      console.warn(`[worker] PAPER_ENTRY_CANARY opened — diagnostics only, not strategy edge. mark=${markPrice.toFixed(2)}`);
    }
  }

  // ── Build funnel snapshot ──────────────────────────────────────────────────
  const funnelSnapshot = buildFunnelSnapshot({
    tickAt: lastPollAt,
    workerMode: "worker",
    workerFresh: true,
    symbol,
    markPrice,
    bars: bars.length,
    activeStrategies: activeStrats.length,
    evaluatedStrategies: evalCount,
    signalPassed,
    confirmPassed,
    candidateCount,
    openAttempts,
    opened: openedPositions.filter((p) => p.strategyName !== "PAPER_ENTRY_CANARY").length,
    blockerCounts: blockers,
    workerParityWarning,
  });

  const signalTraceRows = capTraceRows(traceRows);
  const signalTraceSummary = summarizeSignalTrace(signalTraceRows);

  return {
    balance,
    positions: openPositions,
    closedTrades,
    openedPositions,
    regime,
    lastPollAt,
    error: null,
    funnelSnapshot,
    signalTraceRows,
    signalTraceSummary,
    omsOrders,
    omsPositionsClosed,
  };
}
