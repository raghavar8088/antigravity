"use client";

import type { ReactNode } from "react";
import { DeskCard } from "@/components/desk/ui/DeskCard";
import { DeskChip } from "@/components/desk/ui/DeskChip";
import { DeskMetricTile } from "@/components/desk/ui/DeskMetricTile";
import { DeskSectionHeader } from "@/components/desk/ui/DeskSectionHeader";

type Metrics = {
  totalTrades: number;
  winRate: number;
  netPnl: number;
  profitFactor: number;
  expectancy: number;
  sharpe: number;
  sortino: number;
  calmar: number;
  var95: number;
  cvar95: number;
  maxDrawdown: number;
  riskOfRuin: number;
};

type ExecutionQuality = {
  averageSpreadCost: number;
  averageSlippageCost: number;
  averageImpactCost: number;
  averageFillRatio: number;
  averageLatencyMs: number;
  fundingPnl: number;
};

type MonteCarloDistribution = {
  survivalRate: number;
  percentile5: number;
  median: number;
  percentile95: number;
  worstCase: number;
  bestCase: number;
};

type BenchmarkResult = {
  name: string;
  returnPct: number;
  alpha: number;
  informationRatio: number;
  trackingError: number;
};

export type InstitutionalBacktestDashboardData = {
  backtestSummary: Metrics;
  executionQuality: ExecutionQuality;
  monteCarloDistribution: MonteCarloDistribution;
  benchmarkComparison: { results: BenchmarkResult[] };
  robustness: { robustnessScore: number; profitable: boolean };
  oosPerformance: { positiveOOSPF: boolean; positiveOOSExpectancy: boolean; averageDegradation: number };
  regimePerformance: Record<string, Metrics>;
  portfolio: {
    portfolioReturn: number;
    portfolioSharpe: number;
    portfolioDrawdown: number;
    portfolioVaR: number;
    portfolioCVaR: number;
    portfolioHeatPct: number;
  };
};

type Props = {
  data: InstitutionalBacktestDashboardData;
};

function fmtUsd(value: number) {
  const abs = Math.abs(value).toLocaleString("en-US", { maximumFractionDigits: 2, minimumFractionDigits: 2 });
  return `${value >= 0 ? "+" : "-"}$${abs}`;
}

function fmtPct(value: number) {
  return `${value.toFixed(2)}%`;
}

function fmtRatio(value: number) {
  return value === 0 ? "0.00" : value.toFixed(2);
}

