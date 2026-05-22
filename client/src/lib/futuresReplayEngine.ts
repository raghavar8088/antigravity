/**
 * Deterministic offline replay of the BTC futures paper desk (one symbol, bar-by-bar).
 * Mirrors live hook order: mark → exits → entries (gates + sizing + slippage).
 */

import type { BTCFuturesTrade, FuturesTradeExitReason } from "@/lib/btcFuturesTrade.types";
import {
  buildPaperDeskStrategies,
  canOpenCategory,
  countOpenByCategory,
  deskEffectiveHoldMinutesAtOpen,
  isUtcHourInSession,
} from "@/lib/futuresDeskPolicy";
import {
  applyMarkToFuturesPosition,
  resolveFuturesExitStep,
  type FuturesExitStepPosition,
} from "@/lib/futuresDeskRuntime";
import {
  paperApplyEntrySlippage,
  paperApplyExitSlippage,
  paperContracts,
  paperEstimatedMaxLossAtStopSl,
  paperLiquidationDistancePct,
  paperLiquidationPrice,
  paperMarginRequired,
  paperMinExpectedMoveVsFees,
  paperNetPnlOnClose,
  paperNotional,
  paperNotionalForTargetRisk,
  paperPriceMovePctOnNotional,
  paperSameDirNotionalWouldExceedCap,
  type PaperSide,
} from "@/lib/futuresPaperMath";
import {
  deskPaperMakerFillModelEnabled,
  deskPaperMakerFeePctFromEnv,
  deskPaperMakerFillProbabilityFromEnv,
} from "@/lib/futuresDeskPolicy";
import { FUTURES_STRAT_DEFS, type FuturesStratDef, type RegimeTag } from "@/lib/futuresStrategies";
import {
  buildSignalInputs,
  classifyRegimeTagFrom1mOhlcv,
  effectiveSignalThreshold,
  evalMinuteSignal,
  passesEntryConfirmation,
} from "@/lib/futuresSignals";
import {
  FUTURES_STRATEGY_PROFILES,
  effectiveMinExpectedMoveSafetyK,
  resolveStrategyProfile,
  type FuturesStrategyProfile,
} from "@/lib/futuresSessionMetrics";
import type { ReplayCandle } from "@/lib/futuresReplayFixtures";

const RAW_TAKER_FEE_PCT = 0.001;

/**
 * Effective per-leg fee for paper math.
 *
 * When `deskPaperMakerFillModelEnabled()` is OFF (default 2026-05-21): equals
 * RAW_TAKER_FEE_PCT (0.10% per leg → 0.20% RT) — the conservative all-taker model.
 *
 * When ON (env `NEXT_PUBLIC_DESK_PAPER_MAKER_FILL_MODEL=1`): blended per-leg
 * fee = p_maker * makerFeePct + (1 - p_maker) * takerFeePct. With Delta's
 * 0.05% maker / 0.10% taker rates and 70% maker fill probability, drops to
 * ~0.065% per leg → ~0.13% RT. The live execution path must actually place
 * post-only orders with taker-fallback timeout for this model to hold.
 *
 * Captured at module load so the replay run is deterministic per env config.
 */
const TAKER_FEE_PCT: number = (() => {
  if (!deskPaperMakerFillModelEnabled()) return RAW_TAKER_FEE_PCT;
  const pMaker = deskPaperMakerFillProbabilityFromEnv();
  const makerFee = deskPaperMakerFeePctFromEnv();
  return pMaker * makerFee + (1 - pMaker) * RAW_TAKER_FEE_PCT;
})();
const CONTRACT_SIZE = 1;
const MIN_CONTRACTS = 1;
const MAX_CONTRACTS = 50;
const MIN_POSITION_NOTIONAL = 100;
const MAX_POSITION_NOTIONAL = 500;
const MIN_BARS_DEFAULT = 18;
const MAX_BARS_DEFAULT = 120;
const MAX_LOSS_PER_TRADE_PCT = 2;
const MIN_ABS_NET_PNL_USD = 2;
const SIGNAL_THRESHOLD_DEFAULT = 28;
/** Match `useBTCFuturesScalperEngine` drawdown entry pause. */
const REPLAY_MAX_DRAWDOWN_LOCK_PCT = 25;
const REPLAY_DRAWDOWN_LOCK_RECOVERY_FRAC = 0.84;

