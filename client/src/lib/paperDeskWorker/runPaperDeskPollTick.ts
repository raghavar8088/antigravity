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
 */

import { randomUUID } from "crypto";
import {
  buildSignalInputs,
  classifyRegimeTagFrom1mOhlcv,
  evalMinuteSignal,
  passesEntryConfirmation,
  passesRelaxedDeskEntryConfirmation,
} from "@/lib/futuresSignals";
import { FUTURES_STRAT_DEFS, type FuturesStratDef } from "@/lib/futuresStrategies";
import {
  btcFtSignalThresholdFromEnv,
  deskFirehoseModeEnabled,
  deskMaxOpenPositionsEffective,
} from "@/lib/futuresDeskPolicy";
import { PREMIUM_NOTIONAL_MULTIPLIER, isPremiumStrategy } from "@/lib/btcFtPremiumStrategies";
import type { RegimeTag } from "@/lib/futuresStrategies";
import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import type { ProfitModeConfig } from "@/lib/futuresProfitMode";

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
}

export interface PaperDeskWorkerContext {
  accountKey: string;
  storageNamespace: string;
  strategyIds?: number[];
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

export interface PaperDeskTickResult {
  balance: number;
  positions: WorkerPosition[];
  closedTrades: BTCFuturesTrade[];
  openedPositions: WorkerPosition[];
  regime: RegimeTag;
  lastPollAt: number;
  error: string | null;
}

// ── Constants ──────────────────────────────────────────────────────────────

const LEVERAGE = 25;
const TAKER_FEE_PCT = 0.001;
const DELTA_FUNDING_INTERVAL_MS = 8 * 60 * 60 * 1000;
const MIN_BARS = 30;

const ACTIVE_STRATS: FuturesStratDef[] = FUTURES_STRAT_DEFS.filter(
  (s) => !s.researchOnly && s.id < 600,
);

// ── Math ───────────────────────────────────────────────────────────────────

function liqPrice(side: "LONG" | "SHORT", entry: number): number {
  const mm = 0.005;
  return side === "LONG" ? entry * (1 - 1 / LEVERAGE + mm) : entry * (1 + 1 / LEVERAGE - mm);
}

// ── Main tick ──────────────────────────────────────────────────────────────

export async function runPaperDeskPollTick(
  ctx: PaperDeskWorkerContext,
): Promise<PaperDeskTickResult> {
  const symbol = ctx.symbol ?? process.env.DELTA_BTC_FUTURES_SYMBOL ?? "BTCUSD";

  let bars: DeltaBar[];
  let markPrice: number;
  let fundingRate: number;

  const lastPollAt = Date.now();

  try {
    ({ bars, markPrice, fundingRate } = await fetchDeltaKlines(symbol));
  } catch (err) {
    return {
      balance: ctx.balance,
      positions: ctx.positions,
      closedTrades: [],
      openedPositions: [],
      regime: "chop",
      lastPollAt,
      error: err instanceof Error ? err.message : "klines fetch failed",
    };
  }

  if (bars.length < MIN_BARS) {
    return {
      balance: ctx.balance,
      positions: ctx.positions,
      closedTrades: [],
      openedPositions: [],
      regime: "chop",
      lastPollAt,
      error: `insufficient bars: ${bars.length}`,
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

  const threshold = ctx.signalThreshold ?? btcFtSignalThresholdFromEnv();
  const maxOpen = deskMaxOpenPositionsEffective();
  const useRelaxed = ctx.relaxConfirm || deskFirehoseModeEnabled();
  const now = Date.now();

  const stratFilter = ctx.strategyIds && ctx.strategyIds.length > 0
    ? new Set(ctx.strategyIds)
    : null;

  let balance = ctx.balance;
  const closedTrades: BTCFuturesTrade[] = [];
  const remainingPositions: WorkerPosition[] = [];

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

  // ── Process entries ───────────────────────────────────────────────────────
  if (!ctx.pauseEntries && openPositions.filter((p) => p.symbol === symbol).length < maxOpen) {
    for (const strat of ACTIVE_STRATS) {
      if (openPositions.filter((p) => p.symbol === symbol).length >= maxOpen) break;
      if (stratFilter && !stratFilter.has(strat.id)) continue;
      if (openPositions.some((p) => p.strategyId === strat.id && p.symbol === symbol)) continue;

      const cooldownKey = `${strat.id}:${symbol}`;
      const cooldownExpiry = ctx.stratCooldowns[cooldownKey] ?? 0;
      if (now < cooldownExpiry) continue;

      if (strat.regimes && strat.regimes.length > 0 && !strat.regimes.includes(regime)) continue;

      const { score } = evalMinuteSignal(signals, strat);
      if (!Number.isFinite(score) || score < threshold) continue;

      const passes = useRelaxed
        ? passesRelaxedDeskEntryConfirmation(signals, strat)
        : passesEntryConfirmation(signals, strat);
      if (!passes) continue;

      const side: "LONG" | "SHORT" = strat.signalKey.includes("SHORT") ? "SHORT" : "LONG";
      const baseNotional = Number(process.env.DESK_WORKER_NOTIONAL) || 300;
      const notional = isPremiumStrategy(strat) ? baseNotional * PREMIUM_NOTIONAL_MULTIPLIER : baseNotional;
      const marginUsed = notional / LEVERAGE;
      const contracts = Math.max(1, Math.round(notional / markPrice));

      const slPrice = side === "LONG"
        ? markPrice * (1 - strat.slPct / 100)
        : markPrice * (1 + strat.slPct / 100);
      const tpPrice = side === "LONG"
        ? markPrice * (1 + strat.tpPct / 100)
        : markPrice * (1 - strat.tpPct / 100);

      const pos: WorkerPosition = {
        id: randomUUID(),
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
      };

      openPositions.push(pos);
      openedPositions.push(pos);
      ctx.stratCooldowns[cooldownKey] = now + strat.cooldownMin * 60 * 1000;
    }
  }

  return {
    balance,
    positions: openPositions,
    closedTrades,
    openedPositions,
    regime,
    lastPollAt,
    error: null,
  };
}
