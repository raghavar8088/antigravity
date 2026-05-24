/**
 * futuresParameterTuner.ts
 * Pure functions. No side effects. Fully testable.
 *
 * Given real closed-trade data, recommend ONE parameter change
 * at a time with before/after simulation notes.
 * Never changes live state — returns recommendations only.
 */

import { isProbeOrBootstrapTrade } from "./futuresSessionMetrics";
import type { PaperTradeDbRow } from "./paperTradesTypes";

export type TuneTarget =
  | "SIGNAL_THRESHOLD"
  | "TP_PCT"
  | "SL_PCT"
  | "SAME_SIDE_CAP"
  | "COOLDOWN_MIN"
  | "HOLD_MINUTES"
  | "NO_CHANGE";

export interface TuneRecommendation {
  target: TuneTarget;
  currentValue: number;
  suggestedValue: number;
  delta: number;
  confidence: "LOW" | "MED" | "HIGH";
  minTradesNeeded: number;
  tradesAnalyzed: number;
  rationale: string;
  beforeSim: SimNote;
  afterSim: SimNote;
  doNothing: string;
}

export interface SimNote {
  label: string;
  expectedWinRate: number;
  expectedExpectancy: number;
  expectedFeePct: number;
}

interface TradeStats {
  n: number;
  winRate: number;
  expectancy: number;
  feePctOfAbsGross: number;
  avgHoldMin: number;
  slRate: number;
  tpRate: number;
  timeRate: number;
  avgWin: number;
  avgLoss: number;
  profitFactor: number;
}

function calcStats(trades: PaperTradeDbRow[]): TradeStats {
  const prod = trades.filter(
    (t) => !isProbeOrBootstrapTrade({ strategy_name: t.strategy_name }),
  );
  const n = prod.length;
  if (n === 0) {
    return {
      n: 0,
      winRate: 0,
      expectancy: 0,
      feePctOfAbsGross: 0,
      avgHoldMin: 0,
      slRate: 0,
      tpRate: 0,
      timeRate: 0,
      avgWin: 0,
      avgLoss: 0,
      profitFactor: 0,
    };
  }

  const wins = prod.filter((t) => (t.net_pnl ?? 0) > 0);
  const losses = prod.filter((t) => (t.net_pnl ?? 0) <= 0);
  const sumNet = prod.reduce((s, t) => s + (t.net_pnl ?? 0), 0);
  const sumFee = prod.reduce((s, t) => s + (t.fees ?? 0), 0);
  const sumAbsGross = prod.reduce((s, t) => s + Math.abs(t.gross_pnl ?? 0), 0);
  const sumWins = wins.reduce((s, t) => s + (t.net_pnl ?? 0), 0);
  const sumLosses = Math.abs(losses.reduce((s, t) => s + (t.net_pnl ?? 0), 0));
  const holdList = prod.map((t) =>
    t.closed_at && t.opened_at
      ? (new Date(t.closed_at).getTime() - new Date(t.opened_at).getTime()) / 60_000
      : 0,
  );

  return {
    n,
    winRate: wins.length / n,
    expectancy: sumNet / n,
    feePctOfAbsGross: sumAbsGross > 0 ? sumFee / sumAbsGross : 0,
    avgHoldMin: holdList.reduce((a, b) => a + b, 0) / n,
    slRate: prod.filter((t) => t.exit_reason === "SL").length / n,
    tpRate: prod.filter((t) => t.exit_reason === "TP").length / n,
    timeRate: prod.filter((t) => t.exit_reason === "TIME").length / n,
    avgWin: wins.length ? sumWins / wins.length : 0,
    avgLoss: losses.length ? sumLosses / losses.length : 0,
    profitFactor: sumLosses > 0 ? sumWins / sumLosses : sumWins > 0 ? Infinity : 0,
  };
}

/**
 * Primary entry point.
 * Analyzes last windowN production trades and returns
 * the single highest-priority parameter change.
 */