export type PaperReplayConfig = {
  initialBalance: number;
  leverage: number;
  slippageBps: number;
  strategyIds?: number[];
  maxPositions: number;
  barMs: number;
  symbol?: string;
  signalThreshold?: number;
  volSizedNotional?: boolean;
  riskPctOfEquity?: number;
  minExpectedMoveSafetyK?: number;
  maxSameDirFracOfEquity?: number;
  allowFakeDiversity?: boolean;
  minTpSlRatio?: number;
  fundingRate?: number;
  disabledStrategyIds?: number[];
  /** When true, pause new entries at 25% session peak drawdown (resume below 21%). */
  drawdownLock?: boolean;
  /** Per `FuturesStratDef.category` concurrent open cap (omit = no cap; golden tests unchanged). */
  maxOpenPerCategory?: number;
  /** UTC entry window (omit = always on; golden default). */
  sessionStartUtcHour?: number;
  sessionEndUtcHour?: number;
  strategyProfile?: FuturesStrategyProfile;
  minBars?: number;
  maxBars?: number;
};

export type PaperReplayStats = {
  count: number;
  sumNet: number;
  expectancy: number;
  exitReasonCounts: Record<string, number>;
};

export type PaperReplayResult = {
  trades: BTCFuturesTrade[];
  stats: PaperReplayStats;
  finalBalance: number;
  barsProcessed: number;
};

type ReplayPosition = FuturesExitStepPosition & {
  id: string;
  symbol: string;
  strategyId: number;
  strategyName: string;
  contracts: number;
  slPrice: number;
  leverage: number;
  initialMargin: number;
  /** Consecutive bars below the momentum-decay threshold. Resets on reversal. */
  momDecayBars: number;
};

