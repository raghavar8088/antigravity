"use client";

import { useMemo, useState } from "react";
import type { MockTrade } from "@/lib/mockTradingEngine";
import {
  runMonteCarlo,
  analyseWorstStreak,
  type MonteCarloResult,
} from "@/lib/mockMonteCarloEngine";
import {
  DeskCard,
  DeskMetricTile,
  DeskSectionHeader,
  DeskChip,
  DeskButton,
} from "@/components/desk/ui";

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtUsd(v: number): string {
  if (!Number.isFinite(v)) return "—";
  const abs = Math.abs(v);
  if (abs >= 1_000_000) return `$${(v / 1_000_000).toFixed(2)}M`;
  if (abs >= 1_000) return `$${(v / 1_000).toFixed(1)}K`;
  return `$${v.toFixed(0)}`;
}

function fmtPct(v: number, places = 1): string {
  if (!Number.isFinite(v)) return "—";
  return `${(v * 100).toFixed(places)}%`;
}

// ── Fan chart (equity percentile bands, SVG) ──────────────────────────────────

function FanChart({
  percentilesByStep,
  startingEquity,
  height = 140,
}: {
  percentilesByStep: MonteCarloResult["equityPercentilesByStep"];
  startingEquity: number;
  height?: number;
}) {
  if (percentilesByStep.length < 2) return null;

  const allValues = percentilesByStep.flatMap((p) => [p.p5, p.p95]);
  const minV = Math.min(startingEquity, ...allValues);
  const maxV = Math.max(startingEquity, ...allValues);
  const range = maxV - minV || 1;
  const W = 300;
  const H = height;

  const toY = (v: number) => H - ((v - minV) / range) * H;
  const toX = (i: number) => (i / (percentilesByStep.length - 1)) * W;

  const buildPath = (getter: (p: (typeof percentilesByStep)[0]) => number) =>
    percentilesByStep.map((p, i) => `${i === 0 ? "M" : "L"}${toX(i).toFixed(1)},${toY(getter(p)).toFixed(1)}`).join(" ");

  const bandPath = (top: (p: (typeof percentilesByStep)[0]) => number, bot: (p: (typeof percentilesByStep)[0]) => number) => {
    const fwd = percentilesByStep.map((p, i) => `${toX(i).toFixed(1)},${toY(top(p)).toFixed(1)}`).join(" ");
    const bwd = [...percentilesByStep].reverse().map((p, i, arr) => `${toX(arr.length - 1 - i).toFixed(1)},${toY(bot(p)).toFixed(1)}`).join(" ");
    return `M ${fwd} L ${bwd} Z`;
  };

  const baselineY = toY(startingEquity);

  return (
    <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" className="w-full" style={{ height }}>
      {/* 90% band p5–p95 */}
      <path d={bandPath((p) => p.p95, (p) => p.p5)} fill="currentColor" className="text-blue-500/10" />
      {/* 50% band p25–p75 */}
      <path d={bandPath((p) => p.p75, (p) => p.p25)} fill="currentColor" className="text-blue-500/20" />
      {/* Baseline (starting equity) */}
      <line x1="0" y1={baselineY} x2={W} y2={baselineY} stroke="currentColor" strokeDasharray="4 2" strokeWidth="0.8" className="text-gray-500/40" />
      {/* Median line */}
      <path d={buildPath((p) => p.p50)} fill="none" stroke="currentColor" strokeWidth="1.5" className="text-blue-400" />
    </svg>
  );
}

// ── Risk gauge ────────────────────────────────────────────────────────────────

function RiskGauge({ riskOfRuin }: { riskOfRuin: number }) {
  const pct = Math.min(100, Math.max(0, riskOfRuin * 100));
  const color = pct < 5 ? "var(--desk-profit)" : pct < 20 ? "#f59e0b" : "var(--desk-loss)";
  const label = pct < 5 ? "LOW" : pct < 20 ? "MODERATE" : "HIGH";
  return (
    <div className="flex flex-col items-center gap-1">
      <div
        className="w-14 h-14 rounded-full flex items-center justify-center text-sm font-bold border-4"
        style={{ borderColor: color, color }}
      >
        {pct.toFixed(1)}%
      </div>
      <span className="text-[10px] font-medium" style={{ color }}>{label}</span>
      <span className="text-[10px] text-[var(--desk-muted)]">Risk of Ruin</span>
    </div>
  );
}

// ── Percentile row ────────────────────────────────────────────────────────────

function PercentileRow({ label, p }: { label: string; p: MonteCarloResult["finalEquityPercentiles"] }) {
  return (
    <div className="grid grid-cols-5 text-xs gap-1 text-center">
      <span className="col-span-1 text-left text-[var(--desk-muted)] truncate">{label}</span>
      {(["p5", "p25", "p50", "p75", "p95"] as const).map((k) => (
        <span key={k} className={`font-medium ${(p[k] ?? 0) >= 0 ? "text-[var(--desk-profit)]" : "text-[var(--desk-loss)]"}`}>
          {fmtUsd(p[k] ?? 0)}
        </span>
      ))}
    </div>
  );
}

// ── Main panel ────────────────────────────────────────────────────────────────

interface MockMonteCarloPanelProps {
  trades: readonly MockTrade[];
  startingEquityUsd: number;
  numSimulations?: number;
}

