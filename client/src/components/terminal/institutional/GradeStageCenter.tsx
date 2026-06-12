"use client";

import { useEffect, useState } from "react";
import type { StageSummary } from "@/lib/strategyAuthority/strategyAuthorityMongo";
import type { StrategyStatus, StrategyWithMetrics } from "@/lib/strategyAuthority/types";
import { PromotionTower } from "./PromotionTower";
import { MockStageTradingSuite } from "./MockStageTradingSuite";
import { TerminalCard, Metric } from "./TerminalCard";

const STATUS_LABEL: Record<StrategyStatus, string> = {
  GRADE_5: "Grade 5",
  GRADE_4: "Grade 4",
  GRADE_3: "Grade 3",
  GRADE_2: "Grade 2",
  GRADE_1: "Grade 1",
  MAIN_ENGINE: "Main Engine",
  RETIRED: "Retired",
};

const STATUS_SHORT: Record<StrategyStatus, string> = {
  GRADE_5: "G5", GRADE_4: "G4", GRADE_3: "G3",
  GRADE_2: "G2", GRADE_1: "G1", MAIN_ENGINE: "MAIN", RETIRED: "RET",
};

function fmt(n: number | undefined, dec = 2) {
  if (n == null || isNaN(n)) return "—";
  return n.toFixed(dec);
}