/** Stable UUID-shaped id for golden snapshots (no crypto.randomUUID). */
export function replayClientTradeId(
  symbol: string,
  strategyId: number,
  openedAtMs: number,
  closedAtMs: number,
): string {
  const seed = `${symbol}:${strategyId}:${openedAtMs}:${closedAtMs}`;
  let h = 0x811c9dc5;
  for (let i = 0; i < seed.length; i++) {
    h ^= seed.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  const a = (h >>> 0).toString(16).padStart(8, "0");
  const b = ((h ^ 0x9e3779b9) >>> 0).toString(16).padStart(8, "0");
  const c = ((h ^ 0x85ebca6b) >>> 0).toString(16).padStart(4, "0");
  const d = ((h ^ 0xc2b2ae35) >>> 0).toString(16).padStart(12, "0");
  return `${a.slice(0, 8)}-${b.slice(0, 4)}-4${c.slice(1, 4)}-8${d.slice(0, 3)}-${d.slice(3, 15)}`;
}

export function summarizeReplayTrades(trades: ReadonlyArray<BTCFuturesTrade>): PaperReplayStats {
  const exitReasonCounts: Record<string, number> = {};
  let sumNet = 0;
  for (const t of trades) {
    sumNet += t.netPnl;
    exitReasonCounts[t.exitReason] = (exitReasonCounts[t.exitReason] ?? 0) + 1;
  }
  return {
    count: trades.length,
    sumNet,
    expectancy: trades.length > 0 ? sumNet / trades.length : 0,
    exitReasonCounts,
  };
}

function sliceWindow<T>(arr: T[], maxBars: number): T[] {
  if (arr.length <= maxBars) return arr;
  return arr.slice(arr.length - maxBars);
}

function closeReplayPosition(
  pos: ReplayPosition,
  exitPriceLevel: number,
  exitReason: FuturesTradeExitReason,
  closedAtMs: number,
  slippageBps: number,
): { trade: BTCFuturesTrade; marginReturn: number; netPnl: number } {
  const slippedExit = paperApplyExitSlippage(pos.side, exitPriceLevel, slippageBps);
  const { grossPnl, fees, netPnl } = paperNetPnlOnClose({
    entryPrice: pos.entryPrice,
    exitPrice: slippedExit,
    notional: pos.notional,
    side: pos.side,
    takerFeePct: TAKER_FEE_PCT,
    fundingCosts: pos.fundingCosts,
    minAbsNetWinUsd: MIN_ABS_NET_PNL_USD,
  });
  const netPnlPct = pos.marginUsed > 0 ? (netPnl / pos.marginUsed) * 100 : 0;
  const openedAtMs = new Date(pos.openedAt).getTime();
  const trade: BTCFuturesTrade = {
    clientTradeId: replayClientTradeId(pos.symbol, pos.strategyId, openedAtMs, closedAtMs),
    id: pos.id,
    symbol: pos.symbol,
    strategyId: pos.strategyId,
    strategyName: pos.strategyName,
    side: pos.side,
    entryPrice: pos.entryPrice,
    exitPrice: slippedExit,
    contracts: pos.contracts,
    notional: pos.notional,
    marginUsed: pos.marginUsed,
    realizedPnl: grossPnl,
    fees,
    netPnl,
    netPnlPct,
    priceMovePct: paperPriceMovePctOnNotional(pos.entryPrice, slippedExit, pos.side),
    fundingCosts: pos.fundingCosts,
    lastFundingAppliedAt: pos.lastFundingAppliedAt,
    fundingSinceOpenMs: Math.max(0, closedAtMs - openedAtMs),
    openedAt: pos.openedAt,
    closedAt: new Date(closedAtMs).toISOString(),
    exitReason,
    liquidationPrice: pos.liquidationPrice,
    liquidationDistancePct: paperLiquidationDistancePct(
      slippedExit,
      pos.liquidationPrice,
      pos.side,
    ),
  };
  return { trade, marginReturn: pos.marginUsed, netPnl };
}

export function runPaperDeskReplay(
  candles: ReplayCandle[],
  config: PaperReplayConfig,
): PaperReplayResult {
  const symbol = config.symbol ?? "BTCUSD";
  const minBars = config.minBars ?? MIN_BARS_DEFAULT;
  const maxBars = config.maxBars ?? MAX_BARS_DEFAULT;
  const leverage = config.leverage;
  const slippageBps = config.slippageBps;
  const fundingRate = config.fundingRate ?? 0;
  const profile = resolveStrategyProfile(config.strategyProfile);
  const profileCfg = FUTURES_STRATEGY_PROFILES[profile];
  const holdTimeMul = profileCfg.holdTimeMul;
  const signalThreshold = effectiveSignalThreshold(
    config.signalThreshold ?? SIGNAL_THRESHOLD_DEFAULT,
    profileCfg.signalThresholdDelta,
  );
  const disabled = new Set(config.disabledStrategyIds ?? []);

  const deskOpts = {
    strategyIdAllowlist: null,
    minTpSlRatio: config.minTpSlRatio ?? 2,
    allowFakeDiversity: config.allowFakeDiversity ?? false,
  };
  const allow = config.strategyIds?.length ? new Set(config.strategyIds) : null;
  const raw = allow
    ? FUTURES_STRAT_DEFS.filter((s) => allow.has(s.id))
    : [...FUTURES_STRAT_DEFS];
  const { strategies: activeStrats } = buildPaperDeskStrategies(raw, deskOpts);

  let balance = config.initialBalance;
  const positions: ReplayPosition[] = [];
  const trades: BTCFuturesTrade[] = [];
  const cooldownUntil: Record<string, number> = {};
  let peakEquityForDrawdown = config.initialBalance;
  let drawdownEntryPaused = false;

  const tryOpen = (
    strat: FuturesStratDef,
    side: PaperSide,
    lastPrice: number,
    markPrice: number,
    regime: RegimeTag,
    atr14: number,
    nowMs: number,
    equityUsd: number,
    entryBook: ReadonlyArray<{ side: PaperSide; notional: number }>,
  ): boolean => {
    if (disabled.has(strat.id)) return false;
    if (strat.regimes && strat.regimes.length > 0 && !strat.regimes.includes(regime)) return false;

    const entryPrice = paperApplyEntrySlippage(side, lastPrice, slippageBps);
    const slPrice =
      side === "LONG"
        ? entryPrice * (1 - strat.slPct / 100)
        : entryPrice * (1 + strat.slPct / 100);
    const tpPrice =
      side === "LONG"
        ? entryPrice * (1 + strat.tpPct / 100)
        : entryPrice * (1 - strat.tpPct / 100);

    const targetNotional = config.volSizedNotional
      ? paperNotionalForTargetRisk({
          equityUsd,
          atr14,
          markPrice,
          entryPrice,
          side,
          slPct: strat.slPct,
          takerFeePct: TAKER_FEE_PCT,
          riskPctOfEquity: config.riskPctOfEquity ?? 0.01,
          minNotional: MIN_POSITION_NOTIONAL,
          maxNotional: MAX_POSITION_NOTIONAL,
        })
      : Math.min(
          Math.max(balance * 0.1, MIN_POSITION_NOTIONAL),
          MAX_POSITION_NOTIONAL,
        );

    const contracts = Math.max(
      MIN_CONTRACTS,
      Math.min(MAX_CONTRACTS, paperContracts(targetNotional, CONTRACT_SIZE)),
    );
    const actualNotional = paperNotional(contracts, CONTRACT_SIZE);

    const moveGate = paperMinExpectedMoveVsFees(
      markPrice,
      atr14,
      actualNotional,
      TAKER_FEE_PCT,
      effectiveMinExpectedMoveSafetyK(config.minExpectedMoveSafetyK ?? 1, profileCfg),
    );
    if (!moveGate.ok) return false;

    if (
      paperSameDirNotionalWouldExceedCap(
        entryBook,
        side,
        actualNotional,
        equityUsd,
        config.maxSameDirFracOfEquity ?? 0.35,
      )
    ) {
      return false;
    }

    const marginUsed = paperMarginRequired(actualNotional, leverage);
    if (balance < marginUsed) return false;

    const riskCap = balance * (MAX_LOSS_PER_TRADE_PCT / 100);
    if (
      paperEstimatedMaxLossAtStopSl(entryPrice, slPrice, actualNotional, side, TAKER_FEE_PCT) >
      riskCap
    ) {
      return false;
    }

    const holdRes = deskEffectiveHoldMinutesAtOpen(strat.holdMinutes, profile, strat.deskTpWidened);
    const openedAt = new Date(nowMs).toISOString();
    const id = `${symbol}-${strat.id}-${nowMs}`;

    positions.push({
      id,
      symbol,
      strategyId: strat.id,
      strategyName: strat.name,
      side,
      entryPrice,
      markPrice,
      lastPrice,
      contracts,
      notional: actualNotional,
      marginUsed,
      leverage,
      slPrice,
      tpPrice,
      liquidationPrice: paperLiquidationPrice(entryPrice, side, leverage),
      unrealizedPnl: 0,
      unrealizedPnlPct: 0,
      returnPct: 0,
      peakReturnPct: 0,
      fundingCosts: 0,
      lastFundingAppliedAt: nowMs,
      openedAt,
      holdMinutes: holdRes.holdMinutes,
      adaptiveSl: slPrice,
      breakevenMoved: false,
      initialMargin: marginUsed,
      momDecayBars: 0,
    });

    balance -= marginUsed;
    const cdMs = Math.max(
      30_000,
      Math.round(strat.cooldownMin * 60_000 * profileCfg.cooldownMul),
    );
    cooldownUntil[`${symbol}:${strat.id}`] = nowMs + cdMs;
    return true;
  };

  for (let i = 0; i < candles.length; i++) {
    const bar = candles[i]!;
    const nowMs = bar.time;
    const lastPrice = bar.close;
    const markPrice = bar.close;

    const window = sliceWindow(candles.slice(0, i + 1), maxBars);
    if (window.length < minBars) continue;

    const opens = window.map((c) => c.open);
    const closes = window.map((c) => c.close);
    const highs = window.map((c) => c.high);
    const lows = window.map((c) => c.low);
    const volumes = window.map((c) => c.volume);
    const regime = classifyRegimeTagFrom1mOhlcv(opens, highs, lows, closes, volumes);
    const input = buildSignalInputs(opens, closes, highs, lows, volumes, markPrice, nowMs);

    const marked = positions.map((p) =>
      applyMarkToFuturesPosition(p, markPrice, lastPrice, { fundingRate, nowMs }),
    );

    const MOM_DECAY_THRESHOLD = 3; // consecutive bars
    const MOM_DECAY_ATR_FRAC = 0.3; // mom3 < 0.3 × ATR → decaying bar
    const stillOpen: ReplayPosition[] = [];
    for (const pos of marked) {
      // Momentum-decay pre-screen: count consecutive low-momentum bars.
      // Only fires when position is profitable (unrealizedPnl > 0) to avoid cutting losers early.
      const isMomDecayBar =
        Number.isFinite(input.momentum3) &&
        Number.isFinite(input.atr14) &&
        input.atr14 > 0 &&
        (pos.side === "LONG"
          ? input.momentum3 < MOM_DECAY_ATR_FRAC * input.atr14
          : input.momentum3 > -MOM_DECAY_ATR_FRAC * input.atr14);
      const newDecayBars = isMomDecayBar ? pos.momDecayBars + 1 : 0;
      const momDecayFired =
        newDecayBars >= MOM_DECAY_THRESHOLD && pos.unrealizedPnl > 0;

      if (momDecayFired) {
        const merged: ReplayPosition = { ...pos, momDecayBars: newDecayBars };
        const { trade, marginReturn, netPnl } = closeReplayPosition(
          merged,
          markPrice,
          "MOM_DECAY",
          nowMs,
          slippageBps,
        );
        trades.push(trade);
        balance += marginReturn + netPnl;
        continue;
      }

      const { patched, close } = resolveFuturesExitStep(pos, holdTimeMul, nowMs, {
        takerFeePct: TAKER_FEE_PCT,
        exitSlippageBps: slippageBps,
        atr14: input.atr14,
      });
      const merged: ReplayPosition = { ...pos, ...patched, momDecayBars: newDecayBars };
      if (close.shouldClose && close.reason) {
        const { trade, marginReturn, netPnl } = closeReplayPosition(
          merged,
          close.exitPrice,
          close.reason,
          nowMs,
          slippageBps,
        );
        trades.push(trade);
        balance += marginReturn + netPnl;
      } else {
        stillOpen.push(merged);
      }
    }
    positions.length = 0;
    positions.push(...stillOpen);

    const equityUsd =
      balance + positions.reduce((s, p) => s + p.unrealizedPnl, 0);
    peakEquityForDrawdown = Math.max(peakEquityForDrawdown, equityUsd);
    if (config.drawdownLock) {
      const ddPct =
        peakEquityForDrawdown > 0
          ? ((peakEquityForDrawdown - equityUsd) / peakEquityForDrawdown) * 100
          : 0;
      if (ddPct >= REPLAY_MAX_DRAWDOWN_LOCK_PCT) drawdownEntryPaused = true;
      else if (ddPct <= REPLAY_MAX_DRAWDOWN_LOCK_PCT * REPLAY_DRAWDOWN_LOCK_RECOVERY_FRAC) {
        drawdownEntryPaused = false;
      }
    }

    if (positions.length >= config.maxPositions) continue;
    if (config.drawdownLock && drawdownEntryPaused) continue;

    if (config.sessionStartUtcHour != null && config.sessionEndUtcHour != null) {
      const barUtcHour = new Date(nowMs).getUTCHours();
      if (
        !isUtcHourInSession(barUtcHour, config.sessionStartUtcHour, config.sessionEndUtcHour)
      ) {
        continue;
      }
    }

    const occupied = new Set(positions.map((p) => `${p.symbol}:${p.strategyId}`));
    const intraBook: { side: PaperSide; notional: number }[] = positions.map((p) => ({
      side: p.side,
      notional: p.notional,
    }));
    const categoryByStrategyId = new Map(activeStrats.map((s) => [s.id, s.category]));
    const categoryOpenCounts =
      config.maxOpenPerCategory != null
        ? countOpenByCategory(positions, categoryByStrategyId)
        : null;

    for (const strat of activeStrats) {
      if (positions.length >= config.maxPositions) break;
      if (occupied.has(`${symbol}:${strat.id}`)) continue;
      if ((cooldownUntil[`${symbol}:${strat.id}`] ?? 0) > nowMs) continue;

      if (categoryOpenCounts && config.maxOpenPerCategory != null) {
        const category = strat.category?.trim() || "unknown";
        const catOpen = categoryOpenCounts.get(category) ?? 0;
        if (!canOpenCategory(category, catOpen, config.maxOpenPerCategory)) continue;
      }

      const signal = evalMinuteSignal(input, strat);
      const stratThreshold = strat.dynamicThreshold ?? signalThreshold;
      if (signal.score >= stratThreshold && passesEntryConfirmation(input, strat)) {
        const side: PaperSide = strat.signalKey.includes("SHORT") ? "SHORT" : "LONG";
        if (
          tryOpen(strat, side, lastPrice, markPrice, regime, input.atr14, nowMs, equityUsd, intraBook)
        ) {
          const last = positions[positions.length - 1]!;
          intraBook.push({ side: last.side, notional: last.notional });
          occupied.add(`${symbol}:${strat.id}`);
          if (categoryOpenCounts && config.maxOpenPerCategory != null) {
            const category = strat.category?.trim() || "unknown";
            categoryOpenCounts.set(category, (categoryOpenCounts.get(category) ?? 0) + 1);
          }
        }
      }
    }
  }

  return {
    trades,
    stats: summarizeReplayTrades(trades),
    finalBalance: balance,
    barsProcessed: candles.length,
  };
}