export function MockMonteCarloPanel({
  trades,
  startingEquityUsd,
  numSimulations = 1000,
}: MockMonteCarloPanelProps) {
  const [seed, setSeed] = useState<number>(42);

  const result = useMemo(
    () => runMonteCarlo(trades, { startingEquityUsd, numSimulations, seed }),
    [trades, startingEquityUsd, numSimulations, seed],
  );

  const worstStreak = useMemo(
    () => (result ? analyseWorstStreak(result, trades) : null),
    [result, trades],
  );

  if (!result) {
    return (
      <DeskCard>
        <DeskSectionHeader title="Monte Carlo Analysis" />
        <p className="text-xs text-[var(--desk-muted)] mt-4 text-center">
          Requires at least 10 closed trades to run simulations.
        </p>
      </DeskCard>
    );
  }

  const medianFinalEquity = result.finalEquityPercentiles.p50;
  const medianReturn = (medianFinalEquity - startingEquityUsd) / startingEquityUsd;

  return (
    <div className="flex flex-col gap-4">
      {/* KPIs row */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <DeskMetricTile
          label="Profit Probability"
          value={fmtPct(result.profitProbability)}
          sub={`${numSimulations.toLocaleString()} simulations`}
          subColor={result.profitProbability >= 0.6 ? "profit" : "loss"}
        />
        <DeskMetricTile
          label="Median Return"
          value={fmtPct(medianReturn)}
          sub={`P50 final equity: ${fmtUsd(medianFinalEquity)}`}
          subColor={medianReturn >= 0 ? "profit" : "loss"}
        />
        <DeskMetricTile
          label="Median P50 DD"
          value={fmtPct(result.drawdownPercentiles.p50)}
          sub={`P95 worst: ${fmtPct(result.drawdownPercentiles.p95)}`}
          subColor={result.drawdownPercentiles.p50 > 0.15 ? "loss" : "neutral"}
        />
        <DeskMetricTile
          label="Annlzd Return"
          value={fmtPct(result.medianAnnualizedReturn)}
          sub="Median path"
          subColor={result.medianAnnualizedReturn >= 0 ? "profit" : "loss"}
        />
      </div>

      {/* Fan chart */}
      <DeskCard>
        <div className="flex items-center justify-between">
          <DeskSectionHeader title="Equity Distribution (1000 Paths)" />
          <div className="flex items-center gap-2 text-[10px] text-[var(--desk-muted)]">
            <span className="inline-block w-3 h-1 bg-blue-400/60 rounded"></span>P5–P95
            <span className="inline-block w-3 h-1 bg-blue-400 rounded ml-1"></span>Median
          </div>
        </div>
        <div className="mt-2">
          <FanChart
            percentilesByStep={result.equityPercentilesByStep}
            startingEquity={startingEquityUsd}
          />
        </div>
      </DeskCard>

      {/* Risk of ruin + worst streak */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <DeskCard>
          <DeskSectionHeader title="Risk Analysis" />
          <div className="flex items-center gap-6 mt-3">
            <RiskGauge riskOfRuin={result.riskOfRuin} />
            <div className="text-xs text-[var(--desk-muted)] flex flex-col gap-1">
              <span>Ruin = equity &lt; {fmtPct(result.config.ruinThreshold)} starting</span>
              <span>Threshold: {fmtUsd(startingEquityUsd * result.config.ruinThreshold)}</span>
              <span className="mt-1 text-[var(--desk-foreground)]">
                {(result.riskOfRuin * 100).toFixed(1)}% of {numSimulations.toLocaleString()} paths ruined
              </span>
            </div>
          </div>
        </DeskCard>

        <DeskCard>
          <DeskSectionHeader title="Worst Streak Scenario" />
          {worstStreak ? (
            <div className="flex flex-col gap-2 mt-2 text-sm">
              <div className="flex justify-between">
                <span className="text-[var(--desk-muted)] text-xs">Consecutive losses</span>
                <strong>{worstStreak.worstStreak}</strong>
              </div>
              <div className="flex justify-between">
                <span className="text-[var(--desk-muted)] text-xs">Equity after streak</span>
                <strong className={worstStreak.equityAfterWorstStreak >= startingEquityUsd * 0.8 ? "text-[var(--desk-profit)]" : "text-[var(--desk-loss)]"}>
                  {fmtUsd(worstStreak.equityAfterWorstStreak)}
                </strong>
              </div>
              <div className="flex justify-between">
                <span className="text-[var(--desk-muted)] text-xs">Survival probability</span>
                <DeskChip
                  label={fmtPct(worstStreak.survivalProbability)}
                  variant={worstStreak.survivalProbability >= 0.8 ? "success" : worstStreak.survivalProbability >= 0.5 ? "warning" : "danger"}
                />
              </div>
            </div>
          ) : (
            <p className="text-xs text-[var(--desk-muted)] mt-2">Insufficient data.</p>
          )}
        </DeskCard>
      </div>

      {/* Percentile table */}
      <DeskCard>
        <DeskSectionHeader title="Equity Confidence Intervals" />
        <div className="mt-2 flex flex-col gap-2">
          <div className="grid grid-cols-5 text-[10px] font-medium text-[var(--desk-muted)] text-center">
            <span className="text-left">Metric</span>
            <span>P5</span><span>P25</span><span>P50</span><span>P75</span><span className="hidden">P95</span>
          </div>
          <PercentileRow label="Final Equity" p={result.finalEquityPercentiles} />
          <PercentileRow
            label="Max Drawdown"
            p={{
              p5: -result.drawdownPercentiles.p5,
              p25: -result.drawdownPercentiles.p25,
              p50: -result.drawdownPercentiles.p50,
              p75: -result.drawdownPercentiles.p75,
              p95: -result.drawdownPercentiles.p95,
            }}
          />
        </div>
        <p className="text-[10px] text-[var(--desk-muted)] mt-2">
          Based on {numSimulations.toLocaleString()} bootstrap-resampled paths from {trades.filter((t) => t.status === "CLOSED").length} historical trades.
        </p>
      </DeskCard>
    </div>
  );
}
