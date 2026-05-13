"use client";

import { useCallback, useEffect, useRef, useState, useMemo, type MutableRefObject } from "react";
import { PRIMARY_QUOTE_SYMBOL, TRADING_SYMBOLS } from "@/lib/futuresMarketData";
import { FUTURES_STRAT_DEFS, type FuturesStratDef } from "@/lib/futuresStrategies";
import {
  buildPaperDeskStrategies,
  deskEffectiveHoldMinutesAtOpen,
  deskFakeDiversityEnabledViaEnv,
  deskMinTpSlRatioFromEnv,
} from "@/lib/futuresDeskPolicy";
import {
  buildSignalInputs,
  effectiveSignalThreshold as computeEffectiveThreshold,
  evalMinuteSignal,
  passesEntryConfirmation,
  type FuturesSignalInputs,
} from "@/lib/futuresSignals";
import {
  computeSessionExitReasonAnalytics,
  computeSessionTradingMetrics,
  formatExitReasonSessionSummary,
  FUTURES_STRATEGY_PROFILES,
  resolveStrategyProfile,
  type FuturesStrategyProfile,
} from "@/lib/futuresSessionMetrics";
import {
  applyFundingAccrual,
  DELTA_PAPER_FUNDING_INTERVAL_MS,
  paperApplyFuturesExitPatches,
  paperContracts,
  paperEstimatedMaxLossAtStopSl,
  paperFuturesProgressTowardTp,
  paperLiquidationDistancePct,
  paperLiquidationPrice,
  paperLinearGrossPnl,
  paperMarginRequired,
  paperNetPnlOnClose,
  paperNotional,
  paperResolveHardExit,
  paperReturnOnMargin,
  type PaperFuturesExitPatchConsts,
} from "@/lib/futuresPaperMath";
import type {
  FuturesDataHealth,
  FuturesDataHealthStatus,
  FuturesDataHealthSymbolIssue,
} from "@/lib/futuresDataHealth.types";
import { FUTURES_FEED_WARNING_AFTER_MS } from "@/lib/futuresDataHealth.types";

export type { FuturesStrategyProfile } from "@/lib/futuresSessionMetrics";
export type { FuturesDataHealth, FuturesDataHealthStatus, FuturesDataHealthSymbolIssue } from "@/lib/futuresDataHealth.types";

/**
 * Multi-asset futures paper engine (Delta India REST via /api/btc/futures-klines?symbol=)
 * Each listed symbol runs the same strategy library on its own 1m candle stream.
 * Key features:
 * - 25x leverage support (4% margin requirement)
 * - Contract-based position sizing (notional/price)
 * - Liquidation price tracking
 * - Mark price-based PnL (not last price)
 * - Funding: Delta `/v2/tickers/{symbol}` → `funding_rate`, `next_funding_time` (see `/api/btc/futures-klines`).
 *   Poll cadence `POLL_MS`; accrual uses calendar time vs `DELTA_PAPER_FUNDING_INTERVAL_MS` (see `applyFundingAccrual`).
 * - Real commission structure (0.1% taker)
 */

// ========== FUTURES-SPECIFIC CONSTANTS ==========
const LEVERAGE = 25; // Default leverage
const MARGIN_PCT = 1 / LEVERAGE; // 4% margin requirement
const MAKER_FEE_PCT = 0.0005; // 0.05% maker
const TAKER_FEE_PCT = 0.001; // 0.10% taker (market orders)
const ROUND_TRIP_FEE_FRAC = TAKER_FEE_PCT * 2; // 0.2% round trip
const FEE_BREAKEVEN_PCT = ROUND_TRIP_FEE_FRAC * 100; // 0.2%

// Account settings
const INITIAL_BALANCE = 1000; // $1000 paper balance for futures (higher than spot)
const MIN_CONTRACTS = 1;
const MAX_CONTRACTS = 50;
/** Fewer concurrent micro-positions → more margin headroom per idea; raise only if product needs more parallel symbols×strategies. */
const MAX_OPEN_POSITIONS = 12;
const CONTRACT_SIZE = 1; // 1 USD per contract on Delta

// Risk management
const MAX_DRAWDOWN_LOCK_PCT = 25; // Pause entries if drawdown > 25%
/** Hysteresis: resume entries when drawdown falls to this fraction of the lock threshold (e.g. 0.84 → 21%). */
const DRAWDOWN_LOCK_RECOVERY_FRAC = 0.84;
const MAX_LOSS_PER_TRADE_PCT = 2; // Max 2% of balance at risk (SL + fees) per new position; skip if exceeded
/**
 * Stats only: mark is within this % of modeled liquidation (see paperLiquidationDistancePct).
 */
const LIQUIDATION_RISK_DISPLAY_PCT = 1.0;

// Strategy parameters
// Paper desk default: strict HTF rules used to reject almost all MTF entries when 5m/15m was NEUTRAL.
const SIGNAL_THRESHOLD = 28;
const MAX_BARS = 120;
const MIN_BARS = 18;
/** Slightly slower poll: many symbols × REST calls per tick */
const POLL_MS = 4_000;
const SYMBOL_FETCH_CHUNK = 5;
const MAX_TRADES = 2_000;

// Exit management
const MIN_ABS_NET_PNL_USD = 2;
const PROFIT_LOCK_PROGRESS = 0.60;
const PROFIT_LOCK_SHARE = 0.60;
const LATE_EXIT_PROGRESS = 0.70;
const LATE_EXIT_MIN_GAIN = 0.22;
const BREAKEVEN_TRIGGER_FRAC = 0.40;
const TRAIL_ACTIVATION_PCT = 0.30;
const TRAIL_GIVEBACK_SHARE = 0.18;
const MTF_HOLD_BONUS = 1.3;
const MOMENTUM_HOLD_EXTEND = 1.25;

// Storage namespace defaults are built per-hook instance.

// ========== TYPES ==========
export type Side = "LONG" | "SHORT";
export type Status = "WARMING" | "READY" | "IN_POSITION" | "COOLING";
export type MarginMode = "isolated" | "cross";

/** Futures Position */
export interface BTCFuturesPosition {
  id: string;
  /** Perpetual contract symbol e.g. BTCUSD, ETHUSD */
  symbol: string;
  strategyId: number;
  strategyName: string;
  side: Side;
  entryPrice: number; // Entry price (mark price at open)
  markPrice: number; // Current mark price
  lastPrice: number; // Current last price
  contracts: number; // Number of contracts (USD notional)
  notional: number; // Position notional value in USD
  marginUsed: number; // Margin allocated to this position
  leverage: number; // Leverage used (default 25x)
  liquidationPrice: number; // Price at which position gets liquidated
  unrealizedPnl: number; // Unrealized PnL in USD
  unrealizedPnlPct: number; // Unrealized PnL as % of notional
  returnPct: number; // Return % relative to margin used
  /** Peak `returnPct` (on margin) since open — trail giveback exit. */
  peakReturnPct: number;
  tpPrice: number; // Take profit price
  slPrice: number; // Stop loss price
  fundingCosts: number; // Accumulated funding rate costs
  /** Epoch ms — last poll at which calendar-time funding was accrued (persisted for resume). */
  lastFundingAppliedAt: number;
  openedAt: string; // ISO timestamp
  holdMinutes: number;
  exitReason?: "TP" | "SL" | "TIME" | "TRAIL" | "BREAKEVEN" | "LIQUIDATION_RISK" | "PROFIT_LOCK";
  marginMode: MarginMode;
  // Internal tracking
  adaptiveSl: number;
  breakevenMoved: boolean;
  initialMargin: number;
}