export function recommendOneTune(
  trades: PaperTradeDbRow[],
  currentThreshold: number,
  currentTpPct: number,
  currentSlPct: number,
  currentSameSide: number,
  windowN = 50,
): TuneRecommendation {
  const prod = trades
    .filter((t) => !isProbeOrBootstrapTrade({ strategy_name: t.strategy_name }) && t.closed_at)
    .sort(
      (a, b) =>
        new Date(b.closed_at!).getTime() - new Date(a.closed_at!).getTime(),
    )
    .slice(0, windowN);

  const stats = calcStats(prod);

  if (stats.n < 10) {
    return noChange(
      stats.n,
      10,
      "Fewer than 10 production trades. Collect more data before tuning.",
    );
  }

  if (stats.feePctOfAbsGross > 0.6) {
    const delta = 4;
    const newThresh = currentThreshold + delta;
    return {
      target: "SIGNAL_THRESHOLD",
      currentValue: currentThreshold,
      suggestedValue: newThresh,
      delta,
      confidence: stats.n >= 30 ? "HIGH" : "MED",
      minTradesNeeded: 10,
      tradesAnalyzed: stats.n,
      rationale:
        `fee/|gross| = ${(stats.feePctOfAbsGross * 100).toFixed(1)}% ` +
        `(target <50%). Trades closing after tiny moves. ` +
        `Raising threshold from ${currentThreshold} → ${newThresh} ` +
        `will reduce entry frequency, selecting only higher-conviction ` +
        `signals that sustain larger moves.`,
      beforeSim: simFromStats(stats, "Current (before)"),
      afterSim: {
        label: `After threshold +${delta}`,
        expectedWinRate: Math.min(stats.winRate * 1.15, 0.65),
        expectedExpectancy: stats.expectancy * 0.5 + 2,
        expectedFeePct: stats.feePctOfAbsGross * 0.7,
      },
      doNothing:
        "Fees will continue exceeding gross PnL. Profit factor stays near 0 indefinitely.",
    };
  }

  if (stats.slRate > 0.7 && stats.expectancy < 0) {
    if (stats.avgHoldMin < 5) {
      const delta = 3;
      const newThresh = currentThreshold + delta;
      return {
        target: "SIGNAL_THRESHOLD",
        currentValue: currentThreshold,
        suggestedValue: newThresh,
        delta,
        confidence: "HIGH",
        minTradesNeeded: 10,
        tradesAnalyzed: stats.n,
        rationale:
          `SL rate ${(stats.slRate * 100).toFixed(0)}% with avg hold ` +
          `${stats.avgHoldMin.toFixed(1)}m. Entries reversing too fast ` +
          `— signal quality is low. Raise threshold ${currentThreshold} → ` +
          `${newThresh} to require stronger confirmation before entry.`,
        beforeSim: simFromStats(stats, "Current (before)"),
        afterSim: {
          label: `After threshold +${delta}`,
          expectedWinRate: Math.min(stats.winRate * 1.2, 0.55),
          expectedExpectancy: stats.expectancy + Math.abs(stats.avgLoss) * 0.3,
          expectedFeePct: stats.feePctOfAbsGross * 0.75,
        },
        doNothing:
          "SL-dominant exits will continue. Expectancy stays negative. Edge never recovers.",
      };
    }

    const delta = parseFloat((currentSlPct * 0.3).toFixed(4));
    const newSlPct = parseFloat((currentSlPct + delta).toFixed(4));
    return {
      target: "SL_PCT",
      currentValue: currentSlPct,
      suggestedValue: newSlPct,
      delta,
      confidence: stats.n >= 20 ? "HIGH" : "MED",
      minTradesNeeded: 10,
      tradesAnalyzed: stats.n,
      rationale:
        `SL rate ${(stats.slRate * 100).toFixed(0)}% with avg hold ` +
        `${stats.avgHoldMin.toFixed(1)}m. Price moves but hits SL before ` +
        `TP. Widening SL from ${currentSlPct} → ${newSlPct} gives trades ` +
        `room to breathe while TP target is unchanged.`,
      beforeSim: simFromStats(stats, "Current (before)"),
      afterSim: {
        label: `After SL widen +${(delta * 100).toFixed(2)}%`,
        expectedWinRate: Math.min(stats.winRate * 1.25, 0.55),
        expectedExpectancy: stats.expectancy + Math.abs(stats.avgLoss) * 0.2,
        expectedFeePct: stats.feePctOfAbsGross,
      },
      doNothing:
        "Trades keep stopping out before the move completes. " +
        "Win rate stays suppressed despite correct direction calls.",
    };
  }

  if (stats.tpRate < 0.05 && stats.n >= 20) {
    const delta = parseFloat((currentTpPct * 0.25).toFixed(4));
    const newTpPct = parseFloat((currentTpPct - delta).toFixed(4));
    return {
      target: "TP_PCT",
      currentValue: currentTpPct,
      suggestedValue: newTpPct,
      delta: -delta,
      confidence: "MED",
      minTradesNeeded: 20,
      tradesAnalyzed: stats.n,
      rationale:
        `TP hit rate ${(stats.tpRate * 100).toFixed(1)}% in ${stats.n} trades. ` +
        `Price never reaching TP. Tighten TP from ${currentTpPct} → ` +
        `${newTpPct} to capture smaller but more frequent wins. ` +
        `Current avg hold ${stats.avgHoldMin.toFixed(1)}m suggests ` +
        `momentum fades before TP.`,
      beforeSim: simFromStats(stats, "Current (before)"),
      afterSim: {
        label: `After TP tighten -${(delta * 100).toFixed(2)}%`,
        expectedWinRate: Math.min(stats.winRate * 1.4, 0.5),
        expectedExpectancy: stats.expectancy * 0.8 + 3,
        expectedFeePct: stats.feePctOfAbsGross * 0.9,
      },
      doNothing:
        "TP hit rate stays near 0%. Profit factor stays at 0. No wins recorded.",
    };
  }

  if (currentSameSide >= 2 && stats.slRate > 0.8 && stats.n >= 15) {
    return {
      target: "SAME_SIDE_CAP",
      currentValue: currentSameSide,
      suggestedValue: 1,
      delta: -1,
      confidence: "MED",
      minTradesNeeded: 15,
      tradesAnalyzed: stats.n,
      rationale:
        `SL rate ${(stats.slRate * 100).toFixed(0)}% with same-side cap ` +
        `${currentSameSide}. Correlated positions likely hitting SL ` +
        `simultaneously, amplifying drawdown. Reduce cap from ` +
        `${currentSameSide} → 1 to limit correlated exposure.`,
      beforeSim: simFromStats(stats, "Current (before)"),
      afterSim: {
        label: "After same-side cap → 1",
        expectedWinRate: stats.winRate,
        expectedExpectancy: stats.expectancy * 1.1,
        expectedFeePct: stats.feePctOfAbsGross,
      },
      doNothing:
        "Correlated losses continue to cluster. Max drawdown risk stays elevated.",
    };
  }

  return noChange(
    stats.n,
    10,
    `Desk metrics within acceptable range. ` +
      `Expectancy=$${stats.expectancy.toFixed(2)} ` +
      `WinRate=${(stats.winRate * 100).toFixed(1)}% ` +
      `fee/gross=${(stats.feePctOfAbsGross * 100).toFixed(1)}%. ` +
      `No parameter change recommended at this time.`,
  );
}

function simFromStats(stats: TradeStats, label: string): SimNote {
  return {
    label,
    expectedWinRate: stats.winRate,
    expectedExpectancy: stats.expectancy,
    expectedFeePct: stats.feePctOfAbsGross,
  };
}

function noChange(n: number, needed: number, rationale: string): TuneRecommendation {
  return {
    target: "NO_CHANGE",
    currentValue: 0,
    suggestedValue: 0,
    delta: 0,
    confidence: n >= needed ? "HIGH" : "LOW",
    minTradesNeeded: needed,
    tradesAnalyzed: n,
    rationale,
    beforeSim: {
      label: "N/A",
      expectedWinRate: 0,
      expectedExpectancy: 0,
      expectedFeePct: 0,
    },
    afterSim: {
      label: "N/A",
      expectedWinRate: 0,
      expectedExpectancy: 0,
      expectedFeePct: 0,
    },
    doNothing: "No action required.",
  };
}
