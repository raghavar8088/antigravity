"use client";

import { useCallback, useEffect, useRef, useState, useMemo, type MutableRefObject } from "react";
import { PRIMARY_QUOTE_SYMBOL, TRADING_SYMBOLS } from "@/lib/futuresMarketData";
import { FUTURES_STRAT_DEFS, type FuturesStratDef, type RegimeTag } from "@/lib/futuresStrategies";
import {
  appendPrunedDeskRegimePersistEvent,
  buildPaperDeskStrategies,
  DESK_REGIME_HISTOGRAM_LS_WINDOW_MS,
  deskEffectiveHoldMinutesAtOpen,
  deskFakeDiversityEnabledViaEnv,
  deskHoldTuningAnalysisModeEnabled,
  deskHoldTuningExportIntervalMsFromEnv,
  deskMaxSameDirNotionalFracFromEnv,
  deskAutoDisableStratsEnabled,
  deskKillMaxExpectancyUsdFromEnv,
  deskKillMaxSumNetUsdFromEnv,
  deskKillMinTradesFromEnv,
  deskKillWindowDaysFromEnv,
  deskRiskPctOfEquityFromEnv,
  deskVolSizedNotionalEnabledFromEnv,
  deskEntryReplaceWeakestFromEnv,
  deskMaxLastMarkSpreadPctFromEnv,
  deskMaxOpenPerCategoryFromEnv,
  deskEntryUtcSessionFromEnv,
  deskForceProbeOpenEnabledFromEnv,
  formatDeskEntryUtcSessionLabel,
  isUtcHourInSession,
  canOpenCategory,
  countOpenByCategory,
  deskMinExpectedMoveSafetyKFromEnv,
  paperEntryPriorityScore,
  weakestEntryPriorityIndex,
  deskMinTpSlRatioFromEnv,
  deskAdaptiveTpEnabled,
  deskAllocationByEdgeEnabled,
  deskFirehoseModeEnabled,
  deskFixedNotionalPctOfEquity,
  deskInitialBalanceUsd,
  deskMaxOpenPerSideFromEnv,
  deskMaxOpenPerTemplateFromEnv,
  deskMaxOpenPositionsEffective,
  deskProfitLockMinNetUsdFromEnv,
  deskProfitLockMinProgressFromEnv,
  deskSessionGateEnabled,
  deskSlippageBpsFromEnv,
  deskRegimeHistogramDevPersistEnabled,
  deskRegimeWatchIntervalMsFromEnv,
  deskRegimeWatchPollWindowFromEnv,
  deskPaperMakerFillModelEnabled,
  deskPaperMakerFeePctFromEnv,
  deskPaperMakerFillProbabilityFromEnv,
  btcFtPaperMinHoldBeforeSlMinFromEnv,
  btcFtPaperQuickTpMinNetUsdFromEnv,
  btcFtPaperSlPctMulFromEnv,
  isBtcFtPaperDeskMode,
  paperDeskEffectiveStops,
  paperDeskWidenPositionStops,
  applyWinnersOnlyGate,
  checkEntryBurstGuard,
  createEntryBurstContext,
  recordBurstEntry,
  ENTRY_BURST_MAX_PER_FAMILY_DEFAULT,
  ENTRY_OPPOSITE_SIDE_WINDOW_MS,
  entryBurstMaxPerSymbolFromEnv,
  computeAdaptiveThreshold,
  computeWinRateAdaptiveThreshold,
  clearRuntimeBlocklist,
  getRuntimeBlocklist,
  isStrategyRuntimeBlocked,
  isSameSideCapped,
  MAX_SAME_SIDE_POSITIONS,
  type EntryBurstContext,
  type DeskRegimePersistEvent,
  type DeskEntryUtcSession,
  histogramRegimePolls,
  parseDeskRegimePersistLsPayload,
  regimeHistogramShares,
  serializeDeskRegimePersistLsPayload,
} from "@/lib/futuresDeskPolicy";
import { logSkipReason, getSkipReasonSummary, resetSkipReasonLog } from "@/lib/entrySkipReason";
import { createTraceRow, capTraceRows, summarizeSignalTrace, type StrategySignalTraceRow, type SignalTraceSummary } from "@/lib/strategySignalTrace";
import { scoreSignalQuality, type SignalQualityScore } from "@/lib/futuresSignalQuality";
import {
  buildTFSnapshotsFromInputMap,
  computeMTFConfluence,
  mtfSkipReason,
  type MTFConfluenceResult,
} from "@/lib/futuresMTFConfluence";
import { computeAttribution, type AttributionReport } from "@/lib/futuresAttribution";
import { deriveDeskRecommendation, type DeskRecommendation } from "@/lib/futuresDeskRecommendation";
import { computeGoLiveGates, type GoLiveGateReport } from "@/lib/futuresGoLiveGates";
import {
  applyProfitModeThreshold,
  profitModeAllowsRotationStrategy,
  profitModeAllowsStrategyInChop,
  profitModeAllocationByEdgeEnabled,
  profitModeCounterTrendSkip,
  profitModeDailyStratCap,
  profitModeExitConfig,
  profitModeFromEnv,
  profitModeMaxOpenPositions,
  profitModeMaxSameSide,
  profitModeMinExpectedMoveK,
  profitModeMtfMinScore,
  profitModePassesQuality,
  profitModeSessionGateEnabled,
  type ProfitModeConfig,
} from "@/lib/futuresProfitMode";
import {
  computeDeskRollingPnLScorecard,
  type DeskRollingPnLScorecard,
} from "@/lib/futuresDeskPnLTracker";
import { deriveScorecardAction, type ScorecardAction } from "@/lib/futuresScorecardActions";
import {
  appendSoakSnapshot,
  loadSoakHistory,
  saveSoakHistory,
  soakTrendSummary,
  type SoakDaySnapshot,
} from "@/lib/futuresSoakTracker";
import {
  computeUnifiedReadiness,
  type UnifiedReadiness,
} from "@/lib/futuresUnifiedReadiness";
import { recommendOneTune } from "@/lib/futuresParameterTuner";
import type { PaperTradeDbRow } from "@/lib/paperTradesTypes";
import {
  createEmptyEntryPollDebug,
  deskEntryDebugEnabledFromEnv,
  finalizeEntryPollDebug,
  type DeskEntryPollDebug,
} from "@/lib/futuresEntryDebug";
import { funnelSnapshotFromBrowserDebug } from "@/lib/deskEntryFunnelSnapshot";
import { isWorkerHeartbeatFresh } from "@/lib/paperDeskWorker/workerLease";
import {
  buildDeskHoldTuningDumpPayload,
  rankWorstTimeOffenders,
  reduceTradesToStrategyDeskBuckets,
  type PrimaryRegimeHistogram24h,
  type PrimaryRegimePollWatch,
  type WorstTimeOffenderRow,
} from "@/lib/futuresHoldTuningAnalysis";
import {
  buildSignalInputs,
  classifyRegimeTagFrom1mOhlcv,
  effectiveSignalThreshold as computeEffectiveThreshold,
  evalMinuteSignal,
  passesEntryConfirmation,
  passesRelaxedDeskEntryConfirmation,
  type FuturesSignalInputs,
} from "@/lib/futuresSignals";
import {
  computeSessionEquityFromProduction,
  computeSessionExitReasonAnalytics,
  computeSessionTradingMetrics,
  formatExitReasonSessionSummary,
  FUTURES_STRATEGY_PROFILES,
  effectiveMinExpectedMoveSafetyK,
  isProbeOrBootstrapTrade,
  resolveStrategyProfile,
  type FuturesStrategyProfile,
} from "@/lib/futuresSessionMetrics";
import {
  paperApplyEntrySlippage,
  paperApplyExitSlippage,
  paperContracts,
  paperEstimatedMaxLossAtStopSl,
  paperLiquidationDistancePct,
  paperLiquidationPrice,
  paperMarginRequired,
  paperNetPnlOnClose,
  paperNotional,
  paperNotionalForTargetRisk,
  paperPriceMovePctOnNotional,
  paperLastMarkSpreadPct,
  paperMinExpectedMoveVsFees,
  paperSameDirNotionalWouldExceedCap,
} from "@/lib/futuresPaperMath";
import {
  applyMarkToFuturesPosition,
  resolveFuturesExitStep,
} from "@/lib/futuresDeskRuntime";
import type {
  FuturesDataHealth,
  FuturesDataHealthStatus,
  FuturesDataHealthSymbolIssue,
} from "@/lib/futuresDataHealth.types";
import { FUTURES_FEED_WARNING_AFTER_MS } from "@/lib/futuresDataHealth.types";
import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import {
  buildStratDisableSetFromStats,
  formatAutoDisabledStratIds,
  mergeDisabledStrategyIds,
  type StrategyStatRow,
} from "@/lib/paperTradesAnalytics";
import type { PaperTradeModuleKey } from "@/lib/paperTradesTypes";
import { resolveCloudPaperTradesAccountKey } from "@/lib/paperTradesAuth";
import { getOrCreateAnonAccountKey } from "@/lib/anonAccountKey";
import {
  flushTradeSyncQueue,
  persistTradeToServer,
  pullAndMergePaperTrades,
} from "@/lib/paperTradesSync";
import {
  isDeskShadowLogOpenEnabled,
  shadowIntentFromPaperClose,
  shadowIntentFromPaperOpen,
} from "@/lib/shadowTradeIntentMapper";
import { persistShadowTradeIntent } from "@/lib/shadowTradeIntentSync";
import { btcFtTemplateFamilyKey, deskMinAbsNetWinUsd, researchDailyStratCap } from "@/lib/btcFtResearch";
import { PREMIUM_NOTIONAL_MULTIPLIER, isPremiumStrategy } from "@/lib/btcFtPremiumStrategies";
import { btcFtUseRankedEnabled, winnerIdsFromRankings, type BtcFtStrategyRankingRow } from "@/lib/btcFtRoster";
import {
  atrPctFromAtr,
  computeAdaptiveTpPct,
  computeAllocationMultiplier,
  computeStrategyEdgeStats,
} from "@/lib/strategyAllocation";
import {
  computeStrategyHourlyStats,
  isStrategyInProvenSession,
} from "@/lib/strategySessionStats";
import { markAgeMs, setCanonicalMark } from "@/lib/canonicalMarkPrice";
import { runProductionReadiness } from "@/lib/futuresProductionReadiness";
import {
  getSuspendedStrategyIds,
  type RotationReport,
} from "@/lib/futuresStrategyRotation";
import {
  computeStrategyDiagnostics,
  computeRollingHealthCheck,
  computeStrategyDiagnosticsFromEngineTrades,
  computeRollingHealthCheckFromEngineTrades,
  type DiagnosticSummary,
  type HealthCheckResult,
} from "@/lib/futuresStrategyDiagnostics";

export type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
export type { FuturesStrategyProfile } from "@/lib/futuresSessionMetrics";
export type { WorstTimeOffenderRow } from "@/lib/futuresHoldTuningAnalysis";
export type { RegimeTag } from "@/lib/futuresStrategies";
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
const MAKER_FEE_PCT = 0.0005; // 0.05% maker (Delta Exchange)
const RAW_TAKER_FEE_PCT = 0.001; // 0.10% taker (market orders)
// When maker-first fill model is ON, blend taker fee down based on expected fill probability.
const TAKER_FEE_PCT: number = (() => {
  if (!deskPaperMakerFillModelEnabled()) return RAW_TAKER_FEE_PCT;
  const pMaker = deskPaperMakerFillProbabilityFromEnv();
  const makerFee = deskPaperMakerFeePctFromEnv();
  return pMaker * makerFee + (1 - pMaker) * RAW_TAKER_FEE_PCT;
})();
const ROUND_TRIP_FEE_FRAC = TAKER_FEE_PCT * 2;
const FEE_BREAKEVEN_PCT = ROUND_TRIP_FEE_FRAC * 100;

// Account settings
// Initial paper balance — env override via NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD so
// research mode can run $1M with 1%-of-equity notional ($10K/trade) without
// recompiling. Resolved per-hook-instance below so tests can stub the env.
const INITIAL_BALANCE_DEFAULT = 1000;
const MIN_CONTRACTS = 1;
const MAX_CONTRACTS = 50_000;
/** Concurrent position ceiling. Resolved per-hook-instance via
 *  `deskMaxOpenPositionsEffective()` so firehose mode can raise it to 60. */
const MAX_OPEN_POSITIONS_FALLBACK = 12;
const CONTRACT_SIZE = 1; // 1 USD per contract on Delta

// Risk management
const MAX_DRAWDOWN_LOCK_PCT = 25; // Pause entries if drawdown > 25%
/** Hysteresis: resume entries when drawdown falls to this fraction of the lock threshold (e.g. 0.84 → 21%). */
const DRAWDOWN_LOCK_RECOVERY_FRAC = 0.84;
/** Intraday soft DD lock (P2.3.4): pause entries when daily P&L drops to −2% of day-start equity. */
const INTRADAY_SOFT_DD_LOCK_PCT = 2;
const MAX_LOSS_PER_TRADE_PCT = 2; // Max 2% of balance at risk (SL + fees) per new position; skip if exceeded
const MIN_POSITION_NOTIONAL = 100;
const MAX_POSITION_NOTIONAL_DEFAULT = 500;
/**
 * Per-trade max notional. Firehose mode raises this to $20K so 1%-of-$1M
 * sizing ($10K) fits comfortably. Production keeps the $500 micro-cap.
 */
function maxPositionNotionalEffective(): number {
  if (deskFirehoseModeEnabled() || deskFixedNotionalPctOfEquity() > 0) {
    return 20_000;
  }
  return MAX_POSITION_NOTIONAL_DEFAULT;
}
/**
 * Stats only: mark is within this % of modeled liquidation (see paperLiquidationDistancePct).
 */
const LIQUIDATION_RISK_DISPLAY_PCT = 1.0;

// Strategy parameters
// Paper desk default: strict HTF rules used to reject almost all MTF entries when 5m/15m was NEUTRAL.
const SIGNAL_THRESHOLD = 28;
const MAX_BARS = 120;
const MIN_BARS = 15;
/** Slightly slower poll: many symbols × REST calls per tick */
const POLL_MS = 4_000;
const SYMBOL_FETCH_CHUNK = 5;
const MAX_TRADES = 2_000;

/**
 * PR 10 — multi-bar-interval engine routing.
 *
 * Bar intervals supported by the engine. Each research-pool strategy declares
 * a `primaryBarInterval`; the polling loop fetches every interval needed by
 * the active roster, and signal eval routes each strategy to its TF cache.
 *
 * "1m" is always fetched — needed for regime classification, mark price, exit
 * pipeline, and probe/bootstrap entries.
 */
type BarInterval = "1m" | "5m" | "15m" | "4h" | "1d";
const BAR_INTERVALS_SUPPORTED: readonly BarInterval[] = ["1m", "5m", "15m", "4h", "1d"];
function isBarInterval(v: unknown): v is BarInterval {
  return typeof v === "string" && (BAR_INTERVALS_SUPPORTED as readonly string[]).includes(v);
}

/**
 * Per-TF poll cadence (ms). Picked as the minimum across categories that use
 * each TF so the fastest strategy on a TF still gets the data it needs.
 *
 *   1m  → 4s   (scalping: 1m bars tick every minute, 15 polls/bar is fine)
 *   5m  → 15s  (day/breakout/momentum: 20 polls/bar)
 *   15m → 15s  (trend/range: 60 polls/bar — same fetch as 5m, no reason slower)
 *   4h  → 60s  (swing: 240 polls/bar of 4h — diminishing returns past 60s)
 *   1d  → 300s (position: 288 polls/bar of 1d — only need a few/day)
 *
 * Skipping a fetch when its cadence hasn't elapsed cuts HTTP traffic
 * 5–60× per non-1m TF without losing signal freshness.
 */
const INTERVAL_POLL_MS: Record<BarInterval, number> = {
  "1m": 4_000,
  "5m": 15_000,
  "15m": 15_000,
  "4h": 60_000,
  "1d": 300_000,
};

/** Shared between the polling effect and the cached-payload refs. */
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

