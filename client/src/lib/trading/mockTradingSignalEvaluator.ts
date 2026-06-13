/**
 * Mock-trading signal evaluator — replaces the deleted paper desk worker/browser
 * engine for trace generation. Produces CANDIDATE trace rows the mock engine ingests.
 */

import { resolveBtcFtActiveStrategyIds } from "@/lib/trading/btcFtRoster";
import {
  btcFtRelaxConfirmEnabledFromEnv,
  btcFtSignalThresholdFromEnv,
  buildPaperDeskStrategies,
  deskFirehoseModeEnabled,
} from "@/lib/trading/futuresDeskPolicy";
import type { RegimeTag } from "@/lib/trading/futuresStrategies";
import { FUTURES_STRAT_DEFS, type FuturesStratDef } from "@/lib/trading/futuresStrategies";
import {
  buildSignalInputs,
  classifyRegimeTagFrom1mOhlcv,
  describeEntryConfirmationFailure,
  evalMinuteSignal,
  passesEntryConfirmation,
  passesRelaxedDeskEntryConfirmation,
} from "@/lib/trading/futuresSignals";
import {
  capTraceRows,
  createTraceRow,
  summarizeSignalTrace,
  type SignalTraceSummary,
  type StrategySignalTraceRow,
} from "@/lib/ai/strategySignalTrace";
import type { MockTradingBar } from "@/lib/trading/mockTradingMarketData";

export const MOCK_TRADING_MIN_BARS = 30;
const TAKER_FEE_PCT = 0.001;
const DISCOVERY_NOTIONAL_USD = 300;

export type MockTradingSignalEvalResult = {
  tickAt: number;
  symbol: string;
  markPrice: number;
  regime: RegimeTag;
  bars: number;
  activeStrategies: number;
  evaluatedStrategies: number;
  candidateCount: number;
  rows: StrategySignalTraceRow[];
  summary: SignalTraceSummary;
  error: string | null;
};

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

export function resolveMockTradingStrategies(): FuturesStratDef[] {
  const { ids } = resolveBtcFtActiveStrategyIds();
  const idSet = new Set(ids);
  const raw = FUTURES_STRAT_DEFS.filter((s) => idSet.has(s.id) && !s.researchOnly);
  const { strategies } = buildPaperDeskStrategies(raw, {
    strategyIdAllowlist: null,
    minTpSlRatio: 2,
    allowFakeDiversity: false,
  });
  return strategies;
}

function mockTradingRelaxedConfirmEnabled(): boolean {
  if (process.env.NEXT_PUBLIC_MOCK_TRADING_RELAX_CONFIRM === "0") return false;
  return deskFirehoseModeEnabled() || btcFtRelaxConfirmEnabledFromEnv() || true;
}

function stableTraceId(tickAt: number, strategyId: number): string {
  const minuteBucket = Math.floor(tickAt / 60_000);
  return `mock-${minuteBucket}-${strategyId}`;
}