/** Futures Trade (closed position) */
export interface BTCFuturesTrade {
  id: string;
  symbol: string;
  strategyId: number;
  strategyName: string;
  side: Side;
  entryPrice: number;
  exitPrice: number;
  contracts: number;
  notional: number;
  marginUsed: number;
  realizedPnl: number; // Gross PnL
  fees: number; // Entry + exit fees
  netPnl: number; // Net after fees
  netPnlPct: number; // Net % return on margin
  fundingCosts: number; // Total funding paid/received
  /** Debug: ms epoch after last funding accrual tick (optional export). */
  lastFundingAppliedAt?: number;
  /** Debug: wall-clock hold length (optional export). */
  fundingSinceOpenMs?: number;
  openedAt: string;
  closedAt: string;
  exitReason: "TP" | "SL" | "TIME" | "TRAIL" | "BREAKEVEN" | "LIQUIDATION_RISK" | "PROFIT_LOCK";
  liquidationPrice: number; // For reference
  liquidationDistancePct: number; // How close to liquidation at close
}

/** Strategy Definition — imported from @/lib/futuresStrategies */
type StratDef = FuturesStratDef;

/** Funding Rate Info */
interface FundingInfo {
  rate: number; // Funding rate (e.g., 0.0001 = 0.01%)
  nextFundingTime: number; // Unix timestamp
  timestamp: number;
}

/** Engine State */
interface EngineState {
  balance: number;
  positions: BTCFuturesPosition[];
  trades: BTCFuturesTrade[];
  quote: {
    lastPrice: number;
    markPrice: number;
    indexPrice: number;
    changePct24h: number;
    fundingRate: number;
    nextFunding: number;
    timestamp: number;
  } | null;
  status: Status;
  warmingPct: number;
  disabledStrategies: number[];
  pauseEntries: boolean;
  lastTradeAt: number;
  dayStartBalance: number;
  dayStartDate: number;
}

/** Engine Reference */
export interface EngineRef {
  positions: BTCFuturesPosition[];
  trades: BTCFuturesTrade[];
  balance: number;
  equity: number;
  availableMargin: number;
  usedMargin: number;
  stats: BTCFuturesEngineStats;
  quote: EngineState["quote"];
  isReady: boolean;
  pauseEntries: boolean;
  disabledStrategies: number[];
  togglePause: () => void;
  resetPaperAccount: () => void;
  clearTradeHistory: () => void;
  setDisabledStrategies: (ids: number[]) => void;
  exportCSV: () => string;
  exportJSON: () => string;
  dataHealth: FuturesDataHealth;
}

/** Engine Stats */
export interface BTCFuturesEngineStats {
  totalTrades: number;
  winCount: number;
  lossCount: number;
  winRate: number;
  avgWin: number;
  avgLoss: number;
  profitFactor: number;
  realizedPnl: number;
  unrealizedPnl: number;
  totalFees: number;
  totalFundingCosts: number;
  netPnl: number;
  maxDrawdownPct: number;
  currentDrawdownPct: number;
  openPositions: number;
  maxPositions: number;
  longCount: number;
  shortCount: number;
  balance: number;
  equity: number;
  availableMargin: number;
  usedMargin: number;
  marginUtilization: number;
  liquidationRisk: number; // Positions near liquidation
  avgLeverage: number;
  dayStartBalance: number;
  /** Closed trades / session hour (earliest open → now). */
  sessionTradesPerHour: number;
  /** Mean net PnL per closed trade (expectancy proxy). */
  sessionExpectancyPerTrade: number;
  /** sum(fees) / sum(|realized gross|) × 100. */
  sessionFeePctOfAbsGross: number;
  sessionAvgHoldMinutes: number;
  sessionMedianHoldMinutes: number;
  sessionHoldP95Minutes: number;
  strategyProfile: FuturesStrategyProfile;
  effectiveSignalThreshold: number;
  /** Peak-to-trough equity drawdown % vs session peak equity (balance + open unrealized). */
  drawdownPct: number;
  /** New entries paused (drawdown lock) until partial recovery (hysteresis). */
  isDrawdownLocked: boolean;
  /** Phase 2: count of strategies with TP% widened to meet min TP/SL ratio. */
  deskTpWidenedStratCount: number;
  /** Phase 2: strategies excluded (TP widen would exceed cap). */
  deskLowRrSkippedStratCount: number;
  /** Phase 3: count of fake-diversity IDs (79–110) filtered when env flag off. */
  deskFakeDiversityFilteredCount: number;
  /** Comma-separated strat IDs (TP widened), truncated for UI. */
  deskTpWidenedStratIds: string;
  /** Comma-separated strat IDs skipped for low RR after widen cap. */
  deskLowRrSkippedStratIds: string;
  /** Opens where `scalp_aggro_v1` + desk-widened TP bumped base `holdMinutes` before `holdTimeMul`. */
  deskProfileAdjustedHoldAppliedCount: number;
  /** Last-N closed trades: exit reason × count and mean net (grouped expectancy). */
  sessionExitReasonSummary: string;
}

/** Strategy Status */
export interface BTCFuturesStrategyStatus {
  id: number;
  name: string;
  category: string;
  status: "OPEN" | "COOLING" | "AVAILABLE";
  disabled: boolean;
  openCount: number;
  lastTradeAt: number | null;
  score: number; // Performance score (0-100)
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
}

export type BTCFuturesEngineOptions = {
  /** Separate persistence namespace so multiple futures modules don't share state. */
  storageNamespace?: string;
  /** Optional strategy allow-list. If empty, full strategy library runs. */
  strategyIds?: number[];
  /** Optional symbol allow-list. If empty, full trading symbol list runs. */
  symbols?: readonly string[];
  /** Optional module-specific signal threshold override. */
  signalThreshold?: number;
  /** Paper-only: tune entry cadence / cooldown / time exits without editing STRAT_DEFS. */
  strategyProfile?: FuturesStrategyProfile;
};

// Signal inputs type imported from @/lib/futuresSignals
type SignalInputs = FuturesSignalInputs;

// ========== STRATEGY DEFINITIONS (imported from @/lib/futuresStrategies) ==========
const STRAT_DEFS = FUTURES_STRAT_DEFS;

// ========== LIQUIDATION / PnL (pure helpers live in @/lib/futuresPaperMath) ==========