export default function InstitutionalBacktestDashboard({ data }: Props) {
  const summary = data.backtestSummary;
  const execution = data.executionQuality;
  const benchmarkAlphaPositive = data.benchmarkComparison.results.some((result) => result.alpha > 0);
  const regimeRows = Object.entries(data.regimePerformance).slice(0, 6);

  return (
    <DeskCard>
      <DeskSectionHeader
        title="Institutional Backtest"
        subtitle="Execution realism, OOS validation, Monte Carlo, regimes, and benchmark attribution"
        actions={
          <>
            <DeskChip tone={data.robustness.profitable ? "success" : "error"}>
              Robustness {fmtPct(data.robustness.robustnessScore)}
            </DeskChip>
            <DeskChip tone={benchmarkAlphaPositive ? "success" : "warning"}>Benchmark Alpha</DeskChip>
          </>
        }
      />

      <div className="grid gap-3 md:grid-cols-4">
        <DeskMetricTile
          label="Net PnL"
          value={fmtUsd(summary.netPnl)}
          valueClassName={summary.netPnl >= 0 ? "desk-pnl-positive" : "desk-pnl-negative"}
        />
        <DeskMetricTile label="Profit Factor" value={fmtRatio(summary.profitFactor)} />
        <DeskMetricTile label="Sharpe / Sortino" value={`${fmtRatio(summary.sharpe)} / ${fmtRatio(summary.sortino)}`} />
        <DeskMetricTile label="Max DD" value={fmtUsd(-summary.maxDrawdown)} valueClassName="desk-pnl-negative" />
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-3">
        <Panel title="Execution Quality">
          <Metric label="Avg fill" value={fmtPct(execution.averageFillRatio * 100)} />
          <Metric label="Spread cost" value={fmtUsd(-execution.averageSpreadCost)} />
          <Metric label="Slippage cost" value={fmtUsd(-execution.averageSlippageCost)} />
          <Metric label="Market impact" value={fmtUsd(-execution.averageImpactCost)} />
          <Metric label="Latency" value={`${execution.averageLatencyMs.toFixed(0)}ms`} />
          <Metric label="Funding attribution" value={fmtUsd(execution.fundingPnl)} />
        </Panel>

        <Panel title="Monte Carlo">
          <Metric label="Survival" value={fmtPct(data.monteCarloDistribution.survivalRate)} />
          <Metric label="Worst case" value={fmtUsd(data.monteCarloDistribution.worstCase)} />
          <Metric label="P5 / Median / P95" value={`${fmtUsd(data.monteCarloDistribution.percentile5)} / ${fmtUsd(data.monteCarloDistribution.median)} / ${fmtUsd(data.monteCarloDistribution.percentile95)}`} />
          <Metric label="Best case" value={fmtUsd(data.monteCarloDistribution.bestCase)} />
        </Panel>

        <Panel title="Risk & Portfolio">
          <Metric label="VaR / CVaR" value={`${fmtUsd(-summary.var95)} / ${fmtUsd(-summary.cvar95)}`} />
          <Metric label="Risk of ruin" value={fmtPct(summary.riskOfRuin)} />
          <Metric label="Portfolio return" value={fmtPct(data.portfolio.portfolioReturn)} />
          <Metric label="Portfolio heat" value={fmtPct(data.portfolio.portfolioHeatPct)} />
          <Metric label="Portfolio Sharpe" value={fmtRatio(data.portfolio.portfolioSharpe)} />
        </Panel>
      </div>

      <div className="mt-4 grid gap-3 lg:grid-cols-2">
        <Panel title="Out Of Sample">
          <Metric label="OOS PF" value={data.oosPerformance.positiveOOSPF ? "Positive" : "Failed"} />
          <Metric label="OOS expectancy" value={data.oosPerformance.positiveOOSExpectancy ? "Positive" : "Failed"} />
          <Metric label="Degradation" value={fmtPct(data.oosPerformance.averageDegradation)} />
        </Panel>

        <Panel title="Benchmark Comparison">
          {data.benchmarkComparison.results.slice(0, 4).map((result) => (
            <Metric
              key={result.name}
              label={result.name}
              value={`alpha ${fmtPct(result.alpha)} · IR ${fmtRatio(result.informationRatio)} · TE ${fmtRatio(result.trackingError)}`}
            />
          ))}
        </Panel>
      </div>

      {regimeRows.length > 0 ? (
        <div className="mt-4">
          <Panel title="Regime Performance">
            {regimeRows.map(([regime, metrics]) => (
              <Metric
                key={regime}
                label={regime}
                value={`${metrics.totalTrades} trades · PF ${fmtRatio(metrics.profitFactor)} · exp ${fmtUsd(metrics.expectancy)} · DD ${fmtUsd(-metrics.maxDrawdown)}`}
              />
            ))}
          </Panel>
        </div>
      ) : null}
    </DeskCard>
  );
}

function Panel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="rounded-[16px] border border-zinc-200 bg-zinc-50/80 p-4">
      <h3 className="mb-3 text-sm font-semibold text-zinc-900">{title}</h3>
      <div className="space-y-2">{children}</div>
    </section>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 text-xs">
      <span className="text-zinc-500">{label}</span>
      <span className="text-right font-mono font-medium text-zinc-900">{value}</span>
    </div>
  );
}