export function evaluateMockTradingSignals(args: {
  bars: readonly MockTradingBar[];
  markPrice: number;
  symbol: string;
  tickAt?: number;
  strategies?: readonly FuturesStratDef[];
}): MockTradingSignalEvalResult {
  const tickAt = args.tickAt ?? Date.now();
  const symbol = args.symbol;
  const markPrice = args.markPrice;
  const activeStrats = args.strategies ?? resolveMockTradingStrategies();

  if (activeStrats.length === 0) {
    const row = createTraceRow({
      traceId: stableTraceId(tickAt, 0),
      tickAt,
      mode: "browser",
      symbol,
      strategyId: 0,
      strategyName: "NO_STRATEGIES",
      status: "REJECTED",
      gate: "NO_STRATEGIES",
      reason: "No mock-trading strategies resolved",
      signalScore: 0,
      requiredThreshold: btcFtSignalThresholdFromEnv(),
      confirmPassed: false,
      regime: "unknown",
      regimeAllowed: false,
    });
    const rows = [row];
    return {
      tickAt,
      symbol,
      markPrice,
      regime: "chop",
      bars: args.bars.length,
      activeStrategies: 0,
      evaluatedStrategies: 0,
      candidateCount: 0,
      rows,
      summary: summarizeSignalTrace(rows),
      error: "no active strategies",
    };
  }

  if (args.bars.length < MOCK_TRADING_MIN_BARS) {
    return {
      tickAt,
      symbol,
      markPrice,
      regime: "chop",
      bars: args.bars.length,
      activeStrategies: activeStrats.length,
      evaluatedStrategies: 0,
      candidateCount: 0,
      rows: [],
      summary: summarizeSignalTrace([]),
      error: `insufficient bars: ${args.bars.length}`,
    };
  }

  const opens = args.bars.map((b) => b.open);
  const closes = args.bars.map((b) => b.close);
  const highs = args.bars.map((b) => b.high);
  const lows = args.bars.map((b) => b.low);
  const volumes = args.bars.map((b) => b.volume);
  const lastBarTimeMs = (args.bars[args.bars.length - 1]?.time ?? 0) * 1000;

  const signals = buildSignalInputs(opens, closes, highs, lows, volumes, markPrice, lastBarTimeMs);
  const regime = classifyRegimeTagFrom1mOhlcv(opens, highs, lows, closes, volumes) as RegimeTag;
  const baseThreshold = btcFtSignalThresholdFromEnv();
  const operatorRelaxed = mockTradingRelaxedConfirmEnabled();
  const atrFeeK = operatorRelaxed ? 0.45 : 1.0;
  const atrPct = markPrice > 0 ? signals.atr14 / markPrice : 0;

  const traceRows: StrategySignalTraceRow[] = [];
  let evalCount = 0;
  let candidateCount = 0;

  const traceBase = {
    tickAt,
    mode: "browser" as const,
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

    const regimeAllowed = !(strat.regimes && strat.regimes.length > 0 && !strat.regimes.includes(regime));
    if (!regimeAllowed) {
      traceRows.push(
        createTraceRow({
          ...traceBase,
          traceId: stableTraceId(tickAt, strat.id),
          strategyId: strat.id,
          strategyName: strat.name,
          category: strat.category,
          status: "REJECTED",
          gate: "REGIME",
          reason: `Regime ${regime} not in [${strat.regimes?.join(",")}]`,
          regimeAllowed: false,
        }),
      );
      continue;
    }

    const { score, contributions } = evalMinuteSignal(signals, strat);
    const side: "LONG" | "SHORT" = strat.signalKey.includes("SHORT") ? "SHORT" : "LONG";

    if (!Number.isFinite(score) || score < baseThreshold) {
      traceRows.push(
        createTraceRow({
          ...traceBase,
          traceId: stableTraceId(tickAt, strat.id),
          strategyId: strat.id,
          strategyName: strat.name,
          category: strat.category,
          side,
          status: "EVALUATED",
          gate: "SIGNAL",
          reason: `Score ${score.toFixed(1)} below threshold ${baseThreshold}`,
          signalScore: score,
          requiredThreshold: baseThreshold,
          contributions,
        }),
      );
      continue;
    }

    const passes = operatorRelaxed
      ? passesRelaxedDeskEntryConfirmation(signals, strat)
      : passesEntryConfirmation(signals, strat);
    if (!passes) {
      const confirmReason = describeEntryConfirmationFailure(signals, strat);
      traceRows.push(
        createTraceRow({
          ...traceBase,
          traceId: stableTraceId(tickAt, strat.id),
          strategyId: strat.id,
          strategyName: strat.name,
          category: strat.category,
          side,
          status: "FIRED",
          gate: "CONFIRM",
          reason: confirmReason,
          signalScore: score,
          requiredThreshold: baseThreshold,
          confirmPassed: false,
          contributions,
        }),
      );
      continue;
    }

    if (!passesAtrFeeGate(markPrice, signals.atr14, DISCOVERY_NOTIONAL_USD, atrFeeK)) {
      traceRows.push(
        createTraceRow({
          ...traceBase,
          traceId: stableTraceId(tickAt, strat.id),
          strategyId: strat.id,
          strategyName: strat.name,
          category: strat.category,
          side,
          status: "REJECTED",
          gate: "ATR_FEES",
          reason: `ATR/fee hurdle failed (atrPct=${atrPct.toFixed(4)})`,
          signalScore: score,
          requiredThreshold: baseThreshold,
          confirmPassed: true,
          feeHurdlePassed: false,
          contributions,
        }),
      );
      continue;
    }

    candidateCount += 1;
    traceRows.push(
      createTraceRow({
        ...traceBase,
        traceId: stableTraceId(tickAt, strat.id),
        strategyId: strat.id,
        strategyName: strat.name,
        category: strat.category,
        side,
        status: "CANDIDATE",
        gate: "OPENED",
        reason: "candidate",
        signalScore: score,
        requiredThreshold: baseThreshold,
        confirmPassed: true,
        feeHurdlePassed: true,
        openAttempted: true,
        contributions,
      }),
    );
  }

  const rows = capTraceRows(traceRows);
  return {
    tickAt,
    symbol,
    markPrice,
    regime,
    bars: args.bars.length,
    activeStrategies: activeStrats.length,
    evaluatedStrategies: evalCount,
    candidateCount,
    rows,
    summary: summarizeSignalTrace(rows),
    error: null,
  };
}
