"use client";

import type { StrategyWithMetrics } from "@/lib/strategyAuthority/types";
import { TIER_COLOR, TIER_BG, computeAuthorityScore } from "@/lib/strategyAuthority/authorityScore";

const STATUS_LABEL: Record<string, string> = {
  GRADE_5: "G5", GRADE_4: "G4", GRADE_3: "G3",
  GRADE_2: "G2", GRADE_1: "G1", MAIN_ENGINE: "MAIN", RETIRED: "RET",
};
const STATUS_COLOR: Record<string, string> = {
  GRADE_5: "text-zinc-400", GRADE_4: "text-sky-400", GRADE_3: "text-blue-400",
  GRADE_2: "text-violet-400", GRADE_1: "text-amber-400", MAIN_ENGINE: "text-emerald-400", RETIRED: "text-rose-400",
};

function fmt(n: number, dec = 2) {
  return isNaN(n) ? "—" : n.toFixed(dec);
}

function GateChip({ label, value, pass }: { label: string; value: string; pass: boolean }) {
  return (
    <div className={`flex items-center justify-between rounded border px-1.5 py-0.5 ${pass ? "border-emerald-800/60 bg-emerald-950/30" : "border-zinc-800 bg-zinc-900/40"}`}>
      <span className="text-[9px] text-zinc-500">{label}</span>
      <span className={`ml-2 text-[9px] font-bold tabular-nums ${pass ? "text-emerald-400" : "text-zinc-400"}`}>
        {value} {pass ? "✓" : ""}
      </span>
    </div>
  );
}

export function PromotionProgressCard({ strategy }: { strategy: StrategyWithMetrics }) {
  const p = strategy.promotionProgress;
  const score = computeAuthorityScore(strategy.metrics, strategy.current_status as any);

  const gates = p ? [
    { label: "Trades", value: `${strategy.metrics.closedTrades}/${p.requirement.minClosedTrades}`, pass: strategy.metrics.closedTrades >= p.requirement.minClosedTrades },
    { label: "Win Rate", value: `${fmt(strategy.metrics.winRate, 1)}% / ${p.requirement.minWinRate}%`, pass: p.winRatePass },
    { label: "PF", value: `${fmt(strategy.metrics.profitFactor, 2)} / ${p.requirement.minProfitFactor}`, pass: p.profitFactorPass },
    { label: "Expectancy", value: fmt(strategy.metrics.expectancy, 2), pass: p.expectancyPass },
    { label: "Drawdown", value: `${fmt(strategy.metrics.maxDrawdown, 1)}%`, pass: p.drawdownPass },
    ...(p.requirement.minSharpe > -999 ? [{ label: "Sharpe", value: fmt(strategy.metrics.sharpeRatio, 2), pass: p.sharpePass }] : []),
  ] : [];

  return (
    <div className="rounded border border-zinc-800 bg-zinc-900/40 p-2.5 space-y-2">
      {/* Header */}
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="text-xs font-semibold text-zinc-200 truncate">{strategy.strategy_name}</div>
          <div className="mt-0.5 flex items-center gap-1.5">
            <span className={`text-[9px] font-bold uppercase ${STATUS_COLOR[strategy.current_status]}`}>
              {STATUS_LABEL[strategy.current_status]}
            </span>
            <span className="text-[9px] text-zinc-600">{strategy.family}</span>
            <span className="text-[9px] text-zinc-700">{strategy.timeframe}</span>
          </div>
        </div>
        {/* Authority score */}
        <div className={`flex-shrink-0 rounded border px-1.5 py-0.5 text-center ${TIER_BG[score.tier]}`}>
          <div className={`text-sm font-bold tabular-nums ${TIER_COLOR[score.tier]}`}>{score.total}</div>
          <div className={`text-[8px] font-bold uppercase ${TIER_COLOR[score.tier]}`}>Tier {score.tier}</div>
        </div>
      </div>

      {/* Status chips */}
      <div className="flex gap-1 flex-wrap">
        {strategy.promotionEligible && (
          <span className="rounded bg-emerald-900/40 px-1.5 py-0.5 text-[9px] font-bold text-emerald-400 border border-emerald-800/50">
            ↑ PROMOTION READY
          </span>
        )}
        {strategy.demotionRisk && (
          <span className="rounded bg-rose-900/30 px-1.5 py-0.5 text-[9px] font-bold text-rose-400 border border-rose-800/50">
            ↓ DEMOTION RISK
          </span>
        )}
        {strategy.retirementCandidate && !strategy.promotionEligible && (
          <span className="rounded bg-rose-950/50 px-1.5 py-0.5 text-[9px] font-bold text-rose-500 border border-rose-900">
            ⚠ RETIREMENT CANDIDATE
          </span>
        )}
      </div>

      {/* Promotion progress bar */}
      {p && (
        <div>
          <div className="flex items-center justify-between mb-1">
            <span className="text-[9px] text-zinc-500">Progress → {STATUS_LABEL[p.targetGrade]}</span>
            <span className={`text-[9px] font-bold tabular-nums ${p.overallProgress >= 100 ? "text-emerald-400" : "text-sky-400"}`}>
              {p.overallProgress}%
            </span>
          </div>
          <div className="h-1.5 rounded-full bg-zinc-800">
            <div
              className={`h-full rounded-full transition-all ${p.overallProgress >= 100 ? "bg-emerald-500" : "bg-sky-600"}`}
              style={{ width: `${Math.min(p.overallProgress, 100)}%` }}
            />
          </div>
        </div>
      )}

      {/* Gate checks */}
      {gates.length > 0 && (
        <div className="grid grid-cols-2 gap-1">
          {gates.map((g) => (
            <GateChip key={g.label} {...g} />
          ))}
        </div>
      )}

      {/* Score breakdown mini-bar */}
      <div className="rounded bg-zinc-950/60 p-1.5 grid grid-cols-4 gap-1 text-center">
        {[
          { label: "WR", val: score.winRateScore, max: 20 },
          { label: "PF", val: score.pfScore, max: 20 },
          { label: "E", val: score.expectancyScore, max: 15 },
          { label: "DD", val: score.drawdownScore, max: 15 },
        ].map((s) => (
          <div key={s.label}>
            <div className="text-[8px] text-zinc-600 uppercase">{s.label}</div>
            <div className="text-[9px] font-bold text-zinc-400 tabular-nums">{s.val}/{s.max}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Grid version for pipeline stage detail ────────────────────────────────────

export function PromotionProgressGrid({ strategies }: { strategies: StrategyWithMetrics[] }) {
  if (strategies.length === 0) {
    return <div className="py-6 text-center text-xs text-zinc-600">No strategy data for this stage</div>;
  }
  return (
    <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
      {strategies.slice(0, 12).map((s) => (
        <PromotionProgressCard key={s.strategy_id} strategy={s} />
      ))}
    </div>
  );
}