// Exit management
const MIN_ABS_NET_PNL_USD = 2;
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
  /** P1-F: rank at open for slot competition when `MAX_OPEN_POSITIONS` is full. */
  entryPriorityScore?: number;
  /** Signal feature contributions at entry — for offline weight calibration (P0.1.1). */
  signalContributions?: Array<{ reason: string; pts: number }>;
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
  addDisabledStrategyIds: (ids: number[]) => void;
  updateSuspendedStrategies: (report: RotationReport) => void;
  restoreRotationStrategy: (strategyId: number) => void;
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
  /** Built strats that received default `regimes[]` from category map (defs omitted regimes). */
  deskRegimeAnnotatedStratCount: number;
  /** Opens where `scalp_aggro_v1` + desk-widened TP bumped base `holdMinutes` before `holdTimeMul`. */
  deskProfileAdjustedHoldAppliedCount: number;
  /** Last-N closed trades: exit reason × count and mean net (grouped expectancy). */
  sessionExitReasonSummary: string;
  /** Last-N closed: top TIME bleeders by total net at TIME (read-only tuning hint). */
  sessionWorstTimeOffenders: WorstTimeOffenderRow[];
  /** Last sampled 15m regime (primary symbol poll). */
  deskLastRegimeTag: RegimeTag;
  /** Skips: ATR$ move below K× round-trip fees. */
  deskSkippedMinExpectedMove: number;
  /** Skips: same-side notional would exceed equity × frac. */
  deskSkippedSameDirCap: number;
  /** Skips: strategy `regimes` filter. */
  deskSkippedByRegime: number;
  /** e.g. `chop×3 · trendLow×1` */
  deskSkippedByRegimeBreakdown: string;
  /** `NEXT_PUBLIC_DESK_AUTO_DISABLE_STRATS=1` */
  deskAutoDisableEnabled: boolean;
  /** Rolling-window auto-disabled strategy count (Supabase analytics). */
  deskAutoDisabledStratCount: number;
  /** Comma-separated auto-disabled IDs (truncated for UI). */
  deskAutoDisabledStratIds: string;
  /** Window days used for kill-switch fetch. */
  deskKillWindowDays: number;
  /** Vol-sized notional from env (`NEXT_PUBLIC_DESK_VOL_SIZED_NOTIONAL=1`). */
  deskVolSizedNotionalEnabled: boolean;
  /** Opens sized via `paperNotionalForTargetRisk` this session. */
  deskVolSizedNotionalAppliedCount: number;
  /** Skips: full slots and candidate lost priority queue (or lost vs incumbent). */
  deskSkippedLowPriorityEntry: number;
  /** `NEXT_PUBLIC_DESK_ENTRY_REPLACE_WEAKEST=1` — swap weakest open at max slots. */
  deskEntryReplaceWeakestEnabled: boolean;
  /** Skips: |last−mark|/mark % above desk cap. */
  deskSkippedSpread: number;
  /** Max last/mark spread % (`NEXT_PUBLIC_DESK_MAX_LAST_MARK_SPREAD_PCT`). */
  deskMaxLastMarkSpreadPct: number;
  /** Skips: category already at max concurrent opens. */
  deskSkippedCategoryCap: number;
  /** Per-category open cap (`NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY`). */
  deskMaxOpenPerCategory: number;
  /** Skips: poll tick outside UTC entry session window. */
  deskSkippedOutsideSession: number;
  /** Active UTC entry window label for UI. */
  deskEntryUtcSessionLabel: string;
  /** Skips: strategy hit its per-UTC-day close cap (research-mode anti-churn). */
  deskSkippedDailyStratCap: number;
  /** Skips: another open position already covers this template family (research-mode anti-churn). */
  deskSkippedTemplateFamily: number;
  /** Skips: max-open-per-side cap reached (#5 correlation-aware cap). */
  deskSkippedSideCap: number;
  /** Skips: max-open-per-template-family cap reached (#5 stricter than research dedup). */
  deskSkippedTemplateCap: number;
  /** Skips: strategy not in proven session for this UTC hour (#4). */
  deskSkippedOutsideProvenSession: number;
  /** Count of opens where allocation multiplier scaled notional up (>1.05) or down (<0.95). */
  deskAllocationScaledCount: number;
  /** Count of opens where adaptive TP widened or tightened the strategy's nominal TP. */
  deskAdaptiveTpAppliedCount: number;
  /** Skips: per-symbol burst cap (>= maxPerSymbolPerPoll new entries for same symbol this tick). */
  deskBurstBlockedCount: number;
  /** Skips: per-family burst cap (>= maxPerFamilyPerPoll new entries for same template family this tick). */
  deskFamilyCapBlockedCount: number;
  /** Skips: same symbol has open or recently-closed opposite-side position (chop guard). */
  deskOppositeSideBlockedCount: number;
  /** Skips: PR-6 same-side correlated position count cap (MAX_SAME_SIDE_POSITIONS). */
  deskSkippedCorrelatedCap: number;
  /** Last-60s skip reason frequency (PR-6 entry quality diagnostics). */
  skipReasonSummary: Array<{ reason: string; count: number }>;
  /** Per-strategy aggregated performance (PR-7 edge diagnostics). Null until >= 5 production trades. */
  strategyDiagnostics: DiagnosticSummary | null;
  /** Rolling 20-trade health check with grade A/B/C/F (PR-7). Null until >= 5 production trades. */
  rollingHealthCheck: HealthCheckResult | null;
  /** PR-8: monitor-driven runtime blocks (session-only, not persisted). */
  runtimeBlocklistIds: number[];
  /** PR-12: latest rotation report from monitor (null until diagnostics poll). */
  rotationReport: RotationReport | null;
  /** PR-12: count of strategies suspended by rotation engine this session. */
  suspendedStrategyCount: number;
  /** PR-13: performance attribution by hold, side, exit, hour (>= 10 production trades). */
  attributionReport: AttributionReport | null;
  /** PR-14: last entry candidate signal quality breakdown. */
  lastSignalQuality: SignalQualityScore | null;
  /** PR-14: last entry candidate MTF confluence result. */
  lastMtfConfluence: MTFConfluenceResult | null;
  /** PR-14: session count of quality-gate rejects. */
  qualitySkipCount: number;
  /** PR-14: session count of MTF-gate rejects. */
  mtfSkipCount: number;
  /** PR-14: session count of PR-13 gate evaluations (quality + MTF). */
  gateEvaluationCount: number;
  /** PR-14: closed-loop desk recommendation (never auto-applied). */
  deskRecommendation: DeskRecommendation | null;
  /** PR-15: go-live gates vs LIVE_TRADING_PHASE.md §3 (read-only). */
  goLiveGates: GoLiveGateReport | null;
  profitModeEnabled: boolean;
  profitModeSkipCount: number;
  profitModeExitActive: boolean;
  /** Count of strategies allowed by the rotation gate this session (profit mode ON). */
  entryRotationAllowedCount: number;
  /** Count of strategies blocked by the rotation gate this session (profit mode ON). */
  entryRotationBlockedCount: number;
  deskPnLScorecard: DeskRollingPnLScorecard | null;
  scorecardAction: ScorecardAction | null;
  /** PR-19: last 7–14 UTC daily soak snapshots (localStorage). */
  soakHistory: SoakDaySnapshot[];
  soakSummary: ReturnType<typeof soakTrendSummary>;
  unifiedReadiness: UnifiedReadiness;
  unifiedReadinessBlockers: string[];
  unifiedReadinessNextStep: string;
  replaySignFlipRate: number | null;
  /** |raw balance − production-expected balance|. > $1 means probes or corruption inflated balance. */
  balanceDriftUsd: number;
  /** Epoch ms of the last VPS worker poll (null = no worker or never polled). */
  workerLastPollAt: number | null;
  /** Who currently owns execution: "vps" = headless worker, "browser" = this tab, null = unknown. */
  workerOwner: "browser" | "vps" | null;
  /** True when worker heartbeat is fresh and this browser is in read-only monitor mode. */
  workerMonitorMode: boolean;
}

/** Strategy Status */
export interface BTCFuturesStrategyStatus {
  id: number;
  name: string;
  category: string;
  /**
   * - OPEN: currently holds at least one position
   * - COOLING: in per-strategy cooldown window
   * - AVAILABLE: in the active roster, ready to fire
   * - POOL: registered in the research pool but NOT in the active batch (display-only)
   */
  status: "OPEN" | "COOLING" | "AVAILABLE" | "POOL";
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
  /** Supabase auth user id — cloud `account_key` when set; local paper works when null. */
  supabaseUserId?: string | null;
  /** Optional strategy allow-list. If empty, full strategy library runs. */
  strategyIds?: number[];
  /** Optional symbol allow-list. If empty, full trading symbol list runs. */
  symbols?: readonly string[];
  /** Optional module-specific signal threshold override. */
  signalThreshold?: number;
  /** Paper-only: tune entry cadence / cooldown / time exits without editing STRAT_DEFS. */
  strategyProfile?: FuturesStrategyProfile;
  /** Dev-only diagnostic for BTC Future Trading: loosen template confirmation while keeping score gates. */
  relaxEntryConfirmation?: boolean;
  /** Dev-only diagnostic: open one tiny paper probe after market data is ready. */
  forceProbeOpen?: boolean;
  /** Research tournament: shorten strategy cooldowns without editing definitions. */
  cooldownMultiplier?: number;
  /** Research tournament: multiply the ATR-vs-fee movement hurdle. */
  minMoveKMultiplier?: number;
  /** Research tournament: paper fill slippage override in bps. */
  slippageBpsOverride?: number;
  /** Research tournament: do not apply Supabase auto-kill while collecting samples. */
  disableAutoKill?: boolean;
  /** Research tournament: lower threshold by 4 for zero-trade active strats after two hours. */
  researchEnsureTrades?: boolean;
  /** BTC FT paper desk: lower threshold for quiet strats after ~45m (default on module). */
  paperEnsureTrades?: boolean;
  /** Force entry debug panel (BTC FT module sets true). */
  entryDebugEnabled?: boolean;
  /**
   * Paper-only: after ~8m with zero closed trades, open one probe position to prove
   * the ledger path (works in production, not only NODE_ENV=development).
   */
  paperBootstrapProbe?: boolean;
  /** Module-specific UTC entry session override. */
  entryUtcSessionOverride?: DeskEntryUtcSession;
  /**
   * Module identifier persisted with each closed paper trade so dashboards
   * can filter leaderboards / exports per workspace tab. Optional —
   * legacy callers without a moduleKey keep working (row stored without it).
   */
  moduleKey?: PaperTradeModuleKey;
  /**
   * IDs of strategies with verified positive expectancy (from MongoDB promotions
   * or `winnerIdsFromRankings`). When set, premium strategies (tier: "premium") only
   * receive 2× notional if their ID appears here — otherwise they trade at 1× until
   * they earn it.
   */
  promotedStrategyIds?: ReadonlySet<number> | readonly number[];
  /** When true: clear manual/auto disables and show every roster strategy as AVAILABLE (unless OPEN/COOLING). */
  allStrategiesAvailable?: boolean;
};

/** Progressive threshold reduction so quiet strats get samples on chop. */
export function paperEnsureThresholdDrop(
  enabled: boolean,
  nowMs: number,
  mountedAtMs: number,
  stratTradeCount: number,
): number {
  if (!enabled || stratTradeCount >= 5) return 0;
  const mins = (nowMs - mountedAtMs) / 60_000;
  let drop = 0;
  if (mins >= 10) drop = 12;
  else if (mins >= 5) drop = 10;
  else if (mins >= 2) drop = 6;
  // Strats that already fired once still get half the drop so the roster keeps rotating.
  if (stratTradeCount > 0) drop = Math.floor(drop / 2);
  return drop;
}

/** When the desk is idle (no opens), nudge threshold down so paper keeps sampling. */
export function paperQuietEntryBoost(
  enabled: boolean,
  nowMs: number,
  lastClosedMs: number,
  openCount: number,
): number {
  if (!enabled || openCount > 0) return 0;
  const quietMs = lastClosedMs > 0 ? nowMs - lastClosedMs : 0;
  if (quietMs >= 10 * 60_000) return 6;
  if (quietMs >= 5 * 60_000) return 4;
  if (quietMs >= 2 * 60_000) return 3;
  return 0;
}

