"use client";

import { useEffect, useMemo, useState } from "react";
import type { StrategyWithMetrics } from "@/lib/strategyAuthority/types";
import type { PortfolioAllocationSummary } from "@/lib/strategyAuthority/portfolioTypes";
import { computeAuthorityScore } from "@/lib/strategyAuthority/authorityScore";
import { AllocationView } from "./AllocationView";
import { CorrelationMatrix } from "./CorrelationMatrix";
import { FamilyLeaderboard } from "./FamilyLeaderboard";
import { MainEngineSurvivors } from "./MainEngineSurvivors";
import { PromotionTower } from "./PromotionTower";
import { MockStageTradingSuite } from "./MockStageTradingSuite";
import { RegimeIntelligence } from "./RegimeIntelligence";
import { TerminalCard, Metric } from "./TerminalCard";
import { SkeletonBlock } from "@/components/ui/EmptyState";
import type { StrategyStatus } from "@/lib/strategyAuthority/types";

type SortKey = "authority" | "pf" | "sharpe" | "drawdown" | "allocation";

function fmt(n: number | undefined, dec = 2) {
  if (n == null || isNaN(n)) return "—";
  return n.toFixed(dec);
}

function EngineRosterSkeleton() {
  return (
    <div className="overflow-x-auto" aria-busy="true" aria-label="Loading engine roster">
      <table className="w-full text-xs">
        <thead>
          <tr className="text-left text-[9px] uppercase tracking-wider text-zinc-500 border-b border-zinc-800">
            <th className="py-2 px-2">#</th>
            <th className="py-2 px-2">Strategy</th>
            <th className="py-2 px-2">Family</th>
            <th className="py-2 px-2 text-right">Authority</th>
            <th className="py-2 px-2 text-right">PF</th>
            <th className="py-2 px-2 text-right">Sharpe</th>
            <th className="py-2 px-2 text-right">Drawdown</th>
            <th className="py-2 px-2 text-right">Allocation</th>
          </tr>
        </thead>
        <tbody>
          {Array.from({ length: 5 }).map((_, rowIndex) => (
            <tr key={rowIndex} className="border-t border-zinc-800">
              {Array.from({ length: 8 }).map((__, cellIndex) => (
                <td key={cellIndex} className={`py-2 px-2 ${cellIndex >= 3 ? "text-right" : ""}`}>
                  <SkeletonBlock width={cellIndex === 1 ? 164 : cellIndex === 2 ? 96 : 54} height={12} rounded={3} />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function MockEngineCenter() {
  const [strategies, setStrategies] = useState<StrategyWithMetrics[]>([]);
  const [allocation, setAllocation] = useState<PortfolioAllocationSummary | null>(null);
  const [tower, setTower] = useState<{ status: StrategyStatus; count: number }[]>([]);
  const [totalStrategies, setTotalStrategies] = useState(0);
  const [loading, setLoading] = useState(true);
  const [hasAuthority, setHasAuthority] = useState(false);
  const [sortKey, setSortKey] = useState<SortKey>("authority");

  useEffect(() => {
    Promise.all([
      fetch("/api/strategy-authority/stage?status=MAIN_ENGINE").then((r) => r.json()),
      fetch("/api/strategy-authority/allocation").then((r) => r.json()),
      fetch("/api/strategy-authority/counts").then((r) => r.json()),
    ])
      .then(([stageData, allocData, countsData]) => {
        if (stageData.ok) {
          setStrategies(stageData.strategies);
          setHasAuthority(true);
        }
        if (allocData.ok) setAllocation(allocData.allocation);
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
      .catch(() => setLoading(false));
  }, []);

  const allocationMap = useMemo(() => {
    const m = new Map<string, number>();
    if (allocation) {
      for (const s of allocation.strategies) {
        m.set(s.strategy_name, s.allocation_weight);
      }
    }
    return m;
  }, [allocation]);

  const ranked = useMemo(() => {
    return [...strategies].sort((a, b) => {
      const scoreA = computeAuthorityScore(a.metrics, "MAIN_ENGINE");
      const scoreB = computeAuthorityScore(b.metrics, "MAIN_ENGINE");
      const allocA = allocationMap.get(a.strategy_name) ?? 0;
      const allocB = allocationMap.get(b.strategy_name) ?? 0;
      switch (sortKey) {
        case "pf":
          return b.metrics.profitFactor - a.metrics.profitFactor;
        case "sharpe":
          return b.metrics.sharpeRatio - a.metrics.sharpeRatio;
        case "drawdown":
          return a.metrics.maxDrawdown - b.metrics.maxDrawdown;
        case "allocation":
          return allocB - allocA;
        default:
          return scoreB.total - scoreA.total;
      }
    });
  }, [strategies, sortKey, allocationMap]);

  const dash = "—";
  const population = hasAuthority ? String(strategies.length) : dash;

  return (
    <div className="m3-page-stack">
      <MockStageTradingSuite status="MAIN_ENGINE" />
      {tower.length > 0 && (
        <TerminalCard title="Institutional Pipeline" subtitle="Grade 5 ↓ Grade 4 ↓ Grade 3 ↓ Grade 2 ↓ Grade 1 ↓ Mock Trading Engine">
          <PromotionTower
            layers={tower.map((t) => ({
              status: t.status,
              count: t.count,
              promotionEventsTotal: 0,
              demotionEventsTotal: 0,
            }))}
            selectedStatus="MAIN_ENGINE"
            totalStrategies={totalStrategies || undefined}
          />
        </TerminalCard>
      )}

      <div className="m3-kpi-strip">
        <Metric label="Engine Population" value={population} tone={strategies.length > 0 ? "positive" : "neutral"} />
        <Metric
          label="Portfolio Strategies"
          value={allocation ? String(allocation.strategies.length) : dash}
        />
        <Metric
          label="Allocated Weight"
          value={
            allocation
              ? `${allocation.strategies.reduce((a, s) => a + s.allocation_weight, 0).toFixed(1)}%`
              : dash
          }
        />
        <Metric
          label="Avg Authority Score"
          value={
            strategies.length
              ? String(
                  Math.round(
                    strategies.reduce(
                      (a, s) => a + computeAuthorityScore(s.metrics, "MAIN_ENGINE").total,
                      0
                    ) / strategies.length
                  )
                )
              : dash
          }
        />
      </div>

      <div className="grid gap-3 lg:grid-cols-2">
        <TerminalCard title="Family Distribution" subtitle="Strategy families in main engine roster">
          <FamilyLeaderboard />
        </TerminalCard>
        <TerminalCard title="Regime Distribution" subtitle="Market regime performance intelligence">
          <RegimeIntelligence />
        </TerminalCard>
      </div>

      <TerminalCard title="Portfolio Allocation" subtitle="Authority-weighted capital distribution">
        <AllocationView />
      </TerminalCard>

      <TerminalCard title="Correlation Heatmap" subtitle="Cross-strategy Pearson correlation matrix">
        <CorrelationMatrix />
      </TerminalCard>

      <TerminalCard title="Main Engine Survivors" subtitle="Elite strategies — full authority score breakdown">
        <MainEngineSurvivors />
      </TerminalCard>

      <TerminalCard
        title="Engine Roster — Ranked"
        subtitle="Sort by authority score, PF, Sharpe, drawdown, or allocation weight"
        actions={
          <div className="flex gap-1 flex-wrap">
            {(
              [
                ["authority", "Authority"],
                ["pf", "PF"],
                ["sharpe", "Sharpe"],
                ["drawdown", "Drawdown"],
                ["allocation", "Allocation"],
              ] as const
            ).map(([key, label]) => (
              <button
                key={key}
                type="button"
                onClick={() => setSortKey(key)}
                className={`rounded border px-2 py-0.5 text-[9px] font-bold uppercase transition-colors ${
                  sortKey === key
                    ? "border-emerald-700 bg-emerald-950/50 text-emerald-400"
                    : "border-zinc-800 text-zinc-500 hover:text-zinc-300"
                }`}
              >
                {label}
              </button>
            ))}
          </div>
        }
      >
        {loading ? (
          <EngineRosterSkeleton />
        ) : !hasAuthority ? (
          <div className="py-8 text-center text-xs text-zinc-500">—</div>
        ) : ranked.length === 0 ? (
          <div className="py-8 text-center text-xs text-zinc-600">No strategies in main engine</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-[9px] uppercase tracking-wider text-zinc-500 border-b border-zinc-800">
                  <th className="py-2 px-2">#</th>
                  <th className="py-2 px-2">Strategy</th>
                  <th className="py-2 px-2">Family</th>
                  <th className="py-2 px-2 text-right">Authority</th>
                  <th className="py-2 px-2 text-right">PF</th>
                  <th className="py-2 px-2 text-right">Sharpe</th>
                  <th className="py-2 px-2 text-right">Drawdown</th>
                  <th className="py-2 px-2 text-right">Allocation</th>
                </tr>
              </thead>
              <tbody>
                {ranked.map((s, i) => {
                  const score = computeAuthorityScore(s.metrics, "MAIN_ENGINE");
                  const weight = allocationMap.get(s.strategy_name);
                  return (
                    <tr key={s.strategy_id} className="border-t border-zinc-800 hover:bg-zinc-900/40">
                      <td className="py-1.5 px-2 tabular-nums text-zinc-600">{i + 1}</td>
                      <td className="py-1.5 px-2 font-semibold text-emerald-300 truncate max-w-[200px]">{s.strategy_name}</td>
                      <td className="py-1.5 px-2 text-[10px] text-zinc-500">{s.family}</td>
                      <td className="py-1.5 px-2 tabular-nums text-right font-bold text-amber-400">{score.total}</td>
                      <td className="py-1.5 px-2 tabular-nums text-right text-emerald-400">{fmt(s.metrics.profitFactor)}</td>
                      <td className="py-1.5 px-2 tabular-nums text-right text-sky-400">{fmt(s.metrics.sharpeRatio)}</td>
                      <td className="py-1.5 px-2 tabular-nums text-right">{fmt(s.metrics.maxDrawdown, 1)}%</td>
                      <td className="py-1.5 px-2 tabular-nums text-right">
                        {weight != null ? `${weight.toFixed(1)}%` : "—"}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </TerminalCard>
    </div>
  );
}