// ========== MARGIN CALCULATIONS (delegated to futuresPaperMath) ==========
function calculateMarginRequired(notional: number, leverage: number): number {
  return paperMarginRequired(notional, leverage);
}

function calculateContracts(notional: number, _price: number): number {
  return paperContracts(notional, CONTRACT_SIZE);
}

function calculateNotional(contracts: number): number {
  return paperNotional(contracts, CONTRACT_SIZE);
}

const FUTURES_EXIT_PATCH_CONSTS: PaperFuturesExitPatchConsts = {
  breakevenTriggerProgress: BREAKEVEN_TRIGGER_FRAC,
  trailActivationProgress: TRAIL_ACTIVATION_PCT,
  trailGivebackShare: TRAIL_GIVEBACK_SHARE,
};

/**
 * One poll: persist trail / breakeven / peak, then hard exits (liq→SL→TP→TIME), then profit-lock and trail giveback closes.
 */
function resolveFuturesExitStep(
  p: BTCFuturesPosition,
  _input: SignalInputs,
  holdTimeMul: number,
): {
  patched: BTCFuturesPosition;
  close: { shouldClose: boolean; reason?: NonNullable<BTCFuturesPosition["exitReason"]>; exitPrice: number };
} {
  const progress = paperFuturesProgressTowardTp(p.returnPct, p.entryPrice, p.tpPrice);
  const soft = paperApplyFuturesExitPatches(
    {
      side: p.side,
      entryPrice: p.entryPrice,
      markPrice: p.markPrice,
      adaptiveSl: p.adaptiveSl,
      breakevenMoved: p.breakevenMoved,
      returnPctOnMargin: p.returnPct,
      peakReturnPctOnMargin: p.peakReturnPct ?? p.returnPct,
      progressTowardTp: progress,
    },
    FUTURES_EXIT_PATCH_CONSTS,
  );
  const q: BTCFuturesPosition = { ...p, ...soft };

  const hard = paperResolveHardExit({
    side: q.side,
    markPrice: q.markPrice,
    liquidationPrice: q.liquidationPrice,
    adaptiveSl: q.adaptiveSl,
    tpPrice: q.tpPrice,
    entryPrice: q.entryPrice,
    openedAtMs: new Date(q.openedAt).getTime(),
    nowMs: Date.now(),
    holdMinutes: q.holdMinutes,
    mtfHoldBonus: MTF_HOLD_BONUS,
    holdTimeMul,
  });
  if (hard.shouldClose) {
    return {
      patched: q,
      close: { shouldClose: true, reason: hard.reason, exitPrice: hard.exitPrice },
    };
  }

  const tpPctAbs = Math.abs((q.tpPrice - q.entryPrice) / q.entryPrice) * 100;
  const lockTh = Math.max(LATE_EXIT_MIN_GAIN, tpPctAbs * PROFIT_LOCK_SHARE);
  if (progress >= PROFIT_LOCK_PROGRESS && q.returnPct >= lockTh) {
    return { patched: q, close: { shouldClose: true, reason: "PROFIT_LOCK", exitPrice: q.markPrice } };
  }

  const peak = soft.peakReturnPctOnMargin;
  if (
    progress >= TRAIL_ACTIVATION_PCT &&
    peak > LATE_EXIT_MIN_GAIN &&
    q.returnPct <= peak * (1 - TRAIL_GIVEBACK_SHARE)
  ) {
    return { patched: q, close: { shouldClose: true, reason: "TRAIL", exitPrice: q.markPrice } };
  }

  return { patched: q, close: { shouldClose: false, exitPrice: q.markPrice } };
}

function applyMarkToPosition(
  p: BTCFuturesPosition,
  markPrice: number,
  lastPrice: number,
  ctx: { fundingRate: number; nowMs: number },
): BTCFuturesPosition {
  const unrealizedPnL = paperLinearGrossPnl(p.entryPrice, markPrice, p.notional, p.side);
  const returnPct = paperReturnOnMargin(unrealizedPnL, p.marginUsed);
  const unrealizedPnLPct = p.notional > 0 ? (unrealizedPnL / p.notional) * 100 : 0;

  const lastAcc = Number.isFinite(p.lastFundingAppliedAt) ? p.lastFundingAppliedAt : ctx.nowMs;
  const elapsedMs = Math.max(0, ctx.nowMs - lastAcc);
  const fundingDelta = applyFundingAccrual({
    side: p.side,
    notional: p.notional,
    fundingRate: ctx.fundingRate,
    elapsedMs,
    fundingIntervalMs: DELTA_PAPER_FUNDING_INTERVAL_MS,
  });

  return {
    ...p,
    markPrice,
    lastPrice,
    unrealizedPnl: unrealizedPnL,
    unrealizedPnlPct: unrealizedPnLPct,
    returnPct,
    fundingCosts: p.fundingCosts + fundingDelta,
    lastFundingAppliedAt: ctx.nowMs,
  };
}

function chunkArray<T>(arr: readonly T[], size: number): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < arr.length; i += size) {
    out.push([...arr.slice(i, i + size)]);
  }
  return out;
}