function ProgressBar({ value, max, pass }: { value: number; max: number; pass: boolean }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  return (
    <div className="flex items-center gap-1 min-w-[72px]">
      <div className="flex-1 h-1 rounded bg-zinc-800">
        <div
          className={`h-full rounded ${pass ? "bg-emerald-500" : "bg-sky-600"}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className={`text-[8px] tabular-nums w-6 text-right ${pass ? "text-emerald-400" : "text-zinc-500"}`}>
        {Math.round(pct)}%
      </span>
    </div>
  );
}

function PromotionProgressCell({ strategy }: { strategy: StrategyWithMetrics }) {
  const p = strategy.promotionProgress;
  if (!p) return <span className="text-[9px] text-zinc-600">—</span>;

  const gates = [
    {
      label: "Trades",
      value: strategy.metrics.closedTrades,
      max: p.requirement.minClosedTrades,
      pass: strategy.metrics.closedTrades >= p.requirement.minClosedTrades,
    },
    {
      label: "WR",
      value: strategy.metrics.winRate,
      max: p.requirement.minWinRate,
      pass: p.winRatePass,
    },
    {
      label: "PF",
      value: strategy.metrics.profitFactor,
      max: p.requirement.minProfitFactor,
      pass: p.profitFactorPass,
    },
    {
      label: "Sharpe",
      value: strategy.metrics.sharpeRatio,
      max: Math.max(p.requirement.minSharpe, 0.01),
      pass: p.sharpePass,
    },
    {
      label: "DD",
      value: Math.max(0, p.requirement.maxDrawdown - strategy.metrics.maxDrawdown),
      max: p.requirement.maxDrawdown || 1,
      pass: p.drawdownPass,
    },
  ];

  return (
    <div className="space-y-0.5 py-0.5">
      <div className="flex items-center justify-between gap-1">
        <span className="text-[8px] text-zinc-600">→ {STATUS_SHORT[p.targetGrade]}</span>
        <span className={`text-[8px] font-bold tabular-nums ${p.overallProgress >= 100 ? "text-emerald-400" : "text-sky-400"}`}>
          {p.overallProgress}%
        </span>
      </div>
      <div className="h-1 rounded-full bg-zinc-800">
        <div
          className={`h-full rounded-full ${p.overallProgress >= 100 ? "bg-emerald-500" : "bg-sky-600"}`}
          style={{ width: `${Math.min(p.overallProgress, 100)}%` }}
        />
      </div>
      <div className="grid grid-cols-1 gap-0.5">
        {gates.map((g) => (
          <div key={g.label} className="flex items-center gap-1">
            <span className="text-[7px] text-zinc-600 w-8">{g.label}</span>
            <ProgressBar value={g.value} max={g.max} pass={g.pass} />
          </div>
        ))}
      </div>
    </div>
  );
}

function statusChip(strategy: StrategyWithMetrics) {
  if (strategy.promotionEligible) {
    return <span className="text-[9px] font-bold text-emerald-400">↑ PROMOTE</span>;
  }
  if (strategy.demotionRisk) {
    return <span className="text-[9px] font-bold text-rose-400">↓ DEMOTE</span>;
  }
  if (strategy.retirementCandidate) {
    return <span className="text-[9px] font-bold text-rose-500">RETIRE</span>;
  }
  return <span className="text-[9px] text-zinc-500">ACTIVE</span>;
}

export function GradeStageCenter({ status }: { status: StrategyStatus }) {
  const [strategies, setStrategies] = useState<StrategyWithMetrics[]>([]);
  const [summary, setSummary] = useState<StageSummary | null>(null);
  const [tower, setTower] = useState<{ status: StrategyStatus; count: number }[]>([]);
  const [totalStrategies, setTotalStrategies] = useState(0);
  const [loading, setLoading] = useState(true);
  const [hasAuthority, setHasAuthority] = useState(false);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      fetch(`/api/strategy-authority/stage?status=${status}`).then((r) => r.json()),
      fetch("/api/strategy-authority/counts").then((r) => r.json()),
    ])
      .then(([stageData, countsData]) => {
        if (stageData.ok) {
          setStrategies(stageData.strategies);
          setSummary(stageData.summary);
          setHasAuthority(true);
        } else {
          setHasAuthority(false);
        }
        if (countsData.ok) {
          if (countsData.counts?.total != null) setTotalStrategies(countsData.counts.total);
          if (countsData.tower) {
            setTower(
              countsData.tower
                .filter((t: { status: StrategyStatus }) => t.status !== "RETIRED")
                .map((t: { status: StrategyStatus; count: number }) => ({
                  status: t.status,
                  count: t.count,
                }))
            );
          }
        }
        setLoading(false);
      })
      .catch(() => {
        setHasAuthority(false);
        setLoading(false);
      });
  }, [status]);

  const dash = "—";

  return (
    <div className="m3-page-stack">
      <MockStageTradingSuite status={status} />
      {tower.length > 0 && (
        <TerminalCard title="Strategy Pipeline" subtitle="Grade 5 → Main Engine institutional progression">
          <PromotionTower
            layers={tower.map((t) => ({
              status: t.status,
              count: t.count,
              promotionEventsTotal: 0,
              demotionEventsTotal: 0,
            }))}
            selectedStatus={status}
            totalStrategies={totalStrategies || undefined}
          />
        </TerminalCard>
      )}

      <div className="m3-kpi-strip">
        <Metric label="Total Strategies" value={hasAuthority && summary ? String(summary.totalStrategies) : dash} />
        <Metric
          label="Promotion Candidates"
          value={hasAuthority && summary ? String(summary.promotionCandidates) : dash}
          tone={summary && summary.promotionCandidates > 0 ? "positive" : "neutral"}
        />
        <Metric
          label="Demotion Candidates"
          value={hasAuthority && summary ? String(summary.demotionCandidates) : dash}
          tone={summary && summary.demotionCandidates > 0 ? "warning" : "neutral"}
        />
        <Metric
          label="Average PF"
          value={hasAuthority && summary ? fmt(summary.avgProfitFactor) : dash}
          tone={summary && summary.avgProfitFactor >= 1.2 ? "positive" : "warning"}
        />
        <Metric
          label="Average Sharpe"
          value={hasAuthority && summary ? fmt(summary.avgSharpe) : dash}
        />
        <Metric
          label="Average Expectancy"
          value={hasAuthority && summary ? fmt(summary.avgExpectancy) : dash}
        />
        <Metric
          label="Average Drawdown"
          value={hasAuthority && summary ? `${fmt(summary.avgDrawdown, 1)}%` : dash}
        />
      </div>

      <TerminalCard
        title={`${STATUS_LABEL[status]} Strategies`}
        subtitle="MongoDB authority — mock_trades metrics per strategy"
      >
        {loading ? (
          <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading stage strategies…</div>
        ) : !hasAuthority ? (
          <div className="py-8 text-center text-xs text-zinc-500">—</div>
        ) : strategies.length === 0 ? (
          <div className="py-8 text-center text-xs text-zinc-600">No strategies in this stage</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-[9px] uppercase tracking-wider text-zinc-500 border-b border-zinc-800">
                  <th className="py-2 px-2">Strategy ID</th>
                  <th className="py-2 px-2">Strategy Name</th>
                  <th className="py-2 px-2">Family</th>
                  <th className="py-2 px-2">Grade</th>
                  <th className="py-2 px-2 text-right">Win Rate</th>
                  <th className="py-2 px-2 text-right">Profit Factor</th>
                  <th className="py-2 px-2 text-right">Expectancy</th>
                  <th className="py-2 px-2 text-right">Sharpe</th>
                  <th className="py-2 px-2 text-right">Drawdown</th>
                  <th className="py-2 px-2 min-w-[140px]">Promotion Progress</th>
                  <th className="py-2 px-2 text-center">Current Status</th>
                </tr>
              </thead>
              <tbody>
                {strategies.map((s) => (
                  <tr key={s.strategy_id} className="border-t border-zinc-800 hover:bg-zinc-900/40 transition-colors">
                    <td className="py-1.5 px-2 text-[10px] text-zinc-600 font-mono truncate max-w-[120px]">{s.strategy_id}</td>
                    <td className="py-1.5 px-2 text-xs font-semibold text-zinc-200 max-w-[180px] truncate">{s.strategy_name}</td>
                    <td className="py-1.5 px-2 text-[10px] text-zinc-500">{s.family}</td>
                    <td className="py-1.5 px-2 text-[10px] font-bold text-zinc-400">{STATUS_SHORT[s.current_status]}</td>
                    <td className="py-1.5 px-2 tabular-nums text-xs text-right">{fmt(s.metrics.winRate, 1)}%</td>
                    <td className="py-1.5 px-2 tabular-nums text-xs text-right text-emerald-400">{fmt(s.metrics.profitFactor)}</td>
                    <td className="py-1.5 px-2 tabular-nums text-xs text-right">{fmt(s.metrics.expectancy)}</td>
                    <td className="py-1.5 px-2 tabular-nums text-xs text-right text-sky-400">{fmt(s.metrics.sharpeRatio)}</td>
                    <td className="py-1.5 px-2 tabular-nums text-xs text-right">{fmt(s.metrics.maxDrawdown, 1)}%</td>
                    <td className="py-1.5 px-2"><PromotionProgressCell strategy={s} /></td>
                    <td className="py-1.5 px-2 text-center">{statusChip(s)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </TerminalCard>
    </div>
  );
}