export function paperEnsureEffectiveThresholdDrop(
  profitModeEnabled: boolean,
  ensureDataThresholdDrop: number,
  productionTradeCount: number,
): number {
  if (!profitModeEnabled) return ensureDataThresholdDrop;
  // Profit mode needs a small amount of production evidence before strict
  // scorecard gates can judge it. Keep discovery capped so it cannot become
  // firehose behavior.
  if (productionTradeCount < 10) return Math.min(ensureDataThresholdDrop, 8);
  return 0;
}

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
  addDisabledStrategyIds: (ids: number[]) => void;
  updateSuspendedStrategies: (report: RotationReport) => void;
  restoreRotationStrategy: (strategyId: number) => void;
  exportCSV: () => string;
  exportJSON: () => string;
  strategyStatuses: BTCFuturesStrategyStatus[];
  strategyProfile: FuturesStrategyProfile;
  effectiveSignalThreshold: number;
  dataHealth: FuturesDataHealth;
  /** Last poll entry funnel (when `NEXT_PUBLIC_DESK_ENTRY_DEBUG=1`). */
  entryDebug: DeskEntryPollDebug | null;
  /** PR-19: feed replay sign-flip from monitor (throttled API fetch). */
  setReplaySignFlipRate: (rate: number | null) => void;
} {
  const storageNamespace = options.storageNamespace?.trim() || "btc_futures_scalper";
  const cloudAccountKey = useMemo(
    () =>
      resolveCloudPaperTradesAccountKey({
        supabaseUserId: options.supabaseUserId,
        storageNamespace,
      }) ?? getOrCreateAnonAccountKey(),
    [options.supabaseUserId, storageNamespace],
  );
  const strategyIds = options.strategyIds ?? null;
  const symbols = options.symbols ?? null;
  const strategyProfile = useMemo(
    () => resolveStrategyProfile(options.strategyProfile),
    [options.strategyProfile],
  );
  const relaxEntryConfirmation = options.relaxEntryConfirmation === true;
  const paperBootstrapProbe = options.paperBootstrapProbe === true;
  const forceProbeOpen =
    paperBootstrapProbe ||
    (process.env.NODE_ENV === "development" &&
      (options.forceProbeOpen === true || deskForceProbeOpenEnabledFromEnv()));
  const cooldownMultiplier =
    Number.isFinite(options.cooldownMultiplier) && (options.cooldownMultiplier ?? 0) > 0
      ? Math.min(2, Math.max(0.1, options.cooldownMultiplier as number))
      : 1;
  const minMoveKMultiplier =
    Number.isFinite(options.minMoveKMultiplier) && (options.minMoveKMultiplier ?? 0) > 0
      ? Math.min(2, Math.max(0.5, options.minMoveKMultiplier as number))
      : 1;
  const slippageBpsOverride =
    Number.isFinite(options.slippageBpsOverride) && (options.slippageBpsOverride ?? -1) >= 0
      ? Math.min(50, Math.max(0, options.slippageBpsOverride as number))
      : null;
  const disableAutoKill = options.disableAutoKill === true || options.allStrategiesAvailable === true;
  const allStrategiesAvailable = options.allStrategiesAvailable === true;
  const researchEnsureTrades = options.researchEnsureTrades === true;
  const paperEnsureTrades = options.paperEnsureTrades === true;
  const moduleKey = options.moduleKey;
  const paperDeskMode = isBtcFtPaperDeskMode({
    paperEnsureTrades,
    allStrategiesAvailable,
    moduleKey,
  });
  const disableIntradayDdLock = paperDeskMode;
  const entryUtcSessionOverride = options.entryUtcSessionOverride;
  const promotedIdsSet = useMemo((): ReadonlySet<number> => {
    const raw = options.promotedStrategyIds;
    if (!raw) return new Set();
    if (raw instanceof Set) return raw as ReadonlySet<number>;
    return new Set(raw as readonly number[]);
  }, [options.promotedStrategyIds]);
  const relaxEntryGatesRef = useRef(relaxEntryConfirmation);
  useEffect(() => {
    relaxEntryGatesRef.current = relaxEntryConfirmation;
  }, [relaxEntryConfirmation]);
  const profileCfg = FUTURES_STRATEGY_PROFILES[strategyProfile];
  const activeSignalThreshold = useMemo(() => {
    const base = Number.isFinite(options.signalThreshold)
      ? Number(options.signalThreshold)
      : SIGNAL_THRESHOLD;
    return computeEffectiveThreshold(base, profileCfg.signalThresholdDelta);
  }, [options.signalThreshold, profileCfg.signalThresholdDelta]);
  const stateStorageKey = `${storageNamespace}_paper_state`;
  const regimeHistogramLsKey = `${storageNamespace}_desk_regime_24h_v1`;

  const DESK_REGIME_LS_SAVE_INTERVAL_MS = 15_000;

  // Ranked winners from /api/strategy-rankings — populated when NEXT_PUBLIC_BTC_FT_USE_RANKED=1
  const [rankedWinnerIds, setRankedWinnerIds] = useState<ReadonlySet<number>>(new Set());
  const [strategyRankings, setStrategyRankings] = useState<ReadonlyArray<BtcFtStrategyRankingRow>>([]);

  const deskStrategiesResult = useMemo(() => {
    const allow = strategyIds && strategyIds.length > 0 ? new Set(strategyIds) : null;
    let raw = !allow ? [...STRAT_DEFS] : STRAT_DEFS.filter((s) => allow.has(s.id));
    if (raw.length === 0) raw = [...STRAT_DEFS];
    // Apply winners gate ONLY when no explicit allowlist was provided. When a
    // caller (e.g. BTCFutureTradingScalper) explicitly passes strategyIds, it
    // has already decided which strategies should be live — the MongoDB-ranked
    // winners gate must not further shrink that set, otherwise the research
    // pool can never reach AVAILABLE status in the leaderboard.
    const gated =
      btcFtUseRankedEnabled() && rankedWinnerIds.size > 0 && !allow
        ? applyWinnersOnlyGate(raw, rankedWinnerIds)
        : raw;
    const base = gated.length > 0 ? gated : raw;
    // Inject per-strategy adaptive thresholds (P1.1.2) when rankings are available
    const baseThreshold = 28;
    const rankingById = new Map(strategyRankings.map((r) => [r.id, r]));
    const withThresholds = base.map((s) => {
      const row = rankingById.get(s.id);
      if (!row) return s;
      const adaptive = computeWinRateAdaptiveThreshold(
        baseThreshold,
        row.winRate ?? null,
        row.trades,
      );
      return adaptive !== null ? { ...s, dynamicThreshold: adaptive } : s;
    });
    return buildPaperDeskStrategies(withThresholds, {
      strategyIdAllowlist: null,
      minTpSlRatio: deskMinTpSlRatioFromEnv(),
      // Explicit module rosters (e.g. BTC Future Trading 91–96) must not be dropped: IDs 91–96
      // sit inside the global 79–110 fake-diversity range but have full signal wiring.
      allowFakeDiversity:
        deskFakeDiversityEnabledViaEnv() || Boolean(strategyIds && strategyIds.length > 0),
    });
  }, [strategyIds, rankedWinnerIds, strategyRankings]);

  const activeStratDefs = deskStrategiesResult.strategies;
  const deskPolicySnapshot = useMemo(
    () => ({
      tpWidenedCount: deskStrategiesResult.tpWidenedStratIds.length,
      lowRrSkippedCount: deskStrategiesResult.lowRrSkippedStratIds.length,
      fakeDiversityFiltered: deskStrategiesResult.fakeDiversityFilteredCount,
      tpWidenedIds: [...deskStrategiesResult.tpWidenedStratIds].sort((a, b) => a - b).join(","),
      lowRrSkippedIds: [...deskStrategiesResult.lowRrSkippedStratIds].sort((a, b) => a - b).join(","),
      regimeAnnotatedCount: deskStrategiesResult.deskRegimeAnnotatedStratCount,
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

  /**
   * PR 10 — bar intervals required by the active strategy roster.
   *
   * Always includes "1m" (regime, mark, exits, probe). For each strategy adds
   * its `primaryBarInterval` and `confirmBarInterval` when present. Unknown
   * intervals are dropped — the engine silently falls back to "1m" for those
   * strategies (safe default).
   */
  const requiredBarIntervals = useMemo<BarInterval[]>(() => {
    const set = new Set<BarInterval>(["1m"]);
    for (const def of activeStratDefs) {
      if (isBarInterval(def.primaryBarInterval)) set.add(def.primaryBarInterval);
      if (isBarInterval(def.confirmBarInterval)) set.add(def.confirmBarInterval);
    }
    return BAR_INTERVALS_SUPPORTED.filter((i) => set.has(i));
  }, [activeStratDefs]);

  // State
  // Resolve env-tunable initial balance once per hook instance (stable across renders).
  // Default 1000; firehose / research can set NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD=1000000.
  const INITIAL_BALANCE = useMemo(() => deskInitialBalanceUsd() || INITIAL_BALANCE_DEFAULT, []);
  const profitModeCfg = useMemo(() => profitModeFromEnv(), []);
  // Resolve concurrent-positions ceiling once. Firehose mode raises to 60 (vs default 12).
  const MAX_OPEN_POSITIONS = useMemo(
    () =>
      profitModeMaxOpenPositions(
        deskMaxOpenPositionsEffective() || MAX_OPEN_POSITIONS_FALLBACK,
        profitModeCfg,
      ),
    [profitModeCfg],
  );
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
  const [autoDisabledStratIds, setAutoDisabledStratIds] = useState<number[]>([]);
  const [lastTradeAt, setLastTradeAt] = useState(0);
  const [dayStartBalance, setDayStartBalance] = useState(INITIAL_BALANCE);
  const [dayStartDate, setDayStartDate] = useState(() => new Date().getDate());
  const entryDebugEnabled =
    options.entryDebugEnabled === true || deskEntryDebugEnabledFromEnv();
  const [entryDebug, setEntryDebug] = useState<DeskEntryPollDebug | null>(null);
  // ── Worker monitor mode ──────────────────────────────────────────────────
  // When NEXT_PUBLIC_DESK_WORKER_ENABLED=1 and the VPS worker heartbeat is fresh,
  // the browser skips local poll WRITES and hydrates from Mongo only (read-only mode).
  const workerEnabled = process.env.NEXT_PUBLIC_DESK_WORKER_ENABLED === "1";
  const [workerLastPollAt, setWorkerLastPollAt] = useState<number | null>(null);
  const [workerOwner, setWorkerOwner] = useState<"browser" | "vps" | null>(null);
  const workerMonitorMode =
    workerEnabled &&
    workerLastPollAt !== null &&
    Date.now() - workerLastPollAt < 45_000;

  // Expose worker status for UI — both fields available via engine stats
  // (see calculateStats return below).
  const entryDebugLiveRef = useRef<DeskEntryPollDebug | null>(null);
  const forceProbeOpenedRef = useRef(false);
  const pollInFlightRef = useRef(false);
  const entryDebugLoggedOnceRef = useRef(false);
  const researchMountedAtRef = useRef(Date.now());
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

  /** PR 10 (poll cadence) — per-TF kline cache that persists across polls. The
   *  polling effect re-fetches 1m every tick (4s) but only re-fetches non-1m
   *  TFs when their cadence has elapsed (see INTERVAL_POLL_MS). Between fetches
   *  the previous payloads are reused, so signal eval always has data even on
   *  ticks where, say, 4h wasn't refreshed.                                  */
  const cachedPayloadsRef = useRef<Map<BarInterval, Map<string, KlinePayload>>>(new Map());
  const lastFetchedByIntervalRef = useRef<Map<BarInterval, number>>(new Map());
  const fundingTickerMetaWarnedRef = useRef(false);
  const peakEquityForDrawdownRef = useRef(INITIAL_BALANCE);
  const drawdownEntryPausedRef = useRef(false);
  // Intraday soft DD lock — pauses entries when day-over-day equity drops 2%; resets at UTC midnight.
  const dailyStartEquityRef = useRef(INITIAL_BALANCE);
  const dailyStartUtcDayRef = useRef<string>(new Date().toISOString().slice(0, 10));
  const intradayDdPausedRef = useRef(false);
  const positionsRef = useRef(positions);
  const tradesRef = useRef(trades);
  const balanceRef = useRef(balance);
  const disabledRef = useRef(disabledStrategies);
  const autoDisabledRef = useRef<Set<number>>(new Set());
  const autoDisableFetchLoggedRef = useRef(false);
  const pauseRef = useRef(pauseEntries);
  const lastTradeAtRef = useRef(lastTradeAt);
  const stratCooldownsRef = useRef<Record<string, number>>({});
  const suspendedStrategiesRef = useRef<Set<number>>(new Set());
  const rotationReportRef = useRef<RotationReport | null>(null);
  const rotationOverrideRef = useRef<Set<number>>(new Set());
  const lastSignalQualityRef = useRef<SignalQualityScore | null>(null);
  const lastMtfConfluenceRef = useRef<MTFConfluenceResult | null>(null);
  const tickSignalTraceRef = useRef<StrategySignalTraceRow[]>([]);
  const latestSignalTraceSummaryRef = useRef<SignalTraceSummary | null>(null);
  const qualitySkipCountRef = useRef(0);
  const mtfSkipCountRef = useRef(0);
  const profitModeSkipCountRef = useRef(0);
  const entryRotationAllowedCountRef = useRef(0);
  const entryRotationBlockedCountRef = useRef(0);
  const replaySignFlipRateRef = useRef<number | null>(null);
  const lastSoakLogDateRef = useRef<string | null>(null);
  const soakHistoryRef = useRef<SoakDaySnapshot[]>([]);
  const setReplaySignFlipRate = useCallback((rate: number | null) => {
    replaySignFlipRateRef.current = rate;
  }, []);
  const gateEvaluationCountRef = useRef(0);

  const applyRotationReport = useCallback((report: RotationReport) => {
    rotationReportRef.current = report;
    const suspended = getSuspendedStrategyIds(report);
    for (const id of rotationOverrideRef.current) {
      suspended.delete(id);
    }
    suspendedStrategiesRef.current = suspended;
  }, []);

  const updateSuspendedStrategies = useCallback(
    (report: RotationReport) => {
      applyRotationReport(report);
    },
    [applyRotationReport],
  );

  const restoreRotationStrategy = useCallback(
    (strategyId: number) => {
      rotationOverrideRef.current.add(strategyId);
      if (rotationReportRef.current) {
        applyRotationReport(rotationReportRef.current);
      } else {
        suspendedStrategiesRef.current.delete(strategyId);
      }
      console.info(`[Rotation] Override restore for strategy ${strategyId}`);
    },
    [applyRotationReport],
  );
  const deskProfileAdjustedHoldCountRef = useRef(0);
  const deskSkippedMinExpectedMoveRef = useRef(0);
  const deskSkippedSameDirCapRef = useRef(0);
  const deskSkippedByRegimeRef = useRef<Record<RegimeTag, number>>({ chop: 0, trendLow: 0, trendHigh: 0 });
  const deskVolSizedNotionalAppliedCountRef = useRef(0);
  const deskSkippedLowPriorityEntryRef = useRef(0);
  const deskSkippedSpreadRef = useRef(0);
  const deskSkippedCategoryCapRef = useRef(0);
  const deskSkippedOutsideSessionRef = useRef(0);
  const deskSkippedDailyStratCapRef = useRef(0);
  const deskSkippedTemplateFamilyRef = useRef(0);
  const deskSkippedSideCapRef = useRef(0);
  const deskSkippedTemplateCapRef = useRef(0);
  const deskSkippedOutsideProvenSessionRef = useRef(0);
  const deskAllocationScaledCountRef = useRef(0);
  const deskAdaptiveTpAppliedCountRef = useRef(0);
  const deskBurstBlockedRef = useRef(0);
  const deskFamilyCapBlockedRef = useRef(0);
  const deskOppositeSideBlockedRef = useRef(0);
  const deskSkippedCorrelatedCapRef = useRef(0);
  /** Dev LS persist: sliding 24h primary-symbol regime events (see `deskRegimeHistogramDevPersistEnabled`). */
  const deskRegime24hEventsRef = useRef<DeskRegimePersistEvent[]>([]);
  const deskRegimeLsLastSaveRef = useRef(0);
  const deskLastRegimeTagRef = useRef<RegimeTag>("chop");
  /** Dev analysis: rolling `classifyRegimeTag` samples on primary symbol (cap from env). */
  const deskRegimePollHistoryRef = useRef<RegimeTag[]>([]);
  const activeStratDefsRef = useRef(activeStratDefs);
  const profileCooldownMulRef = useRef(1);
  const profileHoldTimeMulRef = useRef(1);
  const dayStartBalanceRef = useRef(dayStartBalance);
  const dayStartDateRef = useRef(dayStartDate);

  // Sync refs
  useEffect(() => { positionsRef.current = positions; }, [positions]);
  useEffect(() => { tradesRef.current = trades; }, [trades]);
  useEffect(() => { balanceRef.current = balance; }, [balance]);
  useEffect(() => { disabledRef.current = disabledStrategies; }, [disabledStrategies]);
  useEffect(() => {
    autoDisabledRef.current = new Set(autoDisabledStratIds);
  }, [autoDisabledStratIds]);
  useEffect(() => { pauseRef.current = pauseEntries; }, [pauseEntries]);
  useEffect(() => { lastTradeAtRef.current = lastTradeAt; }, [lastTradeAt]);
  useEffect(() => { dayStartBalanceRef.current = dayStartBalance; }, [dayStartBalance]);
  useEffect(() => {
    dayStartDateRef.current = dayStartDate;
  }, [dayStartDate]);
  useEffect(() => {
    if (dayStartBalance > 0) dailyStartEquityRef.current = dayStartBalance;
  }, [dayStartBalance]);

  useEffect(() => {
    profileCooldownMulRef.current = profileCfg.cooldownMul;
    profileHoldTimeMulRef.current = profileCfg.holdTimeMul;
  }, [profileCfg]);

  useEffect(() => {
    activeStratDefsRef.current = activeStratDefs;
  }, [activeStratDefs]);

  // Regime histogram dev persistence removed (no localStorage).

  const buildHoldDumpPayload = useCallback(() => {
    const defs = activeStratDefsRef.current;
    const deskW = new Map(defs.map((s) => [s.id, s.deskTpWidened === true]));
    const cat = new Map(defs.map((s) => [s.id, s.category]));
    const rows = tradesRef.current.map((t) => ({
      strategyId: t.strategyId,
      strategyName: t.strategyName,
      category: cat.get(t.strategyId) ?? "?",
      exitReason: t.exitReason,
      netPnl: t.netPnl,
    }));
    const polls = deskRegimePollHistoryRef.current;
    let primaryRegimePollWatch: PrimaryRegimePollWatch | undefined;
    if (polls.length > 0) {
      const counts = histogramRegimePolls(polls);
      primaryRegimePollWatch = {
        symbol: PRIMARY_QUOTE_SYMBOL,
        pollWindowMax: deskRegimeWatchPollWindowFromEnv(),
        sampleCount: polls.length,
        counts,
        share: regimeHistogramShares(counts),
      };
    }
    const ev24 = deskRegime24hEventsRef.current;
    let primaryRegimeHistogram24h: PrimaryRegimeHistogram24h | undefined;
    if (ev24.length > 0) {
      const tags = ev24.map((x) => x.tag);
      const counts24 = histogramRegimePolls(tags);
      const tMin = ev24.reduce((acc, x) => Math.min(acc, x.t), ev24[0]!.t);
      const nowMs = Date.now();
      primaryRegimeHistogram24h = {
        symbol: PRIMARY_QUOTE_SYMBOL,
        windowMs: DESK_REGIME_HISTOGRAM_LS_WINDOW_MS,
        sampleCount: ev24.length,
        oldestEventAgeMs: Number.isFinite(tMin) ? Math.max(0, nowMs - tMin) : null,
        counts: counts24,
        share: regimeHistogramShares(counts24),
      };
    }
    return buildDeskHoldTuningDumpPayload(
      rows,
      deskW,
      cat,
      400,
      primaryRegimePollWatch,
      primaryRegimeHistogram24h,
    );
  }, []);

  useEffect(() => {
    if (typeof window === "undefined" || !deskHoldTuningAnalysisModeEnabled()) return;
    const w = window as Window & { __deskHoldTuningDump?: () => string };
    w.__deskHoldTuningDump = () => JSON.stringify(buildHoldDumpPayload(), null, 2);
    return () => {
      delete w.__deskHoldTuningDump;
    };
  }, [buildHoldDumpPayload]);

  useEffect(() => {
    if (typeof window === "undefined" || !deskHoldTuningAnalysisModeEnabled()) return;
    const intervalMs = deskHoldTuningExportIntervalMsFromEnv();
    if (intervalMs <= 0) return;

    const logPayload = () => {
      console.info("[desk-hold-tuning]", JSON.stringify(buildHoldDumpPayload()));
    };

    const id = window.setInterval(logPayload, intervalMs);
    return () => window.clearInterval(id);
  }, [buildHoldDumpPayload]);

  useEffect(() => {
    if (typeof window === "undefined" || !deskHoldTuningAnalysisModeEnabled()) return;
    const intervalMs = deskRegimeWatchIntervalMsFromEnv();
    if (intervalMs <= 0) return;

    const logRegime = () => {
      const polls = deskRegimePollHistoryRef.current;
      if (polls.length === 0) return;
      const counts = histogramRegimePolls(polls);
      console.info(
        "[desk-regime-watch]",
        JSON.stringify({
          symbol: PRIMARY_QUOTE_SYMBOL,
          sampleCount: polls.length,
          pollWindowMax: deskRegimeWatchPollWindowFromEnv(),
          counts,
          share: regimeHistogramShares(counts),
        }),
      );
    };

    const id = window.setInterval(logRegime, intervalMs);
    return () => window.clearInterval(id);
  }, []);

  // ========== MONGODB STATE (no localStorage) ==========

  const [clearedAt, setClearedAt] = useState(0);

  /** POST current engine state to MongoDB paper_state collection. */
  const saveToMongo = useCallback((overrides?: { clearedAt?: number; balance?: number; pauseEntries?: boolean; disabledStrategies?: number[] }) => {
    if (!cloudAccountKey) return;
    const body = {
      accountKey: cloudAccountKey,
      balance: overrides?.balance ?? balance,
      positions,
      pauseEntries: overrides?.pauseEntries ?? pauseEntries,
      disabledStrategies: overrides?.disabledStrategies ?? disabledStrategies,
      lastTradeAt,
      dayStartBalance,
      dayStartDate,
      clearedAt: overrides?.clearedAt ?? clearedAt,
    };
    void fetch("/api/paper-state", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify(body),
    }).catch(() => { /* non-fatal */ });
  }, [cloudAccountKey, balance, positions, pauseEntries, disabledStrategies, lastTradeAt, dayStartBalance, dayStartDate, clearedAt]);

  // Initial load: migrate any existing localStorage data to MongoDB, then fetch from MongoDB
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      // ── One-time migration of legacy localStorage state ──────────────────────
      if (typeof localStorage !== "undefined") {
        try {
          const legacyRaw = localStorage.getItem(stateStorageKey);
          if (legacyRaw) {
            const legacy = JSON.parse(legacyRaw) as Partial<EngineState>;
            const legacyClearedAt = Number(localStorage.getItem(`${stateStorageKey}_cleared_at`) || "0");
            // POST legacy state to MongoDB so data is not lost
            await fetch("/api/paper-state", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              credentials: "include",
              body: JSON.stringify({
                accountKey: cloudAccountKey,
                balance: legacy.balance ?? 1000,
                positions: legacy.positions ?? [],
                pauseEntries: legacy.pauseEntries ?? false,
                disabledStrategies: legacy.disabledStrategies ?? [],
                lastTradeAt: legacy.lastTradeAt ?? 0,
                dayStartBalance: legacy.dayStartBalance ?? 1000,
                dayStartDate: legacy.dayStartDate ?? 0,
                clearedAt: legacyClearedAt,
              }),
            }).catch(() => {});
            // Clear migrated localStorage keys
            localStorage.removeItem(stateStorageKey);
            localStorage.removeItem(`${stateStorageKey}_cleared_at`);
            localStorage.removeItem("btc_ft_active_collection");
          }
        } catch { /* ignore corrupt legacy data */ }
      }

      if (cancelled) return;

      // ── Fetch account state from MongoDB ─────────────────────────────────────
      let resolvedClearedAt = 0;
      let storedBalance: number | undefined;
      let loadedPositionCount = 0;
      try {
        const res = await fetch(`/api/paper-state?account_key=${encodeURIComponent(cloudAccountKey ?? "")}`, {
          cache: "no-store",
          credentials: "include",
        });
        if (!cancelled && res.ok) {
          const data = (await res.json()) as { ok?: boolean; state?: Partial<{ balance: number; positions: unknown[]; pause_entries: boolean; disabled_strategies: number[]; last_trade_at: number; day_start_balance: number; day_start_date: number; cleared_at: number }> };
          if (data.ok && data.state) {
            const s = data.state;
            resolvedClearedAt = s.cleared_at ?? 0;
            setClearedAt(resolvedClearedAt);
            if (typeof s.balance === "number") storedBalance = s.balance;
            loadedPositionCount = Array.isArray(s.positions) ? s.positions.length : 0;
            if (Array.isArray(s.positions)) {
              const now = Date.now();
              const slMul = btcFtPaperSlPctMulFromEnv();
              const paperModeOnLoad = isBtcFtPaperDeskMode({
                paperEnsureTrades,
                allStrategiesAvailable,
                moduleKey,
              });
              setPositions(
                (s.positions as BTCFuturesPosition[]).map((p) => {
                  const base = {
                    ...p,
                    symbol: p.symbol || PRIMARY_QUOTE_SYMBOL,
                    lastFundingAppliedAt:
                      typeof p.lastFundingAppliedAt === "number" && Number.isFinite(p.lastFundingAppliedAt)
                        ? p.lastFundingAppliedAt
                        : now,
                    peakReturnPct:
                      typeof p.peakReturnPct === "number" && Number.isFinite(p.peakReturnPct) ? p.peakReturnPct : 0,
                  };
                  if (!paperModeOnLoad) return base;
                  const strat = STRAT_DEFS.find((d) => d.id === p.strategyId);
                  const stops = paperDeskWidenPositionStops(base, strat, slMul);
                  const adaptiveSl =
                    base.side === "LONG"
                      ? Math.min(base.adaptiveSl, stops.adaptiveSl)
                      : Math.max(base.adaptiveSl, stops.adaptiveSl);
                  return { ...base, ...stops, adaptiveSl, slPrice: adaptiveSl };
                }),
              );
            }
            if (typeof s.pause_entries === "boolean") setPauseEntries(s.pause_entries);
            if (!allStrategiesAvailable && Array.isArray(s.disabled_strategies)) {
              setDisabledStrategies((s.disabled_strategies as number[]).filter((id) => activeStrategyIdSet.has(id)));
            }
            if (typeof s.last_trade_at === "number") setLastTradeAt(s.last_trade_at);
            if (typeof s.day_start_balance === "number") {
              setDayStartBalance(s.day_start_balance);
              dailyStartEquityRef.current = s.day_start_balance;
            }
            if (typeof s.day_start_date === "number") setDayStartDate(s.day_start_date);
          }
        }
      } catch { /* non-fatal; start with defaults */ }

      if (cancelled) return;

      // ── Fetch trades from MongoDB ─────────────────────────────────────────────
      await flushTradeSyncQueue(cloudAccountKey, { force: true });
      if (cancelled) return;
      const merged = await pullAndMergePaperTrades(cloudAccountKey, []);
      const afterClear = (t: BTCFuturesTrade) =>
        resolvedClearedAt === 0 || new Date(t.closedAt).getTime() > resolvedClearedAt;
      const tradesAfterClear = merged.filter(afterClear);
      if (!cancelled) {
        setTrades(tradesAfterClear);
        // Reconcile stored balance against production trade sum to detect probe corruption.
        const productionClosed = tradesAfterClear.filter(
          (t) => !isProbeOrBootstrapTrade({ strategyName: t.strategyName }),
        );
        const expectedBalance = INITIAL_BALANCE + productionClosed.reduce((s, t) => s + t.netPnl, 0);
        const raw = storedBalance ?? INITIAL_BALANCE;
        const driftThreshold = Math.max(1, 0.01 * INITIAL_BALANCE);
        if (productionClosed.length === 0 && loadedPositionCount === 0 && raw !== INITIAL_BALANCE) {
          // Corruption guard: 0 production trades + 0 positions but balance ≠ initial → force reset.
          console.warn(`[paper-load] Corruption guard: 0 trades + 0 positions, balance=${raw.toFixed(2)} → forcing ${INITIAL_BALANCE}`);
          setBalance(INITIAL_BALANCE);
        } else if (Math.abs(raw - expectedBalance) > driftThreshold) {
          console.warn(`[paper-load] Balance drift detected: stored=${raw.toFixed(2)} expected=${expectedBalance.toFixed(2)} → using expected`);
          setBalance(expectedBalance);
        } else {
          if (storedBalance !== undefined) setBalance(storedBalance);
        }
      }
    })();

    return () => { cancelled = true; };
  }, [
    activeStrategyIdSet,
    allStrategiesAvailable,
    cloudAccountKey,
    stateStorageKey,
    paperEnsureTrades,
    moduleKey,
  ]);

  useEffect(() => {
    if (!allStrategiesAvailable) return;
    setDisabledStrategies([]);
    setAutoDisabledStratIds([]);
  }, [allStrategiesAvailable]);

  // ── Worker heartbeat poll (monitor mode) ─────────────────────────────────
  // When NEXT_PUBLIC_DESK_WORKER_ENABLED=1 the browser reads worker_* fields
  // from the paper-state endpoint on every poll so the UI can show "last tick Xs ago".
  // When the worker is fresh (<45s) we skip local poll WRITES (entries/exits)
  // and hydrate state from Mongo instead. When the worker goes stale the
  // browser transparently takes over (legacy behavior).
  useEffect(() => {
    if (!workerEnabled || !cloudAccountKey) return;
    let cancelled = false;
    const fetchWorkerState = async () => {
      try {
        const res = await fetch(
          `/api/paper-state?account_key=${encodeURIComponent(cloudAccountKey)}`,
          { cache: "no-store" },
        );
        if (cancelled || !res.ok) return;
        const data = (await res.json()) as {
          ok?: boolean;
          state?: {
            worker_last_poll_at?: number | null;
            worker_owner?: "browser" | "vps" | null;
            balance?: number;
            positions?: unknown[];
          };
        };
        if (!data.ok || !data.state) return;
        const s = data.state;
        if (cancelled) return;
        setWorkerLastPollAt(s.worker_last_poll_at ?? null);
        setWorkerOwner(s.worker_owner ?? null);
        // In monitor mode hydrate balance + positions from worker-written state
        const isMonitorMode =
          s.worker_last_poll_at !== null &&
          s.worker_last_poll_at !== undefined &&
          Date.now() - s.worker_last_poll_at < 45_000;
        if (isMonitorMode) {
          if (typeof s.balance === "number") setBalance(s.balance);
          if (Array.isArray(s.positions)) {
            setPositions(s.positions as BTCFuturesPosition[]);
          }
        }
      } catch { /* non-fatal */ }
    };
    void fetchWorkerState();
    const id = setInterval(() => void fetchWorkerState(), 4_000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [workerEnabled, cloudAccountKey]);

  // ── Screen wake lock (optional) ───────────────────────────────────────────
  // Keeps the browser screen on when the desk is live and the user opted in
  // (NEXT_PUBLIC_DESK_WAKE_LOCK=1). Released on unmount / visibility loss.
  useEffect(() => {
    if (process.env.NEXT_PUBLIC_DESK_WAKE_LOCK !== "1") return;
    if (typeof window === "undefined" || !("wakeLock" in navigator)) return;
    let wakeLock: WakeLockSentinel | null = null;
    const acquire = async () => {
      try {
        wakeLock = await (navigator as Navigator & { wakeLock: { request: (t: string) => Promise<WakeLockSentinel> } }).wakeLock.request("screen");
      } catch { /* permission denied or not supported */ }
    };
    const handleVisibility = () => {
      if (document.visibilityState === "visible") void acquire();
    };
    void acquire();
    document.addEventListener("visibilitychange", handleVisibility);
    return () => {
      document.removeEventListener("visibilitychange", handleVisibility);
      wakeLock?.release().catch(() => {});
    };
  }, []);

  // ── Catch-up polls on tab focus ───────────────────────────────────────────
  // When the browser re-gains focus and owns execution (no fresh worker),
  // run 3 rapid catch-up polls so positions aren't stale from tab throttling.
  const catchUpPollsRef = useRef(0);
  useEffect(() => {
    if (typeof window === "undefined") return;
    const onVisibility = () => {
      if (document.visibilityState === "visible" && !workerMonitorMode) {
        catchUpPollsRef.current = 3; // processed by the poll loop below
      }
    };
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Ranked winners gate — fetch strategy rankings when NEXT_PUBLIC_BTC_FT_USE_RANKED=1
  useEffect(() => {
    if (!btcFtUseRankedEnabled()) return;
    let cancelled = false;
    void (async () => {
      try {
        const res = await fetch("/api/strategy-rankings", { cache: "no-store" });
        if (cancelled || !res.ok) return;
        const body = (await res.json()) as { ok?: boolean; rankings?: BtcFtStrategyRankingRow[] };
        if (!body.ok || !Array.isArray(body.rankings)) return;
        const ids = winnerIdsFromRankings(body.rankings);
        if (cancelled) return;
        setRankedWinnerIds(ids);
        setStrategyRankings(body.rankings);
        if (process.env.NODE_ENV === "development") {
          console.info("[btc-futures-paper] ranked winners loaded", {
            winnerCount: ids.size,
            totalRanked: body.rankings.length,
          });
        }
      } catch {
        // Network/parse — keep prior winner set
      }
    })();
    return () => { cancelled = true; };
  }, []); // fetch once on mount; re-run requires page refresh (rankings change on rebuild)

  // Rolling expectancy → auto-disable losers (union with manual list; never auto re-enable)
  useEffect(() => {
    if (disableAutoKill || !deskAutoDisableStratsEnabled()) {
      setAutoDisabledStratIds([]);
      return;
    }

    let cancelled = false;
    const windowDays = deskKillWindowDaysFromEnv();
    const killOpts = {
      minTrades: deskKillMinTradesFromEnv(),
      maxExpectancyUsd: deskKillMaxExpectancyUsdFromEnv(),
      maxSumNetUsd: deskKillMaxSumNetUsdFromEnv(),
    };

    if (!cloudAccountKey) {
      setAutoDisabledStratIds([]);
      return;
    }

    void (async () => {
      try {
        const url = `/api/paper-trades/strategy-stats?window_days=${windowDays}`;
        const res = await fetch(url, { credentials: "include", cache: "no-store" });
        if (cancelled || !res.ok) return;
        const body = (await res.json()) as {
          ok?: boolean;
          stats?: StrategyStatRow[];
        };
        if (!body.ok || !Array.isArray(body.stats)) return;

        const disableSet = buildStratDisableSetFromStats(body.stats, killOpts);
        const ids = [...disableSet]
          .filter((id) => activeStrategyIdSet.has(id))
          .sort((a, b) => a - b);
        if (cancelled) return;
        setAutoDisabledStratIds(ids);

        if (
          process.env.NODE_ENV === "development" &&
          !autoDisableFetchLoggedRef.current
        ) {
          autoDisableFetchLoggedRef.current = true;
          console.info("[btc-futures-paper] auto-disable strats", {
            accountKey: cloudAccountKey,
            windowDays,
            count: ids.length,
            ids: formatAutoDisabledStratIds(ids, 120),
            killOpts,
          });
        }
      } catch {
        // network / parse — keep prior auto-disable set
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [activeStrategyIdSet, cloudAccountKey, disableAutoKill]);

  // Periodic save to MongoDB
  useEffect(() => {
    const id = setInterval(() => saveToMongo(), 30000);
    return () => clearInterval(id);
  }, [saveToMongo]);

  // ========== ACTIONS ==========
  const togglePause = useCallback(() => {
    setPauseEntries(p => !p);
  }, []);

  // Shared balance reset — called by both resetPaperAccount and clearTradeHistory.
  const resetPaperBalances = useCallback(() => {
    setBalance(INITIAL_BALANCE);
    setDayStartBalance(INITIAL_BALANCE);
    dailyStartEquityRef.current = INITIAL_BALANCE;
    peakEquityForDrawdownRef.current = INITIAL_BALANCE;
    drawdownEntryPausedRef.current = false;
    forceProbeOpenedRef.current = false;
  }, [INITIAL_BALANCE]);

  const resetPaperAccount = useCallback(() => {
    resetPaperBalances();
    setPositions([]);
    setTrades([]);
    setPauseEntries(false);
    setLastTradeAt(0);
    setDayStartDate(new Date().getDate());
    stratCooldownsRef.current = {};
    deskProfileAdjustedHoldCountRef.current = 0;
    deskSkippedMinExpectedMoveRef.current = 0;
    deskSkippedSameDirCapRef.current = 0;
    deskSkippedByRegimeRef.current = { chop: 0, trendLow: 0, trendHigh: 0 };
    deskVolSizedNotionalAppliedCountRef.current = 0;
    deskSkippedLowPriorityEntryRef.current = 0;
    deskSkippedSpreadRef.current = 0;
    deskSkippedCategoryCapRef.current = 0;
    deskSkippedOutsideSessionRef.current = 0;
    deskSkippedDailyStratCapRef.current = 0;
    deskSkippedTemplateFamilyRef.current = 0;
    deskSkippedSideCapRef.current = 0;
    deskSkippedTemplateCapRef.current = 0;
    deskSkippedOutsideProvenSessionRef.current = 0;
    deskAllocationScaledCountRef.current = 0;
    deskAdaptiveTpAppliedCountRef.current = 0;
    deskBurstBlockedRef.current = 0;
    deskFamilyCapBlockedRef.current = 0;
    deskOppositeSideBlockedRef.current = 0;
    deskSkippedCorrelatedCapRef.current = 0;
    resetSkipReasonLog();
    clearRuntimeBlocklist();
    suspendedStrategiesRef.current = new Set();
    rotationReportRef.current = null;
    rotationOverrideRef.current = new Set();
    lastSignalQualityRef.current = null;
    lastMtfConfluenceRef.current = null;
    qualitySkipCountRef.current = 0;
    mtfSkipCountRef.current = 0;
    profitModeSkipCountRef.current = 0;
    entryRotationAllowedCountRef.current = 0;
    entryRotationBlockedCountRef.current = 0;
    gateEvaluationCountRef.current = 0;
    deskLastRegimeTagRef.current = "chop";
    deskRegimePollHistoryRef.current = [];
    saveToMongo({ balance: INITIAL_BALANCE, pauseEntries: false, disabledStrategies: [] });
  }, [resetPaperBalances, saveToMongo]);

  const clearTradeHistory = useCallback(() => {
    const clearedAtMs = Date.now();
    setClearedAt(clearedAtMs);
    setTrades([]);
    setPositions([]);
    setLastTradeAt(0);
    stratCooldownsRef.current = {};
    resetPaperBalances();
    saveToMongo({ clearedAt: clearedAtMs, balance: INITIAL_BALANCE });
  }, [resetPaperBalances, saveToMongo, INITIAL_BALANCE]);

  const setDisabledStrategiesHandler = useCallback((ids: number[]) => {
    setDisabledStrategies(ids.filter((id) => activeStrategyIdSet.has(id)));
  }, [activeStrategyIdSet]);

  const addDisabledStrategyIds = useCallback(
    (ids: number[]) => {
      if (!ids.length) return;
      setDisabledStrategies((prev) => {
        const next = mergeDisabledStrategyIds(prev, ids).filter((id) => activeStrategyIdSet.has(id));
        saveToMongo({ disabledStrategies: next });
        return next;
      });
    },
    [activeStrategyIdSet, saveToMongo],
  );

  const exportCSV = useCallback(() => {
    const headers = [
      "ID",
      "Symbol",
      "Strategy",
      "Side",
      "Entry",
      "Exit",
      "Contracts",
      "Realized PnL",
      "Fees",
      "Net PnL",
      "Net PnL % on margin",
      "Price move %",
      "Funding",
      "Opened",
      "Closed",
      "Exit Reason",
    ];
    const rows = trades.map((t) => {
      const pm =
        typeof t.priceMovePct === "number" && Number.isFinite(t.priceMovePct)
          ? t.priceMovePct
          : paperPriceMovePctOnNotional(t.entryPrice, t.exitPrice, t.side);
      return [
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
      pm.toFixed(4),
      t.fundingCosts.toFixed(4),
      t.openedAt,
      t.closedAt,
      t.exitReason,
    ];
    });
    return [headers.join(","), ...rows.map(r => r.join(","))].join("\n");
  }, [trades]);

  // ========== STATS ==========
  const calculateStats = useCallback((): BTCFuturesEngineStats => {
    // Exclude synthetic probe/bootstrap trades from all session metrics so they
    // don't inflate win-rate, expectancy, or fee-ratio numbers.
    const productionTrades = trades.filter(
      (t) => !isProbeOrBootstrapTrade({ strategyName: t.strategyName }),
    );
    const productionPositions = positions.filter(
      (p) => !isProbeOrBootstrapTrade({ strategyName: p.strategyName }),
    );

    const wins = productionTrades.filter(t => t.netPnl > 0);
    const losses = productionTrades.filter(t => t.netPnl <= 0);
    const winCount = wins.length;
    const lossCount = losses.length;
    const totalTrades = productionTrades.length;
    const winRate = totalTrades > 0 ? (winCount / totalTrades) * 100 : 0;
    const avgWin = winCount > 0 ? wins.reduce((s, t) => s + t.netPnl, 0) / winCount : 0;
    const avgLoss = lossCount > 0 ? losses.reduce((s, t) => s + t.netPnl, 0) / lossCount : 0;
    const profitFactor = avgLoss !== 0 ? Math.abs(avgWin / avgLoss) : 0;
    const realizedPnl = productionTrades.reduce((s, t) => s + t.netPnl, 0);
    const unrealizedPnl = productionPositions.reduce((s, p) => s + p.unrealizedPnl, 0);
    const totalFees = productionTrades.reduce((s, t) => s + t.fees, 0);
    const totalFunding = productionTrades.reduce((s, t) => s + t.fundingCosts, 0) + productionPositions.reduce((s, p) => s + p.fundingCosts, 0);
    const netPnl = realizedPnl + unrealizedPnl;

    const peak = Math.max(INITIAL_BALANCE, ...productionTrades.map((_, i) => INITIAL_BALANCE + productionTrades.slice(0, i + 1).reduce((s, t) => s + t.netPnl, 0)));
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

    const sm = computeSessionTradingMetrics(productionTrades);
    const deskPnLScorecard =
      productionTrades.length >= 5
        ? computeDeskRollingPnLScorecard(
            productionTrades.map((t) => ({
              closedAt: t.closedAt,
              netPnl: t.netPnl,
              grossPnl: t.realizedPnl,
              fees: t.fees,
              strategyName: t.strategyName,
            })),
          )
        : null;
    const exitAn = computeSessionExitReasonAnalytics(productionTrades);
    const strategyDiagnostics = productionTrades.length >= 5
      ? computeStrategyDiagnosticsFromEngineTrades(productionTrades)
      : null;

    const rollingHealthCheck = productionTrades.length >= 5
      ? computeRollingHealthCheckFromEngineTrades(productionTrades)
      : null;

    if (rollingHealthCheck) {
      console.info(
        `[HealthCheck] Grade=${rollingHealthCheck.grade}` +
        ` Expectancy=${rollingHealthCheck.expectancy.toFixed(2)}` +
        ` WinRate=${(rollingHealthCheck.winRate * 100).toFixed(1)}%` +
        ` fee/gross=${(rollingHealthCheck.feePctOfAbsGross * 100).toFixed(1)}%` +
        ` TP=${rollingHealthCheck.tpHits}/${rollingHealthCheck.window}`,
      );
    }

    const stratDeskW = new Map(activeStratDefs.map((s) => [s.id, s.deskTpWidened === true]));
    const stratCat = new Map(activeStratDefs.map((s) => [s.id, s.category]));
    const tuningRows = productionTrades.map((t) => ({
      strategyId: t.strategyId,
      strategyName: t.strategyName,
      category: stratCat.get(t.strategyId) ?? "?",
      exitReason: t.exitReason,
      netPnl: t.netPnl,
    }));
    const deskBuckets = reduceTradesToStrategyDeskBuckets(tuningRows, stratDeskW, stratCat, 400);
    const sessionWorstTimeOffenders = rankWorstTimeOffenders(deskBuckets, 5);

    const br = deskSkippedByRegimeRef.current;
    const deskSkippedByRegime = br.chop + br.trendLow + br.trendHigh;
    const deskSkippedByRegimeBreakdown =
      (["chop", "trendLow", "trendHigh"] as const)
        .map((k) => (br[k] > 0 ? `${k}×${br[k]}` : null))
        .filter(Boolean)
        .join(" · ") || "—";

    const attributionRows: PaperTradeDbRow[] = productionTrades.map((t) => ({
      strategy_name: t.strategyName,
      net_pnl: t.netPnl,
      gross_pnl: t.realizedPnl,
      fees: t.fees,
      exit_reason: t.exitReason,
      side: t.side,
      template_family: btcFtTemplateFamilyKey(t.strategyName),
      opened_at: t.openedAt,
      closed_at: t.closedAt,
    })) as PaperTradeDbRow[];

    const attributionReport =
      productionTrades.length >= 10 ? computeAttribution(attributionRows) : null;

    if (attributionReport) {
      const longE = attributionReport.bySide.find((s) => s.label === "LONG")?.avgNetPnl ?? 0;
      const shortE = attributionReport.bySide.find((s) => s.label === "SHORT")?.avgNetPnl ?? 0;
      console.info(
        `[Attribution] bestHold=${attributionReport.bestHoldBucket ?? "N/A"} ` +
          `LONG_E=$${longE.toFixed(2)} SHORT_E=$${shortE.toFixed(2)}`,
      );
    }

    const tuneRecommendation =
      productionTrades.length >= 10
        ? recommendOneTune(
            attributionRows,
            activeSignalThreshold,
            1.5,
            0.5,
            MAX_SAME_SIDE_POSITIONS,
            50,
          )
        : null;

    const deskRecommendation = deriveDeskRecommendation({
      attribution: attributionReport,
      rotation: rotationReportRef.current,
      tune: tuneRecommendation,
      qualitySkipCount: qualitySkipCountRef.current,
      mtfSkipCount: mtfSkipCountRef.current,
      totalEvaluations: gateEvaluationCountRef.current,
    });

    const scorecardAction = deriveScorecardAction(
      deskPnLScorecard,
      tuneRecommendation,
      rotationReportRef.current,
      activeSignalThreshold,
    );

    if (scorecardAction.severity !== "OK") {
      console.info(
        `[ScorecardAction] ${scorecardAction.severity} ${scorecardAction.action} — ${scorecardAction.rationale}`,
      );
    }

    if (deskRecommendation.action !== "NO_CHANGE") {
      console.info(
        `[DeskRec] action=${deskRecommendation.action} confidence=${deskRecommendation.confidence} ` +
          `rationale=${deskRecommendation.rationale}`,
      );
    }

    const readinessForGates = runProductionReadiness({
      signalThreshold: activeSignalThreshold,
      leverage: 25,
      takerFeePct: 0.001,
      maxSameSide: MAX_SAME_SIDE_POSITIONS,
      minPositionNotional: 100,
      openPositionCount: positions.length,
      currentRegime: deskLastRegimeTagRef.current,
      markPriceAgeMs: markAgeMs(),
      runtimeBlocklist: getRuntimeBlocklist(),
      timeExitCount: productionTrades.filter((t) => t.exitReason === "TIME").length,
      health: rollingHealthCheck,
      closedTradeCount: productionTrades.length,
      nodeEnv: process.env.NODE_ENV ?? "",
      mongoConnected: Boolean(cloudAccountKey?.trim()),
      accountKeySet: Boolean(cloudAccountKey?.trim()),
    });

    const soakStorageKey = `${storageNamespace}_soak_history`;
    let soakHistory = soakHistoryRef.current;
    if (deskPnLScorecard) {
      soakHistory = appendSoakSnapshot(
        loadSoakHistory(soakStorageKey),
        deskPnLScorecard,
        profitModeSkipCountRef.current,
      );
      saveSoakHistory(soakStorageKey, soakHistory);
      soakHistoryRef.current = soakHistory;
      const lastSnap = soakHistory[soakHistory.length - 1];
      if (lastSnap && lastSoakLogDateRef.current !== lastSnap.dateUtc) {
        console.info(
          `[Soak] day=${lastSnap.dateUtc} grade=${lastSnap.grade} closes=${lastSnap.closes} ` +
            `E=$${lastSnap.expectancy.toFixed(2)}`,
        );
        lastSoakLogDateRef.current = lastSnap.dateUtc;
      }
    } else if (!soakHistory.length && typeof window !== "undefined") {
      soakHistory = loadSoakHistory(soakStorageKey);
      soakHistoryRef.current = soakHistory;
    }
    const soakSummary = soakTrendSummary(soakHistory);

    const goLiveGates =
      productionTrades.length >= 10
        ? computeGoLiveGates({
            trades: attributionRows,
            health: rollingHealthCheck,
            readiness: readinessForGates,
            replaySignFlipRate: replaySignFlipRateRef.current,
          })
        : null;

    if (goLiveGates) {
      console.info(
        `[GoLive] ${goLiveGates.recommendation} score=${(goLiveGates.score * 100).toFixed(0)}% ` +
          `blockers=${goLiveGates.blockers.filter((g) => !g.pass).length}`,
      );
    }

    const unified = computeUnifiedReadiness({
      scorecard: deskPnLScorecard,
      goLive: goLiveGates,
      soak: soakSummary,
      replaySignFlipRate: replaySignFlipRateRef.current,
      profitModeEnabled: profitModeCfg.enabled,
    });

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
      deskRegimeAnnotatedStratCount: deskPolicySnapshot.regimeAnnotatedCount,
      deskProfileAdjustedHoldAppliedCount: deskProfileAdjustedHoldCountRef.current,
      sessionExitReasonSummary: formatExitReasonSessionSummary(exitAn.rows),
      sessionWorstTimeOffenders,
      deskLastRegimeTag: deskLastRegimeTagRef.current,
      deskSkippedMinExpectedMove: deskSkippedMinExpectedMoveRef.current,
      deskSkippedSameDirCap: deskSkippedSameDirCapRef.current,
      deskSkippedByRegime,
      deskSkippedByRegimeBreakdown,
      deskAutoDisableEnabled: !disableAutoKill && deskAutoDisableStratsEnabled(),
      deskAutoDisabledStratCount: autoDisabledStratIds.length,
      deskAutoDisabledStratIds: formatAutoDisabledStratIds(autoDisabledStratIds),
      deskKillWindowDays: deskKillWindowDaysFromEnv(),
      deskVolSizedNotionalEnabled: deskVolSizedNotionalEnabledFromEnv(),
      deskVolSizedNotionalAppliedCount: deskVolSizedNotionalAppliedCountRef.current,
      deskSkippedLowPriorityEntry: deskSkippedLowPriorityEntryRef.current,
      deskEntryReplaceWeakestEnabled: deskEntryReplaceWeakestFromEnv(),
      deskSkippedSpread: deskSkippedSpreadRef.current,
      deskMaxLastMarkSpreadPct: deskMaxLastMarkSpreadPctFromEnv(),
      deskSkippedCategoryCap: deskSkippedCategoryCapRef.current,
      deskMaxOpenPerCategory: deskMaxOpenPerCategoryFromEnv(),
      deskSkippedOutsideSession: deskSkippedOutsideSessionRef.current,
      deskEntryUtcSessionLabel: formatDeskEntryUtcSessionLabel(entryUtcSessionOverride ?? deskEntryUtcSessionFromEnv()),
      deskSkippedDailyStratCap: deskSkippedDailyStratCapRef.current,
      deskSkippedTemplateFamily: deskSkippedTemplateFamilyRef.current,
      deskSkippedSideCap: deskSkippedSideCapRef.current,
      deskSkippedTemplateCap: deskSkippedTemplateCapRef.current,
      deskSkippedOutsideProvenSession: deskSkippedOutsideProvenSessionRef.current,
      deskAllocationScaledCount: deskAllocationScaledCountRef.current,
      deskAdaptiveTpAppliedCount: deskAdaptiveTpAppliedCountRef.current,
      deskBurstBlockedCount: deskBurstBlockedRef.current,
      deskFamilyCapBlockedCount: deskFamilyCapBlockedRef.current,
      deskOppositeSideBlockedCount: deskOppositeSideBlockedRef.current,
      deskSkippedCorrelatedCap: deskSkippedCorrelatedCapRef.current,
      skipReasonSummary: getSkipReasonSummary(),
      strategyDiagnostics,
      rollingHealthCheck,
      runtimeBlocklistIds: getRuntimeBlocklist(),
      rotationReport: rotationReportRef.current,
      suspendedStrategyCount: suspendedStrategiesRef.current.size,
      attributionReport,
      lastSignalQuality: lastSignalQualityRef.current,
      lastMtfConfluence: lastMtfConfluenceRef.current,
      qualitySkipCount: qualitySkipCountRef.current,
      mtfSkipCount: mtfSkipCountRef.current,
      gateEvaluationCount: gateEvaluationCountRef.current,
      deskRecommendation,
      goLiveGates,
      profitModeEnabled: profitModeCfg.enabled,
      profitModeSkipCount: profitModeSkipCountRef.current,
      profitModeExitActive: profitModeCfg.enabled && Boolean(profitModeExitConfig(profitModeCfg)),
      entryRotationAllowedCount: entryRotationAllowedCountRef.current,
      entryRotationBlockedCount: entryRotationBlockedCountRef.current,
      balanceDriftUsd: Math.abs(balance - INITIAL_BALANCE - realizedPnl),
      workerLastPollAt,
      workerOwner,
      workerMonitorMode,
      deskPnLScorecard,
      scorecardAction,
      soakHistory,
      soakSummary,
      unifiedReadiness: unified.state,
      unifiedReadinessBlockers: unified.blockers,
      unifiedReadinessNextStep: unified.nextStep,
      replaySignFlipRate: replaySignFlipRateRef.current,
    };
  }, [
    trades,
    positions,
    balance,
    strategyProfile,
    activeSignalThreshold,
    deskPolicySnapshot,
    activeStratDefs,
    autoDisabledStratIds,
    disableAutoKill,
    entryUtcSessionOverride,
    storageNamespace,
    profitModeCfg.enabled,
    cloudAccountKey,
  ]);

  const exportJSON = useCallback(() => {
    return JSON.stringify({ balance, positions, trades, stats: calculateStats() }, null, 2);
  }, [balance, positions, trades, calculateStats]);

  // ========== STRATEGY STATUSES ==========
  const strategyStatuses = useMemo((): BTCFuturesStrategyStatus[] => {
    const now = Date.now();
    const buildStatus = (strat: Pick<FuturesStratDef, "id" | "name" | "category">): BTCFuturesStrategyStatus => {
      const openCount = positions.filter((p) => p.strategyId === strat.id).length;
      const stratTrades = trades.filter((t) => t.strategyId === strat.id);
      const lastTrade = stratTrades[stratTrades.length - 1];
      const inCooldown = Object.entries(stratCooldownsRef.current).some(
        ([key, until]) => key.endsWith(`:${strat.id}`) && (until ?? 0) > now,
      );
      const wins = stratTrades.filter((t) => t.netPnl > 0).length;
      const losses = stratTrades.filter((t) => t.netPnl <= 0).length;
      const totalPnl = stratTrades.reduce((s, t) => s + t.netPnl, 0);
      const winRate = stratTrades.length > 0 ? (wins / stratTrades.length) * 100 : 0;
      const score =
        stratTrades.length > 0
          ? Math.min(100, winRate * 0.7 + Math.min(30, Math.abs(totalPnl) / 100) * 0.3)
          : 0;

      return {
        id: strat.id,
        name: strat.name,
        category: strat.category,
        status: openCount > 0 ? "OPEN" : inCooldown ? "COOLING" : "AVAILABLE",
        disabled: allStrategiesAvailable
          ? isStrategyRuntimeBlocked(strat.id)
          : disabledStrategies.includes(strat.id) ||
            autoDisabledStratIds.includes(strat.id) ||
            isStrategyRuntimeBlocked(strat.id),
        openCount,
        lastTradeAt: lastTrade ? new Date(lastTrade.closedAt).getTime() : null,
        score,
        totalTrades: stratTrades.length,
        wins,
        losses,
        totalPnl,
        winRate,
      };
    };

    const rows = activeStratDefs.map(buildStatus);
    const seen = new Set(rows.map((r) => r.id));
    const rosterIds = strategyIds && strategyIds.length > 0 ? strategyIds : STRAT_DEFS.map((d) => d.id);
    const extraIds = new Set<number>([
      ...deskStrategiesResult.lowRrSkippedStratIds,
      ...rosterIds.filter((id) => !seen.has(id)),
    ]);
    for (const id of extraIds) {
      if (seen.has(id)) continue;
      const def = STRAT_DEFS.find((d) => d.id === id);
      if (!def) continue;
      rows.push(buildStatus(def));
      seen.add(id);
    }
    return rows;
  }, [
    positions,
    trades,
    disabledStrategies,
    autoDisabledStratIds,
    activeStratDefs,
    allStrategiesAvailable,
    strategyIds,
    deskStrategiesResult.lowRrSkippedStratIds,
  ]);

  // ========== POSITION MANAGEMENT ==========
  type PaperOpenGateCtx = {
    atr14: number;
    regime: RegimeTag;
    entryBook: ReadonlyArray<{ side: Side; notional: number }>;
    signalContributions?: Array<{ reason: string; pts: number }>;
    equityUsd: number;
    entryPriorityScore: number;
  };

  const openPosition = useCallback(
    (strat: StratDef, side: Side, price: number, markPrice: number, symbol: string, gate: PaperOpenGateCtx): BTCFuturesPosition | null => {
      if (
        !relaxEntryGatesRef.current &&
        strat.regimes &&
        strat.regimes.length > 0 &&
        !strat.regimes.includes(gate.regime)
      ) {
        deskSkippedByRegimeRef.current[gate.regime] += 1;
        if (entryDebugLiveRef.current) entryDebugLiveRef.current.failOpenRegime += 1;
        return null;
      }

      // ---- #5 correlation-aware caps: per-side + per-template-family ----
      // Firehose mode loosens these caps so concurrent open positions hit ~MAX_OPEN_POSITIONS
      // for fastest verdict discovery. Defaults stay tight in production.
      const firehose = deskFirehoseModeEnabled();
      const openPositions = positionsRef.current;
      const maxPerSide = firehose ? Math.max(30, deskMaxOpenPerSideFromEnv()) : deskMaxOpenPerSideFromEnv();
      const sameSideCount = openPositions.filter((p) => p.side === side).length;
      if (sameSideCount >= maxPerSide) {
        deskSkippedSideCapRef.current += 1;
        return null;
      }
      const maxPerTemplate = firehose ? Math.max(10, deskMaxOpenPerTemplateFromEnv()) : deskMaxOpenPerTemplateFromEnv();
      const myFamily = btcFtTemplateFamilyKey(strat.name);
      const sameFamilyCount = openPositions.filter(
        (p) => btcFtTemplateFamilyKey(p.strategyName ?? "") === myFamily,
      ).length;
      if (sameFamilyCount >= maxPerTemplate) {
        deskSkippedTemplateCapRef.current += 1;
        return null;
      }

      // ---- #4 session gate: only if explicitly enabled AND enough sample exists ----
      if (profitModeSessionGateEnabled(profitModeCfg)) {
        const sessionStats = computeStrategyHourlyStats(
          strat.id,
          tradesRef.current.map((t) => ({
            strategyId: t.strategyId,
            netPnl: t.netPnl,
            closedAtMs: new Date(t.closedAt).getTime(),
          })),
        );
        const utcHour = new Date().getUTCHours();
        if (!isStrategyInProvenSession(sessionStats, utcHour)) {
          deskSkippedOutsideProvenSessionRef.current += 1;
          return null;
        }
      }

      const id = `${symbol}-${strat.id}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
      const bal = balanceRef.current;
      const slipBps = slippageBpsOverride ?? deskSlippageBpsFromEnv();
      const entryPrice = paperApplyEntrySlippage(side, price, slipBps);
      const isBootstrapProbeStrat =
        strat.name === "PAPER_BOOTSTRAP_PROBE" || strat.name === "DEV_FORCE_PROBE_OPEN";
      const paperSlMul = paperDeskMode ? btcFtPaperSlPctMulFromEnv() : 1;
      const paperStops =
        paperDeskMode && !isBootstrapProbeStrat
          ? paperDeskEffectiveStops(strat, paperSlMul)
          : {
              slPct: isBootstrapProbeStrat ? Math.max(strat.slPct, 1.0) : strat.slPct * paperSlMul,
              tpPct: isBootstrapProbeStrat ? 0.5 : strat.tpPct,
            };
      const slPrice =
        side === "LONG"
          ? entryPrice * (1 - paperStops.slPct / 100)
          : entryPrice * (1 + paperStops.slPct / 100);

      // ---- #3 adaptive TP: scale strat.tpPct by current ATR regime ----
      const baseTpPct = paperStops.tpPct;
      const effectiveTpPct = deskAdaptiveTpEnabled()
        ? computeAdaptiveTpPct(baseTpPct, atrPctFromAtr(gate.atr14, markPrice))
        : baseTpPct;
      if (Math.abs(effectiveTpPct - baseTpPct) / baseTpPct > 0.05) {
        deskAdaptiveTpAppliedCountRef.current += 1;
      }
      const tpPrice =
        side === "LONG" ? entryPrice * (1 + effectiveTpPct / 100) : entryPrice * (1 - effectiveTpPct / 100);

      const equityForSizing = gate.equityUsd;
      const fixedPct = deskFixedNotionalPctOfEquity();
      const volSized = deskVolSizedNotionalEnabledFromEnv();
      const maxNotionalEff = maxPositionNotionalEffective();
      let baseTargetNotional: number;
      if (fixedPct > 0) {
        // Flat sizing — every trade is `fixedPct%` of equity regardless of vol.
        // 1.0 → 1% of equity ($10K on $1M paper balance). Clamped to MIN/MAX.
        const flat = (equityForSizing * fixedPct) / 100;
        baseTargetNotional = Math.min(maxNotionalEff, Math.max(MIN_POSITION_NOTIONAL, flat));
      } else if (volSized) {
        baseTargetNotional = paperNotionalForTargetRisk({
          equityUsd: equityForSizing,
          atr14: gate.atr14,
          markPrice,
          entryPrice,
          side,
          slPct: strat.slPct,
          takerFeePct: TAKER_FEE_PCT,
          riskPctOfEquity: deskRiskPctOfEquityFromEnv(),
          minNotional: MIN_POSITION_NOTIONAL,
          maxNotional: maxNotionalEff,
        });
      } else {
        baseTargetNotional = Math.min(Math.max(bal * 0.1, MIN_POSITION_NOTIONAL), maxNotionalEff);
      }

      // ---- #1+#2 allocation by edge: Kelly + Sharpe-weighted multiplier ----
      let allocationMultiplier = 1.0;
      if (profitModeAllocationByEdgeEnabled(profitModeCfg)) {
        const allTradesNow = tradesRef.current;
        const stratTrades = allTradesNow
          .filter((t) => t.strategyId === strat.id)
          .map((t) => ({
            netPnl: t.netPnl,
            notional: t.notional,
            holdMinutes: Math.max(
              0,
              (new Date(t.closedAt).getTime() - new Date(t.openedAt).getTime()) / 60_000,
            ),
          }));
        const stratStats = computeStrategyEdgeStats(stratTrades);
        // Cohort: each active strategy's Sharpe (computed lazily per open — cost is bounded
        // by roster size × trades-per-strat; replay tests stay snappy at 100 bars).
        const cohortSharpes = activeStratDefs.map((s) => {
          const tt = allTradesNow
            .filter((t) => t.strategyId === s.id)
            .map((t) => ({
            netPnl: t.netPnl,
            notional: t.notional,
            holdMinutes: Math.max(
              0,
              (new Date(t.closedAt).getTime() - new Date(t.openedAt).getTime()) / 60_000,
            ),
          }));
          return computeStrategyEdgeStats(tt).sharpe;
        });
        allocationMultiplier = computeAllocationMultiplier({ stats: stratStats, cohortSharpes });
        if (Math.abs(allocationMultiplier - 1) > 0.05) {
          deskAllocationScaledCountRef.current += 1;
        }
      }
      // Premium tier: 2× notional only once the strategy ID is in the promoted set.
      // Before promotion, premium strategies trade at 1× — same as core — so they
      // don't bleed double dollars while their edge is still unverified.
      const premiumMul =
        isPremiumStrategy(strat) && promotedIdsSet.has(strat.id)
          ? PREMIUM_NOTIONAL_MULTIPLIER
          : 1.0;
      const targetNotional = Math.min(
        maxNotionalEff,
        Math.max(MIN_POSITION_NOTIONAL, baseTargetNotional * allocationMultiplier * premiumMul),
      );

      const contracts = Math.max(
        MIN_CONTRACTS,
        Math.min(MAX_CONTRACTS, calculateContracts(targetNotional, price)),
      );
      const actualNotional = calculateNotional(contracts);

      const baseMoveK = effectiveMinExpectedMoveSafetyK(
        deskMinExpectedMoveSafetyKFromEnv(),
        profileCfg,
      );
      const regimeMoveK = profitModeMinExpectedMoveK(
        gate.regime,
        profitModeCfg,
        baseMoveK,
      );
      const relaxMul =
        profitModeCfg.enabled ? 1 : relaxEntryGatesRef.current ? 0.45 : 1;
      const moveGate = paperMinExpectedMoveVsFees(
        markPrice,
        gate.atr14,
        actualNotional,
        TAKER_FEE_PCT,
        regimeMoveK * minMoveKMultiplier * relaxMul,
      );
      if (!moveGate.ok && !isBootstrapProbeStrat) {
        deskSkippedMinExpectedMoveRef.current += 1;
        if (entryDebugLiveRef.current) entryDebugLiveRef.current.failMinMove += 1;
        return null;
      }

      if (
        paperSameDirNotionalWouldExceedCap(
          gate.entryBook,
          side,
          actualNotional,
          gate.equityUsd,
          deskMaxSameDirNotionalFracFromEnv(),
        )
      ) {
        deskSkippedSameDirCapRef.current += 1;
        if (entryDebugLiveRef.current) entryDebugLiveRef.current.failSameDirCap += 1;
        return null;
      }

      const marginUsed = calculateMarginRequired(actualNotional, LEVERAGE);
      if (bal < marginUsed) {
        if (entryDebugLiveRef.current) entryDebugLiveRef.current.failMargin += 1;
        return null;
      }

      const riskCap = bal * (MAX_LOSS_PER_TRADE_PCT / 100);
      if (paperEstimatedMaxLossAtStopSl(entryPrice, slPrice, actualNotional, side, TAKER_FEE_PCT) > riskCap) {
        if (entryDebugLiveRef.current) entryDebugLiveRef.current.failMaxLoss += 1;
        return null;
      }

      if (volSized) deskVolSizedNotionalAppliedCountRef.current += 1;
      /** Keep ref in sync so multiple opens in one poll tick do not all read stale balance. */
      balanceRef.current = bal - marginUsed;
      const liquidationPrice = paperLiquidationPrice(entryPrice, side, LEVERAGE);

      const holdRes = deskEffectiveHoldMinutesAtOpen(strat.holdMinutes, strategyProfile, strat.deskTpWidened);
      if (holdRes.profileAdjusted) deskProfileAdjustedHoldCountRef.current += 1;

      const position: BTCFuturesPosition = {
        id,
        symbol,
        strategyId: strat.id,
        strategyName: strat.name,
        side,
        entryPrice,
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
        entryPriorityScore: gate.entryPriorityScore,
        signalContributions: gate.signalContributions,
      };

      setPositions(prev => [...prev, position]);
      setBalance(balanceRef.current);
      stratCooldownsRef.current[`${symbol}:${strat.id}`] =
        Date.now() +
        Math.max(
          paperDeskMode ? 15_000 : 30_000,
          Math.round(strat.cooldownMin * 60_000 * profileCooldownMulRef.current * cooldownMultiplier),
        );
      setLastTradeAt(Date.now());
      if (isDeskShadowLogOpenEnabled()) {
        persistShadowTradeIntent(cloudAccountKey, shadowIntentFromPaperOpen(position));
      }
      return position;
    },
    [
      strategyProfile,
      cloudAccountKey,
      cooldownMultiplier,
      minMoveKMultiplier,
      slippageBpsOverride,
      promotedIdsSet,
      paperDeskMode,
      profitModeCfg,
    ],
  );

  const closePosition = useCallback((position: BTCFuturesPosition, exitPrice: number, exitReason: BTCFuturesPosition["exitReason"]) => {
    // SL/TP/TIME/MARK exits pass level/mark first; apply adverse exit slippage before booking.
    const slippedExit = paperApplyExitSlippage(position.side, exitPrice, slippageBpsOverride ?? deskSlippageBpsFromEnv());
    const { grossPnl, fees, netPnl } = paperNetPnlOnClose({
      entryPrice: position.entryPrice,
      exitPrice: slippedExit,
      notional: position.notional,
      side: position.side,
      takerFeePct: TAKER_FEE_PCT,
      fundingCosts: position.fundingCosts,
      // Research mode → 0 (raw expectancy, no $2 floor inflating fake outliers).
      // Production default = $2 (MIN_ABS_NET_PNL_USD). Env override via
      // NEXT_PUBLIC_DESK_MIN_ABS_NET_WIN_USD.
      minAbsNetWinUsd: paperDeskMode ? 0 : deskMinAbsNetWinUsd(),
    });

    if (process.env.NODE_ENV === "development" && exitReason === "TP" && grossPnl > 0 && netPnl < 0) {
      console.warn("[btc-futures-paper] TP booked with positive gross but negative net — check fees/funding", {
        id: position.id,
        grossPnl,
        fees,
        fundingCosts: position.fundingCosts,
        netPnl,
        entryPrice: position.entryPrice,
        exitPrice: slippedExit,
        rawExitPrice: exitPrice,
      });
    }

    const netPnlPct = position.marginUsed > 0 ? (netPnl / position.marginUsed) * 100 : 0;
    const priceMovePct = paperPriceMovePctOnNotional(position.entryPrice, slippedExit, position.side);
    const liqDist = paperLiquidationDistancePct(slippedExit, position.liquidationPrice, position.side);
    const closedAtIso = new Date().toISOString();
    const openedMs = new Date(position.openedAt).getTime();
    const closedMs = new Date(closedAtIso).getTime();
    const fundingSinceOpenMs =
      Number.isFinite(openedMs) && Number.isFinite(closedMs) ? Math.max(0, closedMs - openedMs) : undefined;

    const clientTradeId = crypto.randomUUID();
    const trade: BTCFuturesTrade = {
      clientTradeId,
      id: position.id,
      symbol: position.symbol,
      strategyId: position.strategyId,
      strategyName: position.strategyName,
      side: position.side,
      entryPrice: position.entryPrice,
      exitPrice: slippedExit,
      contracts: position.contracts,
      notional: position.notional,
      marginUsed: position.marginUsed,
      realizedPnl: grossPnl,
      fees,
      netPnl,
      netPnlPct,
      priceMovePct,
      fundingCosts: position.fundingCosts,
      lastFundingAppliedAt: position.lastFundingAppliedAt,
      fundingSinceOpenMs,
      openedAt: position.openedAt,
      closedAt: closedAtIso,
      exitReason: exitReason!,
      liquidationPrice: position.liquidationPrice,
      liquidationDistancePct: liqDist,
      signalContributions: position.signalContributions,
    };

    // Safety cap: probe PnL beyond 10× initial balance is a corruption indicator — clamp to 0.
    let safeNetPnl = netPnl;
    if (isProbeOrBootstrapTrade({ strategyName: position.strategyName }) && Math.abs(netPnl) > 10 * INITIAL_BALANCE) {
      console.error(`[paper-close] Probe safety cap triggered: |netPnl|=${Math.abs(netPnl).toFixed(2)} > ${(10 * INITIAL_BALANCE).toFixed(0)}, clamping to 0`);
      safeNetPnl = 0;
    }
    setTrades(prev => [...prev.slice(-MAX_TRADES + 1), trade]);
    setPositions(prev => prev.filter(p => p.id !== position.id));
    setBalance(prev => prev + position.marginUsed + safeNetPnl);
    if (process.env.NODE_ENV === "development") {
      console.log("[paper-close]", {
        clientTradeId,
        accountKey: cloudAccountKey,
        moduleKey,
        netPnl,
        exitReason,
      });
    }
    persistTradeToServer(trade, cloudAccountKey, moduleKey);
    persistShadowTradeIntent(cloudAccountKey, shadowIntentFromPaperClose(trade));
  }, [cloudAccountKey, slippageBpsOverride, paperDeskMode, moduleKey]);

  const openPositionRef = useRef(openPosition);
  const closePositionRef = useRef(closePosition);
  useEffect(() => {
    openPositionRef.current = openPosition;
    closePositionRef.current = closePosition;
  }, [openPosition, closePosition]);

  // ========== DATA POLLING (multi-symbol) ==========
  useEffect(() => {
    let mounted = true;
    let interval: NodeJS.Timeout | null = null;

    const poll = async () => {
      if (pollInFlightRef.current) return;
      // In worker monitor mode: hydration handled by the worker heartbeat effect above.
      // Only run the local poll for quotes/data-health; skip entry/exit writes.
      if (workerMonitorMode) {
        // Still update data health so the UI has a live quote
        pollInFlightRef.current = false;
        return;
      }
      // Consume catch-up poll budget (set by visibilitychange handler)
      if (catchUpPollsRef.current > 0) catchUpPollsRef.current--;
      pollInFlightRef.current = true;
      const debug502 = process.env.NEXT_PUBLIC_SIMULATE_FUTURES_502 === "1";
      const klineQuery = (sym: string, tf: BarInterval = "1m") =>
        `/api/btc/futures-klines?symbol=${encodeURIComponent(sym)}&interval=${tf}${debug502 ? "&debugFutures502=1" : ""}`;

      /** PR 10 — multi-TF cache. Always populated for "1m"; other TFs only when
       *  the active roster declares strategies needing them. Signal eval routes
       *  each strategy to its primaryBarInterval payload via this map.        */
      const payloadsByInterval = new Map<BarInterval, Map<string, KlinePayload>>();

      try {
        void flushTradeSyncQueue(cloudAccountKey);
        const payloads = new Map<string, KlinePayload>();
        const symbolIssues: FuturesDataHealthSymbolIssue[] = [];

        for (const batch of chunkArray(activeSymbols, SYMBOL_FETCH_CHUNK)) {
          const results = await Promise.all(
            batch.map(async (sym): Promise<{ sym: string; payload: KlinePayload | null; issue: FuturesDataHealthSymbolIssue | null }> => {
              try {
                const res = await fetch(klineQuery(sym, "1m"), { cache: "no-store" });
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

        // Always populate "1m" first — it powers regime/mark/exits/probe.
        payloadsByInterval.set("1m", payloads);
        // Refresh the 1m slot of the persistent cache + bump its timestamp so
        // skip-when-not-due accounting stays consistent across all TFs.
        cachedPayloadsRef.current.set("1m", payloads);
        lastFetchedByIntervalRef.current.set("1m", Date.now());

        // PR 10 (cadence) — additionally fetch every non-1m TF the active roster
        // needs, BUT skip the fetch when the per-TF cadence (INTERVAL_POLL_MS)
        // hasn't elapsed since its last fetch. Cached payloads are reused on
        // skipped polls, so signal eval always has data — just not freshly
        // refetched. Failures are soft: a skipped/failed TF means the strategy
        // is skipped that poll, not the whole desk.
        const extraIntervals = requiredBarIntervals.filter((tf) => tf !== "1m");
        for (const tf of extraIntervals) {
          const cadenceMs = INTERVAL_POLL_MS[tf];
          const lastFetched = lastFetchedByIntervalRef.current.get(tf) ?? 0;
          const dueNow = Date.now() - lastFetched >= cadenceMs;
          if (!dueNow) {
            // Reuse cached payloads from a previous tick (if any).
            const cached = cachedPayloadsRef.current.get(tf);
            if (cached) payloadsByInterval.set(tf, cached);
            continue;
          }
          const tfMap = new Map<string, KlinePayload>();
          for (const batch of chunkArray(activeSymbols, SYMBOL_FETCH_CHUNK)) {
            const results = await Promise.all(
              batch.map(async (sym): Promise<{ sym: string; payload: KlinePayload | null }> => {
                try {
                  const res = await fetch(klineQuery(sym, tf), { cache: "no-store" });
                  if (!res.ok) return { sym, payload: null };
                  const j = (await res.json()) as KlinePayload;
                  if (!j || j.ok !== true || !Array.isArray(j.candles) || j.candles.length < MIN_BARS) {
                    return { sym, payload: null };
                  }
                  return { sym, payload: j };
                } catch {
                  return { sym, payload: null };
                }
              }),
            );
            for (const r of results) {
              if (r.payload) tfMap.set(r.sym, r.payload);
            }
          }
          payloadsByInterval.set(tf, tfMap);
          cachedPayloadsRef.current.set(tf, tfMap);
          lastFetchedByIntervalRef.current.set(tf, Date.now());
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
          setCanonicalMark(primary.markPrice, "engine-poll");
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
          return applyMarkToFuturesPosition(p, d.markPrice, d.lastPrice, {
            fundingRate: d.fundingRate,
            nowMs: now,
          });
        });

        const exitJobs: {
          pos: BTCFuturesPosition;
          exitPrice: number;
          reason: NonNullable<BTCFuturesPosition["exitReason"]>;
        }[] = [];

        const paperSlMulLive = paperDeskMode ? btcFtPaperSlPctMulFromEnv() : 1;
        const patchedPositions = markedPositions.map((pos) => {
          const d = payloads.get(pos.symbol);
          if (!d || d.candles.length < MIN_BARS) return pos;
          let posForExit = pos;
          if (paperDeskMode) {
            const strat = STRAT_DEFS.find((s) => s.id === pos.strategyId);
            const stops = paperDeskWidenPositionStops(pos, strat, paperSlMulLive);
            const adaptiveSl =
              pos.side === "LONG"
                ? Math.min(pos.adaptiveSl, stops.adaptiveSl)
                : Math.max(pos.adaptiveSl, stops.adaptiveSl);
            posForExit = { ...pos, ...stops, adaptiveSl, slPrice: adaptiveSl };
          }
          const profitExit = profitModeExitConfig(profitModeCfg);
          const { patched, close } = resolveFuturesExitStep(posForExit, profileHoldTimeMulRef.current, now, {
            takerFeePct: TAKER_FEE_PCT,
            exitSlippageBps: slippageBpsOverride ?? deskSlippageBpsFromEnv(),
            ...(paperDeskMode
              ? profitExit
                ? {
                    minAgeBeforeSlMs: btcFtPaperMinHoldBeforeSlMinFromEnv() * 60_000,
                    breakevenTriggerProgress: profitExit.breakevenTriggerProgress,
                    trailActivationProgress: profitExit.trailActivationProgress,
                    profitLockMinProgress: profitExit.profitLockMinProgress,
                    profitLockMinNetUsd: profitExit.profitLockMinNetUsd,
                    minGrossMultipleOfFees: profitExit.minGrossMultipleOfFees,
                    paperTpBeforeSl: true,
                    ...(profitExit.disablePaperQuickTp
                      ? {}
                      : {
                          paperQuickTpMinNetUsd: btcFtPaperQuickTpMinNetUsdFromEnv(),
                        }),
                  }
                : {
                    minAgeBeforeSlMs: btcFtPaperMinHoldBeforeSlMinFromEnv() * 60_000,
                    breakevenTriggerProgress: 0.98,
                    trailActivationProgress: 1.01,
                    profitLockMinProgress: 0.18,
                    profitLockMinNetUsd: btcFtPaperQuickTpMinNetUsdFromEnv(),
                    paperTpBeforeSl: true,
                    paperQuickTpMinNetUsd: btcFtPaperQuickTpMinNetUsdFromEnv(),
                  }
              : {
                  profitLockMinNetUsd: deskProfitLockMinNetUsdFromEnv(),
                  profitLockMinProgress: deskProfitLockMinProgressFromEnv(),
                }),
          });
          const merged: BTCFuturesPosition = { ...pos, ...patched };
          // Sync slPrice with adaptiveSl so SL checks use the most protective stop.
          const syncedSl =
            merged.side === "LONG"
              ? Math.max(merged.adaptiveSl, merged.slPrice)
              : Math.min(merged.adaptiveSl, merged.slPrice);
          const finalMerged =
            syncedSl !== merged.slPrice ? { ...merged, slPrice: syncedSl } : merged;
          if (close.shouldClose && close.reason) {
            exitJobs.push({ pos: finalMerged, exitPrice: close.exitPrice, reason: close.reason });
          }
          return finalMerged;
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

        // Intraday soft DD lock (P2.3.4): roll daily snapshot at UTC midnight, pause at −2%.
        const currentUtcDay = new Date(now).toISOString().slice(0, 10);
        if (currentUtcDay !== dailyStartUtcDayRef.current) {
          dailyStartUtcDayRef.current = currentUtcDay;
          dailyStartEquityRef.current = equityForDrawdown;
          intradayDdPausedRef.current = false;
        }
        const dailyDdPct =
          dailyStartEquityRef.current > 0
            ? ((dailyStartEquityRef.current - equityForDrawdown) / dailyStartEquityRef.current) * 100
            : 0;
        if (!disableIntradayDdLock && dailyDdPct >= INTRADAY_SOFT_DD_LOCK_PCT) {
          intradayDdPausedRef.current = true;
        }

        setPositions(survivors);

        for (const job of exitJobs) {
          closePositionRef.current(job.pos, job.exitPrice, job.reason);
        }

        let openCount = survivors.length;
        const lastClosedMs = tradesRef.current.reduce(
          (max, t) => Math.max(max, new Date(t.closedAt).getTime()),
          0,
        );
        // Fresh sessions with zero closes still need an idle clock; otherwise
        // paperEnsureTrades never nudges entries until after a trade has closed.
        const lastEntryActivityMs = lastClosedMs > 0 ? lastClosedMs : researchMountedAtRef.current;
        const quietEntryBoost = paperQuietEntryBoost(
          paperEnsureTrades,
          now,
          lastEntryActivityMs,
          survivors.length,
        );
        const occupied = new Set(survivors.map((p) => `${p.symbol}:${p.strategyId}`));
        const utcSession = entryUtcSessionOverride ?? deskEntryUtcSessionFromEnv();
        const utcHour = new Date(now).getUTCHours();
        const entryUtcSessionOpen = isUtcHourInSession(
          utcHour,
          utcSession.startHour,
          utcSession.endHour,
        );

        // Do not gate entries on statusRef === READY: setStatus is async and statusRef updates next render,
        // so same poll tick would never open trades. Use live payload presence instead.
        const pollDebug = entryDebugEnabled
          ? {
              ...createEmptyEntryPollDebug(activeSignalThreshold),
              pollAt: now,
              pauseEntries: pauseRef.current,
              drawdownLocked: drawdownEntryPausedRef.current,
              intradayDdLocked: intradayDdPausedRef.current,
              hasMarketData,
              payloadsReady: payloads.size,
              symbolsRequested: activeSymbols.length,
              dataHealthStatus: status,
              activeStratCount: activeStratDefs.length,
              disabledStratCount: disabledRef.current.length,
              autoDisabledStratCount: autoDisabledRef.current.size,
              utcHour,
              entryUtcSessionOpen,
              sessionSkipTotal: deskSkippedOutsideSessionRef.current,
            }
          : null;
        entryDebugLiveRef.current = pollDebug;

        const sessionTradeCount = tradesRef.current.filter(
          (t) => new Date(t.openedAt).getTime() >= researchMountedAtRef.current,
        ).length;
        const bootstrapWaitMs = paperEnsureTrades ? 90_000 : 3 * 60_000;
        const bootstrapProbeDue =
          paperBootstrapProbe &&
          sessionTradeCount === 0 &&
          now - researchMountedAtRef.current >= bootstrapWaitMs;
        const shouldRunProbe =
          (forceProbeOpen || bootstrapProbeDue) &&
          hasMarketData &&
          !pauseRef.current &&
          !drawdownEntryPausedRef.current &&
          !intradayDdPausedRef.current &&
          !forceProbeOpenedRef.current;

        if (shouldRunProbe) {
          const probePayload = primary;
          if (probePayload && probePayload.candles.length >= MIN_BARS) {
            forceProbeOpenedRef.current = true;
            const closes = probePayload.candles.map((c) => c.close);
            const opens = probePayload.candles.map((c) => c.open);
            const highs = probePayload.candles.map((c) => c.high);
            const lows = probePayload.candles.map((c) => c.low);
            const volumes = probePayload.candles.map((c) => c.volume);
            const lastBarMs = probePayload.candles[probePayload.candles.length - 1]?.time;
            const input = buildSignalInputs(opens, closes, highs, lows, volumes, probePayload.markPrice, lastBarMs);
            const probeStrat: StratDef = {
              id: 99999,
              name: bootstrapProbeDue ? "PAPER_BOOTSTRAP_PROBE" : "DEV_FORCE_PROBE_OPEN",
              category: "Probe",
              signalKey: "DEV_FORCE_PROBE_LONG",
              slPct: 1.0,
              tpPct: 1.2,
              cooldownMin: 1,
              holdMinutes: 5,
              confluenceMin: 1,
            };
            const opened = openPositionRef.current(probeStrat, "LONG", probePayload.lastPrice, probePayload.markPrice, probePayload === primary ? PRIMARY_QUOTE_SYMBOL : activeSymbols[0] ?? PRIMARY_QUOTE_SYMBOL, {
              atr14: Math.max(input.atr14, probePayload.markPrice * 0.003),
              regime: "chop",
              entryBook: survivors.map((p) => ({ side: p.side, notional: p.notional })),
              equityUsd: balanceRef.current + survivors.reduce((s, p) => s + p.unrealizedPnl, 0),
              entryPriorityScore: 999,
            });
            if (process.env.NODE_ENV === "development" || bootstrapProbeDue) {
              console.info("[btc-futures-paper] paper probe open", {
                bootstrap: bootstrapProbeDue,
                opened: Boolean(opened),
                symbol: opened?.symbol,
                positionId: opened?.id,
              });
            }
          }
        }

        if (hasMarketData && !pauseRef.current && !drawdownEntryPausedRef.current && !intradayDdPausedRef.current) {
          const productionTradesForHealth = tradesRef.current.filter(
            (t) => !isProbeOrBootstrapTrade({ strategyName: t.strategyName }),
          );
          const rollingHealthForEntry =
            productionTradesForHealth.length >= 5
              ? computeRollingHealthCheckFromEngineTrades(productionTradesForHealth)
              : null;

          const equityForEntry = balanceRef.current + survivors.reduce((s, p) => s + p.unrealizedPnl, 0);
          const replaceWeakest = deskEntryReplaceWeakestFromEnv();
          type EntryCandidate = {
            strat: StratDef;
            side: Side;
            symbol: string;
            lastPrice: number;
            markPrice: number;
            atr14: number;
            regime: RegimeTag;
            priority: number;
            slotKey: string;
            signalScore: number;
            stratInput: FuturesSignalInputs;
            inputByTf: Map<BarInterval, FuturesSignalInputs>;
            signalContributions?: Array<{ reason: string; pts: number }>;
          };
          const entryCandidates: EntryCandidate[] = [];
          tickSignalTraceRef.current = [];

          for (const symbol of activeSymbols) {
            const d = payloads.get(symbol);
            if (!d || d.candles.length < MIN_BARS) continue;

            const closes = d.candles.map((c) => c.close);
            const opens = d.candles.map((c) => c.open);
            const highs = d.candles.map((c) => c.high);
            const lows = d.candles.map((c) => c.low);
            const volumes = d.candles.map((c) => c.volume);
            const lastBarMs = d.candles[d.candles.length - 1]?.time;
            const input = buildSignalInputs(opens, closes, highs, lows, volumes, d.markPrice, lastBarMs);
            const regime = classifyRegimeTagFrom1mOhlcv(opens, highs, lows, closes, volumes);

            /** PR 10 — cache per-TF signal inputs for this symbol so we only call
             *  buildSignalInputs once per (symbol, TF) per poll instead of per
             *  strategy. The 1m input is already built above; other TFs are
             *  built lazily on first lookup.                                  */
            const inputByTf = new Map<BarInterval, FuturesSignalInputs>();
            inputByTf.set("1m", input);
            const getInputForTf = (tf: BarInterval): FuturesSignalInputs | null => {
              if (tf === "1m") return input;
              const cached = inputByTf.get(tf);
              if (cached) return cached;
              const tfPayload = payloadsByInterval.get(tf)?.get(symbol);
              if (!tfPayload || tfPayload.candles.length < MIN_BARS) return null;
              const tCloses = tfPayload.candles.map((c) => c.close);
              const tOpens = tfPayload.candles.map((c) => c.open);
              const tHighs = tfPayload.candles.map((c) => c.high);
              const tLows = tfPayload.candles.map((c) => c.low);
              const tVols = tfPayload.candles.map((c) => c.volume);
              const tLastBar = tfPayload.candles[tfPayload.candles.length - 1]?.time;
              const tInput = buildSignalInputs(tOpens, tCloses, tHighs, tLows, tVols, tfPayload.markPrice, tLastBar);
              inputByTf.set(tf, tInput);
              return tInput;
            };
            if (symbol === PRIMARY_QUOTE_SYMBOL) {
              deskLastRegimeTagRef.current = regime;
              if (deskHoldTuningAnalysisModeEnabled()) {
                const buf = deskRegimePollHistoryRef.current;
                buf.push(regime);
                const cap = deskRegimeWatchPollWindowFromEnv();
                while (buf.length > cap) buf.shift();
              }
              if (deskRegimeHistogramDevPersistEnabled()) {
                deskRegime24hEventsRef.current = appendPrunedDeskRegimePersistEvent(
                  deskRegime24hEventsRef.current,
                  { t: now, tag: regime },
                  now,
                );
                deskRegimeLsLastSaveRef.current = now; // timestamp kept for interval throttle
              }
            }

            for (const strat of activeStratDefs) {
              if (pollDebug) pollDebug.evalPairs += 1;
              if (
                disabledRef.current.includes(strat.id) ||
                autoDisabledRef.current.has(strat.id) ||
                isStrategyRuntimeBlocked(strat.id)
              ) {
                if (pollDebug) pollDebug.failDisabled += 1;
                if (isStrategyRuntimeBlocked(strat.id)) {
                  logSkipReason(strat.id, "DISABLED", { reason: "RUNTIME_BLOCKLIST" });
                }
                continue;
              }
              const slotKey = `${symbol}:${strat.id}`;
              if (occupied.has(slotKey)) {
                if (pollDebug) pollDebug.failOccupied += 1;
                continue;
              }
              if ((stratCooldownsRef.current[slotKey] ?? 0) > Date.now()) {
                if (pollDebug) pollDebug.failCooldown += 1;
                continue;
              }

              if (suspendedStrategiesRef.current.has(strat.id)) {
                if (pollDebug) pollDebug.failDisabled += 1;
                logSkipReason(strat.id, "ROTATION_SUSPENDED", {
                  score: "below threshold",
                });
                continue;
              }

              if (
                !profitModeAllowsRotationStrategy(
                  strat.id,
                  rotationReportRef.current,
                  profitModeCfg,
                )
              ) {
                profitModeSkipCountRef.current += 1;
                entryRotationBlockedCountRef.current += 1;
                logSkipReason(strat.id, "PROFIT_MODE_ROTATION", {
                  only: "ACTIVE|PROMOTED",
                });
                if (pollDebug) pollDebug.failDisabled += 1;
                continue;
              } else if (profitModeCfg.enabled) {
                entryRotationAllowedCountRef.current += 1;
              }

              /** PR 10 — pick the right signal-inputs cache for this strategy's
               *  primaryBarInterval. Strategies without a declared TF (legacy
               *  CORE 20, premium) keep using 1m. If the strategy's TF payload
               *  didn't arrive (network failure, insufficient bars), we skip
               *  the strategy this poll rather than silently scoring on 1m,
               *  which would defeat the whole TF-routing purpose.            */
              const stratTf: BarInterval = isBarInterval(strat.primaryBarInterval)
                ? strat.primaryBarInterval
                : "1m";
              const stratInput = getInputForTf(stratTf);
              if (!stratInput) {
                if (pollDebug) pollDebug.failDisabled += 1;
                continue;
              }

              // Research-mode anti-churn gate: per-strategy daily close cap.
              // Skips entries once the strategy has hit researchDailyStratCap()
              // closed trades within the current UTC day. Outside research mode
              // researchDailyStratCap() returns 0 → gate is a no-op.
              // FIREHOSE MODE: bypass entirely (volume IS the goal there).
              const profitCap = profitModeDailyStratCap(profitModeCfg);
              const dailyCap = deskFirehoseModeEnabled()
                ? 0
                : profitCap > 0
                  ? profitCap
                  : researchDailyStratCap();
              if (dailyCap > 0) {
                const todayUtcStartMs = Date.UTC(
                  new Date().getUTCFullYear(),
                  new Date().getUTCMonth(),
                  new Date().getUTCDate(),
                );
                const todayCloses = tradesRef.current.filter(
                  (t) => t.strategyId === strat.id && new Date(t.closedAt).getTime() >= todayUtcStartMs,
                ).length;
                if (todayCloses >= dailyCap) {
                  deskSkippedDailyStratCapRef.current += 1;
                  continue;
                }
              }

              // Research-mode anti-churn gate: template-family dedup.
              // If another open position already belongs to the same template family
              // (e.g. BTCFT_VWAP_V0_LONG), skip — pays 1× fees for 1 signal instead of N×.
              if (dailyCap > 0) {
                const myFamily = btcFtTemplateFamilyKey(strat.name);
                const familyAlreadyOpen = positionsRef.current.some(
                  (p) => btcFtTemplateFamilyKey(p.strategyName ?? "") === myFamily,
                );
                if (familyAlreadyOpen) {
                  deskSkippedTemplateFamilyRef.current += 1;
                  continue;
                }
              }

              const signal = evalMinuteSignal(stratInput, strat);
              const stratTradeCount = tradesRef.current.filter((t) => t.strategyId === strat.id).length;
              const ensureDataThresholdDrop = researchEnsureTrades &&
                now - researchMountedAtRef.current >= 2 * 60 * 60 * 1000 &&
                stratTradeCount < 3
                ? 4
                : paperEnsureThresholdDrop(
                    paperEnsureTrades,
                    now,
                    researchMountedAtRef.current,
                    stratTradeCount,
                  );
              // P1.1.2: per-strategy dynamic threshold; PR-8: regime + health adaptive boost.
              const stratBaseThreshold = strat.dynamicThreshold ?? activeSignalThreshold;
              const thresholdFloor = paperEnsureTrades ? 16 : 18;
              const adaptiveBase = computeAdaptiveThreshold(
                stratBaseThreshold,
                regime,
                rollingHealthForEntry?.grade ?? null,
                rollingHealthForEntry?.feePctOfAbsGross ?? null,
              );
              const profitAwareBase = applyProfitModeThreshold(
                adaptiveBase,
                regime,
                profitModeCfg,
              );
              // Profit mode should harden entries, not deadlock a brand-new
              // paper desk. Until we have enough production closes for a real
              // scorecard, allow a capped paper-discovery drop; quality + MTF
              // gates still remain active below.
              const thresholdDrop = paperEnsureEffectiveThresholdDrop(
                profitModeCfg.enabled,
                ensureDataThresholdDrop,
                productionTradesForHealth.length,
              );
              const effectiveThresholdForStrat = Math.max(
                thresholdFloor,
                profitAwareBase - thresholdDrop - quietEntryBoost,
              );
              // PR-6 Change 4: block strategies with an explicit regimes[] list when
              // the current regime isn't in it — catch this early before candidate push.
              const regimeMatch = strat.regimes?.includes(regime) ?? true;
              if (strat.regimes && strat.regimes.length > 0 && !regimeMatch) {
                tickSignalTraceRef.current.push(createTraceRow({
                  tickAt: now, mode: "browser", symbol,
                  strategyId: strat.id, strategyName: strat.name, category: strat.category,
                  status: "REJECTED", gate: "REGIME",
                  reason: `Regime ${String(regime)} not in [${strat.regimes?.join(",")}]`,
                  signalScore: 0, requiredThreshold: effectiveThresholdForStrat,
                  confirmPassed: false, regime: String(regime), regimeAllowed: false,
                }));
                logSkipReason(strat.id, "REGIME_BLOCKED", { regime, stratRegimes: strat.regimes });
                if (pollDebug) pollDebug.failOpenRegime += 1;
                deskSkippedByRegimeRef.current[regime] += 1;
                continue;
              }

              const templateFamily =
                strat.templateFamily ?? btcFtTemplateFamilyKey(strat.name);
              if (
                !profitModeAllowsStrategyInChop(
                  templateFamily,
                  strat.name,
                  strat.regimes,
                  regime,
                  profitModeCfg,
                )
              ) {
                profitModeSkipCountRef.current += 1;
                logSkipReason(strat.id, "PROFIT_MODE_CHOP_TEMPLATE", {
                  templateFamily,
                  regime,
                });
                if (pollDebug) pollDebug.failOpenRegime += 1;
                continue;
              }

              const confirmPasses =
                passesEntryConfirmation(stratInput, strat) ||
                (relaxEntryConfirmation && passesRelaxedDeskEntryConfirmation(stratInput, strat));
              if (signal.score >= effectiveThresholdForStrat && confirmPasses) {
                const side = strat.signalKey.includes("SHORT") ? "SHORT" : "LONG";
                tickSignalTraceRef.current.push(createTraceRow({
                  tickAt: now, mode: "browser", symbol,
                  strategyId: strat.id, strategyName: strat.name, category: strat.category, side,
                  status: "CANDIDATE", gate: "SIGNAL",
                  reason: `Score ${signal.score.toFixed(1)} >= ${effectiveThresholdForStrat} + confirm pass`,
                  signalScore: signal.score, requiredThreshold: effectiveThresholdForStrat,
                  confirmPassed: true, regime: String(regime), regimeAllowed: true,
                  contributions: signal.contributions,
                }));
                if (paperDeskMode) {
                  const momentumAligned =
                    side === "LONG"
                      ? stratInput.momentum3 > 0 && stratInput.momentum6 >= 0
                      : stratInput.momentum3 < 0 && stratInput.momentum6 <= 0;
                  if (!momentumAligned) {
                    if (pollDebug) pollDebug.failConfirm += 1;
                    continue;
                  }
                }
                entryCandidates.push({
                  strat,
                  side,
                  symbol,
                  lastPrice: d.lastPrice,
                  markPrice: d.markPrice,
                  atr14: stratInput.atr14,
                  regime,
                  priority: paperEntryPriorityScore({
                    signalScore: signal.score,
                    stratId: strat.id,
                    category: strat.category,
                    regimeMatch,
                    deskTpWidened: strat.deskTpWidened,
                    slPct: strat.slPct,
                    tpPct: strat.tpPct,
                  }),
                  slotKey,
                  signalScore: signal.score,
                  stratInput,
                  inputByTf: new Map(inputByTf),
                  signalContributions: signal.contributions,
                });
                if (pollDebug) pollDebug.candidatesBuilt += 1;
              } else {
                const _side = strat.signalKey.includes("SHORT") ? "SHORT" as const : "LONG" as const;
                if (signal.score < effectiveThresholdForStrat) {
                  tickSignalTraceRef.current.push(createTraceRow({
                    tickAt: now, mode: "browser", symbol,
                    strategyId: strat.id, strategyName: strat.name, category: strat.category, side: _side,
                    status: "EVALUATED", gate: "SIGNAL",
                    reason: `Score ${signal.score.toFixed(1)} below ${effectiveThresholdForStrat}`,
                    signalScore: signal.score, requiredThreshold: effectiveThresholdForStrat,
                    confirmPassed: false, regime: String(regime), regimeAllowed: true,
                    contributions: signal.contributions,
                  }));
                  if (pollDebug) {
                    pollDebug.failSignal += 1;
                    logSkipReason(strat.id, "THRESHOLD_NOT_MET", {
                      score: signal.score,
                      required: effectiveThresholdForStrat,
                      regime,
                      grade: rollingHealthForEntry?.grade ?? null,
                    });
                  }
                } else {
                  tickSignalTraceRef.current.push(createTraceRow({
                    tickAt: now, mode: "browser", symbol,
                    strategyId: strat.id, strategyName: strat.name, category: strat.category, side: _side,
                    status: "FIRED", gate: "CONFIRM",
                    reason: "Confirmation check failed",
                    signalScore: signal.score, requiredThreshold: effectiveThresholdForStrat,
                    confirmPassed: false, regime: String(regime), regimeAllowed: true,
                    contributions: signal.contributions,
                  }));
                  if (pollDebug) pollDebug.failConfirm += 1;
                }
              }
            }
          }

          entryCandidates.sort((a, b) => b.priority - a.priority);

          const intraBook: { side: Side; notional: number }[] = [];
          const workingPositions: BTCFuturesPosition[] = [...survivors];
          const burstCtx: EntryBurstContext = createEntryBurstContext();

          const maxSpreadPct = deskMaxLastMarkSpreadPctFromEnv();
          const maxOpenPerCategory = deskMaxOpenPerCategoryFromEnv();
          const categoryByStrategyId = new Map(activeStratDefs.map((s) => [s.id, s.category]));
          let categoryOpenCounts = countOpenByCategory(workingPositions, categoryByStrategyId);

          const burstOpts = {
            maxPerSymbolPerPoll: paperEnsureTrades
              ? Math.max(4, entryBurstMaxPerSymbolFromEnv())
              : entryBurstMaxPerSymbolFromEnv(),
            maxPerFamilyPerPoll: paperEnsureTrades ? 2 : ENTRY_BURST_MAX_PER_FAMILY_DEFAULT,
            oppositeSideWindowMs: paperEnsureTrades ? 60_000 : ENTRY_OPPOSITE_SIDE_WINDOW_MS,
            nowMs: now,
          };

          const tryOpenCandidate = (c: EntryCandidate): boolean => {
            if (pollDebug) pollDebug.openAttempts += 1;

            // ---- Burst guard: per-symbol cap, per-family cap, opposite-side block ----
            const burstResult = checkEntryBurstGuard(
              c.symbol,
              c.side,
              btcFtTemplateFamilyKey(c.strat.name),
              burstCtx,
              workingPositions,
              tradesRef.current,
              burstOpts,
            );
            if (burstResult.blocked) {
              if (burstResult.reason === "burst_symbol") deskBurstBlockedRef.current += 1;
              else if (burstResult.reason === "family_cap") deskFamilyCapBlockedRef.current += 1;
              else deskOppositeSideBlockedRef.current += 1;
              return false;
            }

            if (paperLastMarkSpreadPct(c.lastPrice, c.markPrice) > maxSpreadPct) {
              deskSkippedSpreadRef.current += 1;
              if (pollDebug) pollDebug.failSpread += 1;
              return false;
            }
            // ATR% pre-filter: skip when market is too thin to cover fees (atr/price < 0.06%).
            if (c.markPrice > 0 && c.atr14 / c.markPrice < 0.0006) {
              deskSkippedMinExpectedMoveRef.current += 1;
              logSkipReason(c.strat.id, "ATR_TOO_LOW", {
                atrPct: (c.atr14 / c.markPrice).toFixed(6),
                required: 0.0006,
              });
              if (pollDebug) pollDebug.failMinMove += 1;
              return false;
            }
            const maxSameSide = profitModeMaxSameSide(c.regime, profitModeCfg);
            if (isSameSideCapped(workingPositions, c.side, maxSameSide)) {
              deskSkippedCorrelatedCapRef.current += 1;
              logSkipReason(c.strat.id, "CORRELATED_CAP", {
                side: c.side,
                openCount: workingPositions.filter((p) => p.side === c.side).length,
                max: maxSameSide,
              });
              if (pollDebug) pollDebug.failSameDirCap += 1;
              return false;
            }

            const stratClosed = tradesRef.current.filter(
              (t) =>
                t.strategyId === c.strat.id &&
                !isProbeOrBootstrapTrade({ strategyName: t.strategyName }),
            );
            const strategyWinRate = stratClosed.length
              ? stratClosed.filter((t) => t.netPnl > 0).length / stratClosed.length
              : 0;
            const cooldownRemainMs = Math.max(
              0,
              (stratCooldownsRef.current[c.slotKey] ?? 0) - now,
            );
            const spreadPct =
              c.markPrice > 0 ? paperLastMarkSpreadPct(c.lastPrice, c.markPrice) / 100 : 0;
            const atrPct = c.markPrice > 0 ? c.atr14 / c.markPrice : 0;
            const regimeFitsStrategy = c.strat.regimes?.includes(c.regime) ?? true;

            gateEvaluationCountRef.current += 1;

            const quality = scoreSignalQuality({
              signalScore: c.signalScore,
              atrPct,
              spreadPct,
              volumeRatio: c.stratInput.volRatio,
              regime: c.regime,
              regimeFitsStrategy,
              ema20AboveEma50: c.stratInput.fast > c.stratInput.slow,
              priceAboveEma20: c.markPrice > c.stratInput.fast,
              side: c.side,
              openPositionCount: workingPositions.length,
              sameSideCount: workingPositions.filter((p) => p.side === c.side).length,
              hoursIntoSession: new Date().getUTCHours(),
              strategyWinRate,
              strategyTrades: stratClosed.length,
              cooldownRemainMs,
            });

            const profitQuality = profitModePassesQuality(
              quality.total,
              quality.isHighQuality,
              c.regime,
              profitModeCfg,
            );
            if (!quality.pass || !profitQuality.pass) {
              qualitySkipCountRef.current += 1;
              if (!profitQuality.pass) profitModeSkipCountRef.current += 1;
              logSkipReason(c.strat.id, profitQuality.reason ?? "SIGNAL_QUALITY_FAIL", {
                qualityScore: quality.total,
                minPass: profitModeCfg.enabled
                  ? profitModeCfg.minQualityScore
                  : quality.minPassScore,
                deductions: quality.deductions,
              });
              if (pollDebug) pollDebug.failMinMove += 1;
              return false;
            }
            lastSignalQualityRef.current = quality;

            const tfSnapshots = buildTFSnapshotsFromInputMap(c.inputByTf);
            const mtfResult = computeMTFConfluence(tfSnapshots, c.side);
            lastMtfConfluenceRef.current = mtfResult;
            const mtfMin = profitModeMtfMinScore(profitModeCfg, 55);
            const mtfSkip = mtfSkipReason(mtfResult, mtfMin);
            if (mtfSkip) {
              mtfSkipCountRef.current += 1;
              if (profitModeCfg.enabled) profitModeSkipCountRef.current += 1;
              logSkipReason(c.strat.id, mtfSkip, {
                confluenceScore: mtfResult.confluenceScore,
                overallBias: mtfResult.overallBias,
                minScore: mtfMin,
              });
              if (pollDebug) pollDebug.failConfirm += 1;
              return false;
            }

            const counterTrendSkip = profitModeCounterTrendSkip(
              c.side,
              mtfResult,
              profitModeCfg,
            );
            if (counterTrendSkip) {
              profitModeSkipCountRef.current += 1;
              logSkipReason(c.strat.id, counterTrendSkip, {
                overallBias: mtfResult.overallBias,
                side: c.side,
              });
              if (pollDebug) pollDebug.failConfirm += 1;
              return false;
            }

            if (!entryUtcSessionOpen) {
              deskSkippedOutsideSessionRef.current += 1;
              if (pollDebug) pollDebug.failSession += 1;
              return false;
            }
            const category = c.strat.category?.trim() || "unknown";
            const categoryOpen = categoryOpenCounts.get(category) ?? 0;
            if (!canOpenCategory(category, categoryOpen, maxOpenPerCategory)) {
              deskSkippedCategoryCapRef.current += 1;
              if (pollDebug) pollDebug.failCategoryCap += 1;
              return false;
            }
            const entryBook = [
              ...workingPositions.map((p) => ({ side: p.side, notional: p.notional })),
              ...intraBook,
            ];
            const opened = openPositionRef.current(c.strat, c.side, c.lastPrice, c.markPrice, c.symbol, {
              atr14: c.atr14,
              regime: c.regime,
              entryBook,
              equityUsd: equityForEntry,
              entryPriorityScore: c.priority,
              signalContributions: c.signalContributions,
            });
            if (!opened) return false;
            if (pollDebug) pollDebug.openedThisPoll += 1;
            intraBook.push({ side: opened.side, notional: opened.notional });
            workingPositions.push(opened);
            categoryOpenCounts.set(category, categoryOpen + 1);
            occupied.add(c.slotKey);
            openCount += 1;
            recordBurstEntry(burstCtx, c.symbol, btcFtTemplateFamilyKey(c.strat.name));
            return true;
          };

          for (const c of entryCandidates) {
            if (occupied.has(c.slotKey)) continue;
            const opened = tryOpenCandidate(c);
            const traceIdx = tickSignalTraceRef.current.findIndex(
              (r) => r.strategyId === c.strat.id && r.symbol === c.symbol && r.status === "CANDIDATE",
            );
            if (traceIdx >= 0) {
              if (opened) {
                tickSignalTraceRef.current[traceIdx] = {
                  ...tickSignalTraceRef.current[traceIdx],
                  status: "OPENED",
                  gate: "OPENED",
                  reason: "opened",
                  openAttempted: true,
                };
              } else {
                tickSignalTraceRef.current[traceIdx] = {
                  ...tickSignalTraceRef.current[traceIdx],
                  status: "REJECTED",
                  gate: "QUALITY",
                  reason: "Rejected in post-candidate gate (quality/MTF/spread/session/category/same-side)",
                  openAttempted: true,
                };
              }
            }
          }

          // Finalize and store tick trace
          {
            const capped = capTraceRows(tickSignalTraceRef.current);
            const summary = summarizeSignalTrace(capped);
            latestSignalTraceSummaryRef.current = summary;
            tickSignalTraceRef.current = [];

            if (cloudAccountKey && capped.length > 0) {
              void fetch("/api/strategy-signal-trace", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                  accountKey: cloudAccountKey,
                  trace: {
                    tickAt: now,
                    mode: "browser",
                    symbol: PRIMARY_QUOTE_SYMBOL,
                    summary,
                    rows: capped,
                  },
                }),
              }).catch(() => { /* non-fatal */ });
            }
          }
        }

        if (pollDebug) {
          entryDebugLiveRef.current = null;
          const finalizedDebug = finalizeEntryPollDebug(pollDebug);

          // Build funnel snapshot and persist to MongoDB so /api/desk-entry-funnel
          // reflects the browser-side view when no VPS worker is running.
          if (cloudAccountKey) {
            const funnelSnap = funnelSnapshotFromBrowserDebug(finalizedDebug, {
              workerFresh: isWorkerHeartbeatFresh(workerLastPollAt),
              symbol: PRIMARY_QUOTE_SYMBOL,
              markPrice: primary?.markPrice ?? 0,
              bars: primary?.candles?.length ?? 0,
            });
            void fetch("/api/desk-entry-funnel", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ accountKey: cloudAccountKey, snapshot: funnelSnap }),
            }).catch(() => { /* non-fatal */ });
          }

          if (entryDebugEnabled) {
            const report = rotationReportRef.current;
            const rosterSize = activeStratDefs.length;
            const rotAllowed = entryRotationAllowedCountRef.current;
            const rotBlocked = entryRotationBlockedCountRef.current;
            const suspendedCount = suspendedStrategiesRef.current.size;
            const activeAndPromoted = report
              ? report.active.length + report.promoted.length
              : null;
            console.info(
              "[entry-debug] poll",
              `evalPairs=${finalizedDebug.evalPairs}`,
              `candidates=${finalizedDebug.candidatesBuilt}`,
              `opened=${finalizedDebug.openedThisPoll}`,
              `blocker=${finalizedDebug.dominantBlocker}`,
              `profitModeSkipΔ=${finalizedDebug.failDisabled}`,
              `roster=${rosterSize}`,
              `rotAllowed=${rotAllowed}`,
              `rotBlocked=${rotBlocked}`,
              `suspended=${suspendedCount}`,
              activeAndPromoted !== null
                ? `active+promoted=${activeAndPromoted}`
                : "rotation=no-report",
            );
          }
          setEntryDebug(finalizedDebug);
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
      } finally {
        pollInFlightRef.current = false;
      }
    };

    const onVisibility = () => {
      if (document.visibilityState === "visible" && mounted) void poll();
    };

    poll();
    interval = setInterval(poll, POLL_MS);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      mounted = false;
      if (interval) clearInterval(interval);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [activeStratDefs, activeSymbols, requiredBarIntervals, activeSignalThreshold, strategyProfile, regimeHistogramLsKey, cloudAccountKey, relaxEntryConfirmation, forceProbeOpen, paperBootstrapProbe, researchEnsureTrades, paperEnsureTrades, disableIntradayDdLock, entryUtcSessionOverride, entryDebugEnabled]);

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
      addDisabledStrategyIds,
      updateSuspendedStrategies,
      restoreRotationStrategy,
      exportCSV,
      exportJSON,
      dataHealth,
    };
  }, [positions, trades, balance, quote, status, pauseEntries, disabledStrategies, calculateStats, togglePause, resetPaperAccount, clearTradeHistory, setDisabledStrategiesHandler, addDisabledStrategyIds, updateSuspendedStrategies, restoreRotationStrategy, exportCSV, exportJSON, dataHealth]);

  // ========== DAILY RESET ==========
  useEffect(() => {
    const checkDay = () => {
      const now = new Date();
      const currentDate = now.getDate();
      if (currentDate !== dayStartDateRef.current) {
        setDayStartDate(currentDate);
        const bal = balanceRef.current;
        setDayStartBalance(bal);
        dailyStartEquityRef.current = bal;
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
    addDisabledStrategyIds,
    updateSuspendedStrategies,
    restoreRotationStrategy,
    exportCSV,
    exportJSON,
    strategyStatuses,
    strategyProfile,
    effectiveSignalThreshold: activeSignalThreshold,
    dataHealth,
    entryDebug: entryDebugEnabled ? entryDebug : null,
    setReplaySignFlipRate,
  };
}