// ========== HOOK ==========
export function useBTCFuturesScalperEngine(options: BTCFuturesEngineOptions = {}): {
  positions: BTCFuturesPosition[];
  trades: BTCFuturesTrade[];
  balance: number;
  equity: number;
  availableMargin: number;
  usedMargin: number;
  stats: BTCFuturesEngineStats;
  quote: EngineState["quote"];
  isReady: boolean;
  pauseEntries: boolean;
  disabledStrategies: number[];
  engineRef: React.MutableRefObject<EngineRef | null>;
  togglePause: () => void;
  resetPaperAccount: () => void;
  clearTradeHistory: () => void;
  setDisabledStrategies: (ids: number[]) => void;
  exportCSV: () => string;
  exportJSON: () => string;
  strategyStatuses: BTCFuturesStrategyStatus[];
  strategyProfile: FuturesStrategyProfile;
  effectiveSignalThreshold: number;
  dataHealth: FuturesDataHealth;
} {
  const storageNamespace = options.storageNamespace?.trim() || "btc_futures_scalper";
  const strategyIds = options.strategyIds ?? null;
  const symbols = options.symbols ?? null;
  const strategyProfile = useMemo(
    () => resolveStrategyProfile(options.strategyProfile),
    [options.strategyProfile],
  );
  const profileCfg = FUTURES_STRATEGY_PROFILES[strategyProfile];
  const activeSignalThreshold = useMemo(() => {
    const base = Number.isFinite(options.signalThreshold)
      ? Number(options.signalThreshold)
      : SIGNAL_THRESHOLD;
    return computeEffectiveThreshold(base, profileCfg.signalThresholdDelta);
  }, [options.signalThreshold, profileCfg.signalThresholdDelta]);
  const stateStorageKey = `${storageNamespace}_paper_state`;

  const deskStrategiesResult = useMemo(() => {
    const allow = strategyIds && strategyIds.length > 0 ? new Set(strategyIds) : null;
    const raw = !allow ? [...STRAT_DEFS] : STRAT_DEFS.filter((s) => allow.has(s.id));
    const base = raw.length > 0 ? raw : [...STRAT_DEFS];
    return buildPaperDeskStrategies(base, {
      strategyIdAllowlist: null,
      minTpSlRatio: deskMinTpSlRatioFromEnv(),
      allowFakeDiversity: deskFakeDiversityEnabledViaEnv(),
    });
  }, [strategyIds]);

  const activeStratDefs = deskStrategiesResult.strategies;
  const deskPolicySnapshot = useMemo(
    () => ({
      tpWidenedCount: deskStrategiesResult.tpWidenedStratIds.length,
      lowRrSkippedCount: deskStrategiesResult.lowRrSkippedStratIds.length,
      fakeDiversityFiltered: deskStrategiesResult.fakeDiversityFilteredCount,
      tpWidenedIds: [...deskStrategiesResult.tpWidenedStratIds].sort((a, b) => a - b).join(","),
      lowRrSkippedIds: [...deskStrategiesResult.lowRrSkippedStratIds].sort((a, b) => a - b).join(","),
    }),
    [deskStrategiesResult],
  );
  const activeStrategyIdSet = useMemo(
    () => new Set(activeStratDefs.map((s) => s.id)),
    [activeStratDefs],
  );
  const activeSymbols = useMemo(() => {
    if (!symbols || symbols.length === 0) return [...TRADING_SYMBOLS];
    const cleaned = symbols.map((s) => s.trim().toUpperCase()).filter(Boolean);
    return cleaned.length > 0 ? cleaned : [...TRADING_SYMBOLS];
  }, [symbols]);

  // State
  const [balance, setBalance] = useState(INITIAL_BALANCE);
  const [positions, setPositions] = useState<BTCFuturesPosition[]>([]);
  const [trades, setTrades] = useState<BTCFuturesTrade[]>([]);
  const [quote, setQuote] = useState<EngineState["quote"]>(null);
  const [status, setStatus] = useState<Status>("WARMING");
  const statusRef = useRef(status);
  useEffect(() => {
    statusRef.current = status;
  }, [status]);
  const [warmingPct, setWarmingPct] = useState(0);
  const [pauseEntries, setPauseEntries] = useState(false);
  const [disabledStrategies, setDisabledStrategies] = useState<number[]>([]);
  const [lastTradeAt, setLastTradeAt] = useState(0);
  const [dayStartBalance, setDayStartBalance] = useState(INITIAL_BALANCE);
  const [dayStartDate, setDayStartDate] = useState(() => new Date().getDate());
  const [dataHealth, setDataHealth] = useState<FuturesDataHealth>(() => ({
    status: "stale",
    lastError: null,
    lastOkAt: null,
    lastPollAt: 0,
    failingSymbols: [],
    symbolIssues: [],
    payloadsReady: 0,
    symbolsRequested: 0,
    showFeedWarning: false,
  }));

  // Refs
  const engineRef = useRef<EngineRef | null>(null);
  const notOkSinceRef = useRef<number | null>(null);
  const fundingTickerMetaWarnedRef = useRef(false);
  const peakEquityForDrawdownRef = useRef(INITIAL_BALANCE);
  const drawdownEntryPausedRef = useRef(false);
  const positionsRef = useRef(positions);
  const tradesRef = useRef(trades);
  const balanceRef = useRef(balance);
  const disabledRef = useRef(disabledStrategies);
  const pauseRef = useRef(pauseEntries);
  const lastTradeAtRef = useRef(lastTradeAt);
  const stratCooldownsRef = useRef<Record<string, number>>({});
  const deskProfileAdjustedHoldCountRef = useRef(0);
  const profileCooldownMulRef = useRef(1);
  const profileHoldTimeMulRef = useRef(1);
  const dayStartBalanceRef = useRef(dayStartBalance);
  const dayStartDateRef = useRef(dayStartDate);

  // Sync refs
  useEffect(() => { positionsRef.current = positions; }, [positions]);
  useEffect(() => { tradesRef.current = trades; }, [trades]);
  useEffect(() => { balanceRef.current = balance; }, [balance]);
  useEffect(() => { disabledRef.current = disabledStrategies; }, [disabledStrategies]);
  useEffect(() => { pauseRef.current = pauseEntries; }, [pauseEntries]);
  useEffect(() => { lastTradeAtRef.current = lastTradeAt; }, [lastTradeAt]);
  useEffect(() => { dayStartBalanceRef.current = dayStartBalance; }, [dayStartBalance]);
  useEffect(() => { dayStartDateRef.current = dayStartDate; }, [dayStartDate]);

  useEffect(() => {
    profileCooldownMulRef.current = profileCfg.cooldownMul;
    profileHoldTimeMulRef.current = profileCfg.holdTimeMul;
  }, [profileCfg]);

  // ========== LOCAL STORAGE ==========
  const loadLs = useCallback((): Partial<EngineState> | null => {
    try {
      const raw = localStorage.getItem(stateStorageKey);
      if (!raw) return null;
      const parsed = JSON.parse(raw) as EngineState;
      return parsed;
    } catch {
      return null;
    }
  }, [stateStorageKey]);

  const saveLs = useCallback((state: EngineState) => {
    try {
      localStorage.setItem(stateStorageKey, JSON.stringify(state));
    } catch {
      // ignore
    }
  }, [stateStorageKey]);

  // Initial load
  useEffect(() => {
    const saved = loadLs();
    if (saved) {
      if (typeof saved.balance === "number") setBalance(saved.balance);
      if (Array.isArray(saved.positions)) {
        const hydratedNow = Date.now();
        setPositions(
          saved.positions.map((p: BTCFuturesPosition) => ({
            ...p,
            symbol: p.symbol || PRIMARY_QUOTE_SYMBOL,
            lastFundingAppliedAt:
              typeof p.lastFundingAppliedAt === "number" && Number.isFinite(p.lastFundingAppliedAt)
                ? p.lastFundingAppliedAt
                : hydratedNow,
            peakReturnPct:
              typeof p.peakReturnPct === "number" && Number.isFinite(p.peakReturnPct) ? p.peakReturnPct : 0,
          })),
        );
      }
      if (Array.isArray(saved.trades)) {
        setTrades(
          saved.trades.slice(-MAX_TRADES).map((t: BTCFuturesTrade) => ({
            ...t,
            symbol: t.symbol || PRIMARY_QUOTE_SYMBOL,
          })),
        );
      }
      if (typeof saved.pauseEntries === "boolean") setPauseEntries(saved.pauseEntries);
      if (Array.isArray(saved.disabledStrategies)) {
        setDisabledStrategies(saved.disabledStrategies.filter((id) => activeStrategyIdSet.has(id)));
      }
      if (typeof saved.lastTradeAt === "number") setLastTradeAt(saved.lastTradeAt);
      if (typeof saved.dayStartBalance === "number") setDayStartBalance(saved.dayStartBalance);
      if (typeof saved.dayStartDate === "number") setDayStartDate(saved.dayStartDate);
    }
  }, [activeStrategyIdSet, loadLs]);

  // Periodic save
  useEffect(() => {
    const id = setInterval(() => {
      saveLs({
        balance,
        positions,
        trades: trades.slice(-MAX_TRADES),
        quote,
        status,
        warmingPct,
        disabledStrategies,
        pauseEntries,
        lastTradeAt,
        dayStartBalance,
        dayStartDate,
      });
    }, 30000);
    return () => clearInterval(id);
  }, [balance, positions, trades, quote, status, warmingPct, disabledStrategies, pauseEntries, lastTradeAt, dayStartBalance, dayStartDate, saveLs]);

  // ========== ACTIONS ==========
  const togglePause = useCallback(() => {
    setPauseEntries(p => !p);
  }, []);

  const resetPaperAccount = useCallback(() => {
    setBalance(INITIAL_BALANCE);
    setPositions([]);
    setTrades([]);
    setLastTradeAt(0);
    setDayStartBalance(INITIAL_BALANCE);
    setDayStartDate(new Date().getDate());
    stratCooldownsRef.current = {};
    peakEquityForDrawdownRef.current = INITIAL_BALANCE;
    drawdownEntryPausedRef.current = false;
    deskProfileAdjustedHoldCountRef.current = 0;
    localStorage.removeItem(stateStorageKey);
  }, [stateStorageKey]);

  const clearTradeHistory = useCallback(() => {
    setTrades([]);
    setLastTradeAt(0);
    stratCooldownsRef.current = {};
  }, []);

  const setDisabledStrategiesHandler = useCallback((ids: number[]) => {
    setDisabledStrategies(ids.filter((id) => activeStrategyIdSet.has(id)));
  }, [activeStrategyIdSet]);

  const exportCSV = useCallback(() => {
    const headers = ["ID", "Symbol", "Strategy", "Side", "Entry", "Exit", "Contracts", "Realized PnL", "Fees", "Net PnL", "Net PnL %", "Funding", "Opened", "Closed", "Exit Reason"];
    const rows = trades.map(t => [
      t.id,
      t.symbol || PRIMARY_QUOTE_SYMBOL,
      t.strategyName,
      t.side,
      t.entryPrice.toFixed(2),
      t.exitPrice.toFixed(2),
      t.contracts,
      t.realizedPnl.toFixed(2),
      t.fees.toFixed(4),
      t.netPnl.toFixed(2),
      t.netPnlPct.toFixed(2),
      t.fundingCosts.toFixed(4),
      t.openedAt,
      t.closedAt,
      t.exitReason,
    ]);
    return [headers.join(","), ...rows.map(r => r.join(","))].join("\n");
  }, [trades]);

  // ========== STATS ==========
  const calculateStats = useCallback((): BTCFuturesEngineStats => {
    const wins = trades.filter(t => t.netPnl > 0);
    const losses = trades.filter(t => t.netPnl <= 0);
    const winCount = wins.length;
    const lossCount = losses.length;
    const totalTrades = trades.length;
    const winRate = totalTrades > 0 ? (winCount / totalTrades) * 100 : 0;
    const avgWin = winCount > 0 ? wins.reduce((s, t) => s + t.netPnl, 0) / winCount : 0;
    const avgLoss = lossCount > 0 ? losses.reduce((s, t) => s + t.netPnl, 0) / lossCount : 0;
    const profitFactor = avgLoss !== 0 ? Math.abs(avgWin / avgLoss) : 0;
    const realizedPnl = trades.reduce((s, t) => s + t.netPnl, 0);
    const unrealizedPnl = positions.reduce((s, p) => s + p.unrealizedPnl, 0);
    const totalFees = trades.reduce((s, t) => s + t.fees, 0);
    const totalFunding = trades.reduce((s, t) => s + t.fundingCosts, 0) + positions.reduce((s, p) => s + p.fundingCosts, 0);
    const netPnl = realizedPnl + unrealizedPnl;

    const peak = Math.max(INITIAL_BALANCE, ...trades.map((_, i) => INITIAL_BALANCE + trades.slice(0, i + 1).reduce((s, t) => s + t.netPnl, 0)));
    const currentEquity = balance + unrealizedPnl;
    const drawdown = peak > 0 ? ((peak - currentEquity) / peak) * 100 : 0;

    peakEquityForDrawdownRef.current = Math.max(peakEquityForDrawdownRef.current, currentEquity);
    const sessionEquityDrawdownPct =
      peakEquityForDrawdownRef.current > 0
        ? ((peakEquityForDrawdownRef.current - currentEquity) / peakEquityForDrawdownRef.current) * 100
        : 0;
    if (sessionEquityDrawdownPct >= MAX_DRAWDOWN_LOCK_PCT) drawdownEntryPausedRef.current = true;
    else if (sessionEquityDrawdownPct <= MAX_DRAWDOWN_LOCK_PCT * DRAWDOWN_LOCK_RECOVERY_FRAC) {
      drawdownEntryPausedRef.current = false;
    }

    const usedMargin = positions.reduce((s, p) => s + p.marginUsed, 0);
    const availableMargin = balance - usedMargin;
    const marginUtilization = balance > 0 ? (usedMargin / balance) * 100 : 0;

    const longCount = positions.filter(p => p.side === "LONG").length;
    const shortCount = positions.filter(p => p.side === "SHORT").length;

    const liquidationRisk = positions.filter(p => {
      const dist = paperLiquidationDistancePct(p.markPrice, p.liquidationPrice, p.side);
      return dist >= 0 && dist < LIQUIDATION_RISK_DISPLAY_PCT;
    }).length;

    const avgLeverage = positions.length > 0 ? positions.reduce((s, p) => s + p.leverage, 0) / positions.length : 0;

    const sm = computeSessionTradingMetrics(trades);
    const exitAn = computeSessionExitReasonAnalytics(trades);

    return {
      totalTrades,
      winCount,
      lossCount,
      winRate,
      avgWin,
      avgLoss,
      profitFactor,
      realizedPnl,
      unrealizedPnl,
      totalFees,
      totalFundingCosts: totalFunding,
      netPnl,
      maxDrawdownPct: drawdown,
      currentDrawdownPct: drawdown,
      openPositions: positions.length,
      maxPositions: MAX_OPEN_POSITIONS,
      longCount,
      shortCount,
      balance,
      equity: currentEquity,
      availableMargin,
      usedMargin,
      marginUtilization,
      liquidationRisk,
      avgLeverage,
      dayStartBalance: dayStartBalanceRef.current,
      sessionTradesPerHour: sm.tradesPerHour,
      sessionExpectancyPerTrade: sm.expectancyPerTrade,
      sessionFeePctOfAbsGross: sm.feePctOfAbsGross,
      sessionAvgHoldMinutes: sm.avgHoldMinutes,
      sessionMedianHoldMinutes: sm.medianHoldMinutes,
      sessionHoldP95Minutes: sm.holdP95Minutes,
      strategyProfile,
      effectiveSignalThreshold: activeSignalThreshold,
      drawdownPct: sessionEquityDrawdownPct,
      isDrawdownLocked: drawdownEntryPausedRef.current,
      deskTpWidenedStratCount: deskPolicySnapshot.tpWidenedCount,
      deskLowRrSkippedStratCount: deskPolicySnapshot.lowRrSkippedCount,
      deskFakeDiversityFilteredCount: deskPolicySnapshot.fakeDiversityFiltered,
      deskTpWidenedStratIds: deskPolicySnapshot.tpWidenedIds,
      deskLowRrSkippedStratIds: deskPolicySnapshot.lowRrSkippedIds,
      deskProfileAdjustedHoldAppliedCount: deskProfileAdjustedHoldCountRef.current,
      sessionExitReasonSummary: formatExitReasonSessionSummary(exitAn.rows),
    };
  }, [trades, positions, balance, strategyProfile, activeSignalThreshold, deskPolicySnapshot]);

  const exportJSON = useCallback(() => {
    return JSON.stringify({ balance, positions, trades, stats: calculateStats() }, null, 2);
  }, [balance, positions, trades, calculateStats]);

  // ========== STRATEGY STATUSES ==========
  const strategyStatuses = useMemo((): BTCFuturesStrategyStatus[] => {
    const now = Date.now();
    return activeStratDefs.map(strat => {
      const openCount = positions.filter(p => p.strategyId === strat.id).length;
      const stratTrades = trades.filter(t => t.strategyId === strat.id);
      const lastTrade = stratTrades[stratTrades.length - 1];
      const inCooldown = Object.entries(stratCooldownsRef.current).some(
        ([key, until]) => key.endsWith(`:${strat.id}`) && (until ?? 0) > now,
      );
      const wins = stratTrades.filter(t => t.netPnl > 0).length;
      const losses = stratTrades.filter(t => t.netPnl <= 0).length;
      const totalPnl = stratTrades.reduce((s, t) => s + t.netPnl, 0);
      const winRate = stratTrades.length > 0 ? (wins / stratTrades.length) * 100 : 0;
      // Score: weighted combination of win rate and total PnL
      const score = stratTrades.length > 0
        ? Math.min(100, winRate * 0.7 + Math.min(30, Math.abs(totalPnl) / 100) * 0.3)
        : 0;

      return {
        id: strat.id,
        name: strat.name,
        category: strat.category,
        status: openCount > 0 ? "OPEN" : inCooldown ? "COOLING" : "AVAILABLE",
        disabled: disabledStrategies.includes(strat.id),
        openCount,
        lastTradeAt: lastTrade ? new Date(lastTrade.closedAt).getTime() : null,
        score,
        totalTrades: stratTrades.length,
        wins,
        losses,
        totalPnl,
        winRate,
      };
    });
  }, [positions, trades, disabledStrategies, activeStratDefs]);

  // ========== POSITION MANAGEMENT ==========
  const openPosition = useCallback((strat: StratDef, side: Side, price: number, markPrice: number, symbol: string) => {
    const id = `${symbol}-${strat.id}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const bal = balanceRef.current;
    const notional = Math.min(Math.max(bal * 0.1, 100), 500); // 10% of balance, min $100, max $500
    const contracts = Math.max(MIN_CONTRACTS, Math.min(MAX_CONTRACTS, calculateContracts(notional, price)));
    const actualNotional = calculateNotional(contracts);
    const marginUsed = calculateMarginRequired(actualNotional, LEVERAGE);
    if (bal < marginUsed) return;
    const slPrice = side === "LONG" ? price * (1 - strat.slPct / 100) : price * (1 + strat.slPct / 100);
    const tpPrice = side === "LONG" ? price * (1 + strat.tpPct / 100) : price * (1 - strat.tpPct / 100);
    const riskCap = bal * (MAX_LOSS_PER_TRADE_PCT / 100);
    if (paperEstimatedMaxLossAtStopSl(price, slPrice, actualNotional, side, TAKER_FEE_PCT) > riskCap) return;
    /** Keep ref in sync so multiple opens in one poll tick do not all read stale balance. */
    balanceRef.current = bal - marginUsed;
    const liquidationPrice = paperLiquidationPrice(price, side, LEVERAGE);

    const holdRes = deskEffectiveHoldMinutesAtOpen(strat.holdMinutes, strategyProfile, strat.deskTpWidened);
    if (holdRes.profileAdjusted) deskProfileAdjustedHoldCountRef.current += 1;

    const position: BTCFuturesPosition = {
      id,
      symbol,
      strategyId: strat.id,
      strategyName: strat.name,
      side,
      entryPrice: price,
      markPrice,
      lastPrice: price,
      contracts,
      notional: actualNotional,
      marginUsed,
      leverage: LEVERAGE,
      liquidationPrice,
      unrealizedPnl: 0,
      unrealizedPnlPct: 0,
      returnPct: 0,
      peakReturnPct: 0,
      tpPrice,
      slPrice,
      fundingCosts: 0,
      lastFundingAppliedAt: Date.now(),
      openedAt: new Date().toISOString(),
      holdMinutes: holdRes.holdMinutes,
      marginMode: "isolated",
      adaptiveSl: slPrice,
      breakevenMoved: false,
      initialMargin: marginUsed,
    };

    setPositions(prev => [...prev, position]);
    setBalance(balanceRef.current);
    stratCooldownsRef.current[`${symbol}:${strat.id}`] =
      Date.now() +
      Math.max(30_000, Math.round(strat.cooldownMin * 60_000 * profileCooldownMulRef.current));
    setLastTradeAt(Date.now());
  }, [strategyProfile]);

  const closePosition = useCallback((position: BTCFuturesPosition, exitPrice: number, exitReason: BTCFuturesPosition["exitReason"]) => {
    const { grossPnl, fees, netPnl } = paperNetPnlOnClose({
      entryPrice: position.entryPrice,
      exitPrice,
      notional: position.notional,
      side: position.side,
      takerFeePct: TAKER_FEE_PCT,
      fundingCosts: position.fundingCosts,
      minAbsNetWinUsd: MIN_ABS_NET_PNL_USD,
    });

    const netPnlPct = position.marginUsed > 0 ? (netPnl / position.marginUsed) * 100 : 0;
    const liqDist = paperLiquidationDistancePct(exitPrice, position.liquidationPrice, position.side);
    const closedAtIso = new Date().toISOString();
    const openedMs = new Date(position.openedAt).getTime();
    const closedMs = new Date(closedAtIso).getTime();
    const fundingSinceOpenMs =
      Number.isFinite(openedMs) && Number.isFinite(closedMs) ? Math.max(0, closedMs - openedMs) : undefined;

    const trade: BTCFuturesTrade = {
      id: position.id,
      symbol: position.symbol,
      strategyId: position.strategyId,
      strategyName: position.strategyName,
      side: position.side,
      entryPrice: position.entryPrice,
      exitPrice,
      contracts: position.contracts,
      notional: position.notional,
      marginUsed: position.marginUsed,
      realizedPnl: grossPnl,
      fees,
      netPnl,
      netPnlPct,
      fundingCosts: position.fundingCosts,
      lastFundingAppliedAt: position.lastFundingAppliedAt,
      fundingSinceOpenMs,
      openedAt: position.openedAt,
      closedAt: closedAtIso,
      exitReason: exitReason!,
      liquidationPrice: position.liquidationPrice,
      liquidationDistancePct: liqDist,
    };

    setTrades(prev => [...prev.slice(-MAX_TRADES + 1), trade]);
    setPositions(prev => prev.filter(p => p.id !== position.id));
    setBalance(prev => prev + position.marginUsed + netPnl);
  }, []);

  // ========== DATA POLLING (multi-symbol) ==========
  useEffect(() => {
    let mounted = true;
    let interval: NodeJS.Timeout | null = null;

    type KlinePayload = {
      ok: boolean;
      candles: { time: number; open: number; high: number; low: number; close: number; volume: number }[];
      lastPrice: number;
      markPrice: number;
      indexPrice: number;
      changePct24h: number;
      fundingRate: number;
      nextFunding: number;
      fetchedAt: string;
    };

    const poll = async () => {
      const debug502 = process.env.NEXT_PUBLIC_SIMULATE_FUTURES_502 === "1";
      const klineQuery = (sym: string) =>
        `/api/btc/futures-klines?symbol=${encodeURIComponent(sym)}${debug502 ? "&debugFutures502=1" : ""}`;

      try {
        const payloads = new Map<string, KlinePayload>();
        const symbolIssues: FuturesDataHealthSymbolIssue[] = [];

        for (const batch of chunkArray(activeSymbols, SYMBOL_FETCH_CHUNK)) {
          const results = await Promise.all(
            batch.map(async (sym): Promise<{ sym: string; payload: KlinePayload | null; issue: FuturesDataHealthSymbolIssue | null }> => {
              try {
                const res = await fetch(klineQuery(sym), { cache: "no-store" });
                if (!res.ok) {
                  return {
                    sym,
                    payload: null,
                    issue: { symbol: sym, reason: "http_error", detail: String(res.status) },
                  };
                }
                let j: KlinePayload;
                try {
                  j = (await res.json()) as KlinePayload;
                } catch {
                  return {
                    sym,
                    payload: null,
                    issue: { symbol: sym, reason: "fetch_failed", detail: "json_parse" },
                  };
                }
                if (!j || typeof j.ok !== "boolean") {
                  return {
                    sym,
                    payload: null,
                    issue: { symbol: sym, reason: "payload_not_ok", detail: "invalid_shape" },
                  };
                }
                if (!j.ok) {
                  return {
                    sym,
                    payload: null,
                    issue: { symbol: sym, reason: "payload_not_ok", detail: "ok_false" },
                  };
                }
                if (!Array.isArray(j.candles)) {
                  return {
                    sym,
                    payload: null,
                    issue: { symbol: sym, reason: "payload_not_ok", detail: "candles_not_array" },
                  };
                }
                if (j.candles.length === 0) {
                  return {
                    sym,
                    payload: null,
                    issue: { symbol: sym, reason: "payload_not_ok", detail: "empty_candles" },
                  };
                }
                if (j.candles.length < MIN_BARS) {
                  return {
                    sym,
                    payload: null,
                    issue: {
                      symbol: sym,
                      reason: "insufficient_bars",
                      detail: `${j.candles.length}/${MIN_BARS}`,
                    },
                  };
                }
                return { sym, payload: j, issue: null };
              } catch {
                return {
                  sym,
                  payload: null,
                  issue: { symbol: sym, reason: "fetch_failed", detail: "network" },
                };
              }
            }),
          );
          for (const r of results) {
            if (r.issue) symbolIssues.push(r.issue);
            if (r.payload) payloads.set(r.sym, r.payload);
          }
        }

        if (!mounted) return;

        const now = Date.now();
        const symbolsRequested = activeSymbols.length;
        const failingSymbols = activeSymbols.filter((s) => !payloads.has(s));
        let status: FuturesDataHealthStatus;
        if (payloads.size === 0) status = "stale";
        else if (failingSymbols.length > 0) status = "degraded";
        else status = "ok";

        if (status === "ok") notOkSinceRef.current = null;
        else if (notOkSinceRef.current === null) notOkSinceRef.current = now;

        const showFeedWarning =
          status !== "ok" &&
          notOkSinceRef.current !== null &&
          now - notOkSinceRef.current >= FUTURES_FEED_WARNING_AFTER_MS;

        const lastError =
          symbolIssues.length > 0
            ? symbolIssues
                .slice(0, 4)
                .map((i) => `${i.symbol}:${i.reason}${i.detail ? `(${i.detail})` : ""}`)
                .join("; ")
            : status !== "ok" && activeSymbols.length > 0
              ? "No symbol returned enough bars"
              : null;

        /** At least one symbol returned enough bars — drives quotes + entries (see statusRef note below). */
        const hasMarketData = payloads.size > 0;

        setDataHealth((prev) => ({
          status,
          lastError,
          lastOkAt: hasMarketData ? now : prev.lastOkAt,
          lastPollAt: now,
          failingSymbols,
          symbolIssues,
          payloadsReady: payloads.size,
          symbolsRequested,
          showFeedWarning,
        }));

        const primary =
          payloads.get(PRIMARY_QUOTE_SYMBOL) ?? payloads.values().next().value ?? null;
        if (primary) {
          setQuote({
            lastPrice: primary.lastPrice,
            markPrice: primary.markPrice,
            indexPrice: primary.indexPrice,
            changePct24h: primary.changePct24h,
            fundingRate: primary.fundingRate,
            nextFunding: primary.nextFunding,
            timestamp: new Date(primary.fetchedAt).getTime(),
          });
          setWarmingPct(Math.min(100, Math.round((primary.candles.length / MIN_BARS) * 100)));
        }

        if (hasMarketData && statusRef.current === "WARMING") {
          setStatus("READY");
        }

        if (primary && !fundingTickerMetaWarnedRef.current) {
          const nf = primary.nextFunding;
          if (!Number.isFinite(nf) || nf <= 0 || nf < 1_000_000_000) {
            fundingTickerMetaWarnedRef.current = true;
            console.warn(
              "[btc-futures-paper] Ticker missing or invalid next_funding_time; using 8h funding interval for calendar accrual (DELTA_PAPER_FUNDING_INTERVAL_MS).",
            );
          }
        }

        const prevPos = positionsRef.current;
        const markedPositions = prevPos.map((p) => {
          const d = payloads.get(p.symbol);
          if (!d) return p;
          return applyMarkToPosition(p, d.markPrice, d.lastPrice, {
            fundingRate: d.fundingRate,
            nowMs: now,
          });
        });

        const exitJobs: {
          pos: BTCFuturesPosition;
          exitPrice: number;
          reason: NonNullable<BTCFuturesPosition["exitReason"]>;
        }[] = [];

        const patchedPositions = markedPositions.map((pos) => {
          const d = payloads.get(pos.symbol);
          if (!d || d.candles.length < MIN_BARS) return pos;
          const closes = d.candles.map((c) => c.close);
          const opens = d.candles.map((c) => c.open);
          const highs = d.candles.map((c) => c.high);
          const lows = d.candles.map((c) => c.low);
          const volumes = d.candles.map((c) => c.volume);
          const input = buildSignalInputs(opens, closes, highs, lows, volumes, d.markPrice);
          const { patched, close } = resolveFuturesExitStep(pos, input, profileHoldTimeMulRef.current);
          if (close.shouldClose && close.reason) {
            exitJobs.push({ pos: patched, exitPrice: close.exitPrice, reason: close.reason });
          }
          return patched;
        });

        const exitingIds = new Set(exitJobs.map((j) => j.pos.id));
        const survivors = patchedPositions.filter((p) => !exitingIds.has(p.id));

        const equityForDrawdown =
          balanceRef.current + patchedPositions.reduce((s, p) => s + p.unrealizedPnl, 0);
        peakEquityForDrawdownRef.current = Math.max(peakEquityForDrawdownRef.current, equityForDrawdown);
        const ddPoll =
          peakEquityForDrawdownRef.current > 0
            ? ((peakEquityForDrawdownRef.current - equityForDrawdown) / peakEquityForDrawdownRef.current) * 100
            : 0;
        if (ddPoll >= MAX_DRAWDOWN_LOCK_PCT) drawdownEntryPausedRef.current = true;
        else if (ddPoll <= MAX_DRAWDOWN_LOCK_PCT * DRAWDOWN_LOCK_RECOVERY_FRAC) {
          drawdownEntryPausedRef.current = false;
        }

        setPositions(survivors);

        for (const job of exitJobs) {
          closePosition(job.pos, job.exitPrice, job.reason);
        }

        let openCount = survivors.length;
        const occupied = new Set(survivors.map((p) => `${p.symbol}:${p.strategyId}`));

        // Do not gate entries on statusRef === READY: setStatus is async and statusRef updates next render,
        // so same poll tick would never open trades. Use live payload presence instead.
        if (hasMarketData && !pauseRef.current && !drawdownEntryPausedRef.current) {
          for (const symbol of activeSymbols) {
            if (openCount >= MAX_OPEN_POSITIONS) break;
            const d = payloads.get(symbol);
            if (!d || d.candles.length < MIN_BARS) continue;

            const closes = d.candles.map((c) => c.close);
            const opens = d.candles.map((c) => c.open);
            const highs = d.candles.map((c) => c.high);
            const lows = d.candles.map((c) => c.low);
            const volumes = d.candles.map((c) => c.volume);
            const input = buildSignalInputs(opens, closes, highs, lows, volumes, d.markPrice);

            for (const strat of activeStratDefs) {
              if (openCount >= MAX_OPEN_POSITIONS) break;
              if (disabledRef.current.includes(strat.id)) continue;
              if (occupied.has(`${symbol}:${strat.id}`)) continue;
              const ck = `${symbol}:${strat.id}`;
              if ((stratCooldownsRef.current[ck] ?? 0) > Date.now()) continue;

              const signal = evalMinuteSignal(input, strat);
              if (signal.score >= activeSignalThreshold && passesEntryConfirmation(input, strat)) {
                const side = strat.signalKey.includes("SHORT") ? "SHORT" : "LONG";
                openPosition(strat, side, d.lastPrice, d.markPrice, symbol);
                occupied.add(`${symbol}:${strat.id}`);
                openCount++;
              }
            }
          }
        }
      } catch (e) {
        if (!mounted) return;
        const now = Date.now();
        const msg = e instanceof Error ? e.message : String(e);
        if (notOkSinceRef.current === null) notOkSinceRef.current = now;
        const showFeedWarning =
          notOkSinceRef.current !== null &&
          now - notOkSinceRef.current >= FUTURES_FEED_WARNING_AFTER_MS;
        setDataHealth((prev) => ({
          status: "stale",
          lastError: msg,
          lastOkAt: prev.lastOkAt,
          lastPollAt: now,
          failingSymbols: [...activeSymbols],
          symbolIssues: activeSymbols.map((s) => ({
            symbol: s,
            reason: "fetch_failed" as const,
            detail: "poll_exception",
          })),
          payloadsReady: 0,
          symbolsRequested: activeSymbols.length,
          showFeedWarning,
        }));
      }
    };

    poll();
    interval = setInterval(poll, POLL_MS);
    return () => {
      mounted = false;
      if (interval) clearInterval(interval);
    };
  }, [openPosition, closePosition, activeStratDefs, activeSymbols, activeSignalThreshold, strategyProfile]);

  // ========== ENGINE REF ==========
  useEffect(() => {
    engineRef.current = {
      positions,
      trades,
      balance,
      equity: balance + positions.reduce((s, p) => s + p.unrealizedPnl, 0),
      availableMargin: balance - positions.reduce((s, p) => s + p.marginUsed, 0),
      usedMargin: positions.reduce((s, p) => s + p.marginUsed, 0),
      stats: calculateStats(),
      quote,
      isReady: status === "READY",
      pauseEntries,
      disabledStrategies,
      togglePause,
      resetPaperAccount,
      clearTradeHistory,
      setDisabledStrategies: setDisabledStrategiesHandler,
      exportCSV,
      exportJSON,
      dataHealth,
    };
  }, [positions, trades, balance, quote, status, pauseEntries, disabledStrategies, calculateStats, togglePause, resetPaperAccount, clearTradeHistory, setDisabledStrategiesHandler, exportCSV, exportJSON, dataHealth]);

  // ========== DAILY RESET ==========
  useEffect(() => {
    const checkDay = () => {
      const now = new Date();
      const currentDate = now.getDate();
      if (currentDate !== dayStartDateRef.current) {
        setDayStartDate(currentDate);
        setDayStartBalance(balanceRef.current);
      }
    };
    const id = setInterval(checkDay, 60000);
    return () => clearInterval(id);
  }, []);

  const stats = calculateStats();
  const usedMargin = positions.reduce((s, p) => s + p.marginUsed, 0);
  const availableMargin = balance - usedMargin;
  const equity = balance + positions.reduce((s, p) => s + p.unrealizedPnl, 0);

  return {
    positions,
    trades,
    balance,
    equity,
    availableMargin,
    usedMargin,
    stats,
    quote,
    isReady: status === "READY",
    pauseEntries,
    disabledStrategies,
    engineRef,
    togglePause,
    resetPaperAccount,
    clearTradeHistory,
    setDisabledStrategies: setDisabledStrategiesHandler,
    exportCSV,
    exportJSON,
    strategyStatuses,
    strategyProfile,
    effectiveSignalThreshold: activeSignalThreshold,
    dataHealth,
  };
}

