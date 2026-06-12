"use client";

import { useEffect, useState } from "react";
import type { GenomeTier, MarketRegime, StrategyGenome } from "@/lib/strategyAuthority/portfolioTypes";
import { TIER_COLOR as AUTH_TIER_COLOR } from "@/lib/strategyAuthority/authorityScore";

const GENOME_TIER_CONFIG: Record<GenomeTier, { color: string; bg: string; bar: string }> = {
  ELITE: { color: "text-emerald-400", bg: "bg-emerald-900/40 border-emerald-700", bar: "bg-emerald-600" },
  STRONG: { color: "text-sky-400", bg: "bg-sky-900/30 border-sky-700", bar: "bg-sky-600" },
  ADEQUATE: { color: "text-blue-400", bg: "bg-blue-900/20 border-blue-800", bar: "bg-blue-600" },
  MARGINAL: { color: "text-amber-400", bg: "bg-amber-900/20 border-amber-800", bar: "bg-amber-700" },
  WEAK: { color: "text-rose-400", bg: "bg-rose-900/20 border-rose-900", bar: "bg-rose-700" },
};

const REGIME_SHORT: Partial<Record<MarketRegime, string>> = {
  TRENDING: "TREND",
  RANGING: "RANGE",
  HIGH_VOLATILITY_BREAKOUT: "HIGH-V",
  LOW_VOLATILITY_CHOP: "LOW-V",
};

function GenomeScoreRing({ score, tier }: { score: number; tier: GenomeTier }) {
  const cfg = GENOME_TIER_CONFIG[tier];
  return (
    <div className={`rounded border px-3 py-2 text-center ${cfg.bg}`}>
      <div className={`text-3xl font-black tabular-nums ${cfg.color}`}>{score}</div>
      <div className={`text-[9px] font-bold uppercase ${cfg.color}`}>{tier}</div>
    </div>
  );
}

function DimensionBar({ label, value, max, barColor }: { label: string; value: number; max: number; barColor: string }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  return (
    <div className="flex items-center gap-2">
      <span className="w-24 text-[9px] text-zinc-500">{label}</span>
      <div className="flex-1 h-1.5 rounded bg-zinc-800">
        <div className={`h-full rounded ${barColor}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="w-6 text-right text-[9px] tabular-nums text-zinc-400">{Math.round(value)}</span>
    </div>
  );
}

function GenomeCard({ genome }: { genome: StrategyGenome }) {
  const [expanded, setExpanded] = useState(false);
  const cfg = GENOME_TIER_CONFIG[genome.genome_tier];

  return (
    <div className={`rounded border ${cfg.bg} p-2.5 space-y-2 cursor-pointer`} onClick={() => setExpanded((e) => !e)}>
      {/* Header */}
      <div className="flex items-start gap-2">
        <div className="flex-1 min-w-0">
          <div className="text-xs font-semibold text-zinc-200 truncate">{genome.strategy_name}</div>
          <div className="text-[9px] text-zinc-600">{genome.family} · {genome.timeframe}</div>
        </div>
        <GenomeScoreRing score={genome.genome_score} tier={genome.genome_tier} />
      </div>

      {/* Score bars */}
      <div className="space-y-1">
        <DimensionBar label="Authority" value={genome.authority_score} max={100} barColor="bg-amber-600" />
        <DimensionBar label="Diversification" value={genome.diversification_score} max={100} barColor="bg-sky-600" />
        <DimensionBar label="Regime Strength" value={genome.regime_strength_score} max={100} barColor="bg-violet-600" />
        <DimensionBar label="Alloc Weight" value={Math.min(100, genome.allocation_weight * 4)} max={100} barColor="bg-emerald-600" />
      </div>

      {/* Status chips */}
      <div className="flex flex-wrap gap-1">
        <span className={`rounded px-1 py-0.5 text-[8px] font-bold border ${AUTH_TIER_COLOR[genome.authority_tier] ?? ""} border-zinc-800 bg-zinc-900/40`}>
          Auth T{genome.authority_tier}
        </span>
        {genome.allocation_weight > 0 && (
          <span className="rounded px-1 py-0.5 text-[8px] font-bold text-emerald-400 border border-emerald-800/50 bg-emerald-950/20">
            {genome.allocation_weight.toFixed(1)}%
          </span>
        )}
        {genome.candidate_eligible && (
          <span className="rounded px-1 py-0.5 text-[8px] font-bold text-emerald-300 border border-emerald-700 bg-emerald-900/40">
            ✓ QUEUE ELIGIBLE
          </span>
        )}
        {genome.best_regime && (
          <span className="rounded px-1 py-0.5 text-[8px] text-sky-400 border border-sky-800/40 bg-sky-950/20">
            Best: {REGIME_SHORT[genome.best_regime] ?? genome.best_regime}
          </span>
        )}
      </div>

      {/* Expanded detail */}
      {expanded && (
        <div className="border-t border-zinc-700/50 pt-2 space-y-2">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <div className="text-[8px] uppercase text-zinc-600 mb-1">Performance</div>
              {[
                { label: "Win Rate", value: `${genome.metrics.winRate.toFixed(1)}%` },
                { label: "Profit Factor", value: genome.metrics.profitFactor.toFixed(2) },
                { label: "Expectancy", value: `$${genome.metrics.expectancy.toFixed(2)}` },
                { label: "Sharpe", value: genome.metrics.sharpeRatio.toFixed(2) },
                { label: "Max DD", value: `${genome.metrics.maxDrawdown.toFixed(1)}%` },
                { label: "Trades", value: String(genome.metrics.closedTrades) },
              ].map((r) => (
                <div key={r.label} className="flex justify-between text-[9px]">
                  <span className="text-zinc-600">{r.label}</span>
                  <span className="text-zinc-300 tabular-nums">{r.value}</span>
                </div>
              ))}
            </div>
            <div>
              <div className="text-[8px] uppercase text-zinc-600 mb-1">Regime Breakdown</div>
              {Object.entries(genome.regime_metrics).length === 0 ? (
                <div className="text-[9px] text-zinc-700">No regime data</div>
              ) : (
                Object.entries(genome.regime_metrics).map(([regime, data]) => (
                  <div key={regime} className="flex justify-between text-[9px]">
                    <span className="text-zinc-600">{REGIME_SHORT[regime as MarketRegime] ?? regime}</span>
                    <span className={`tabular-nums ${(data?.pf ?? 0) >= 1 ? "text-emerald-400" : "text-rose-400"}`}>
                      PF {(data?.pf ?? 0).toFixed(2)} · {data?.trades}T
                    </span>
                  </div>
                ))
              )}
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <div className="text-[8px] uppercase text-zinc-600 mb-1">Lifecycle</div>
              {[
                { label: "Promotions", value: String(genome.promotion_count) },
                { label: "Demotions", value: String(genome.demotion_count) },
              ].map((r) => (
                <div key={r.label} className="flex justify-between text-[9px]">
                  <span className="text-zinc-600">{r.label}</span>
                  <span className="text-zinc-300 tabular-nums">{r.value}</span>
                </div>
              ))}
            </div>
            <div>
              <div className="text-[8px] uppercase text-zinc-600 mb-1">Portfolio</div>
              {[
                { label: "Evidence Score", value: `${genome.evidence_score.toFixed(1)}/15` },
                { label: "Correlation Penalty", value: `-${genome.correlation_penalty}` },
                { label: "Alloc Tier", value: genome.allocation_tier },
              ].map((r) => (
                <div key={r.label} className="flex justify-between text-[9px]">
                  <span className="text-zinc-600">{r.label}</span>
                  <span className="text-zinc-300 tabular-nums">{r.value}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export function StrategyGenome() {
  const [genomes, setGenomes] = useState<StrategyGenome[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [tierFilter, setTierFilter] = useState<GenomeTier | "ALL">("ALL");

  useEffect(() => {
    fetch("/api/strategy-authority/genome?limit=305")
      .then((r) => r.json())
      .then((d) => {
        if (d.ok) setGenomes(d.genomes);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  if (loading) return <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading strategy genomes…</div>;

  if (genomes.length === 0) {
    return <div className="py-8 text-center text-xs text-zinc-600">No genome data. Run Portfolio Intelligence Compute first.</div>;
  }

  const tierCounts = (["ELITE", "STRONG", "ADEQUATE", "MARGINAL", "WEAK"] as GenomeTier[]).map((t) => ({
    tier: t,
    count: genomes.filter((g) => g.genome_tier === t).length,
  }));

  const displayed = genomes.filter((g) => {
    const matchTier = tierFilter === "ALL" || g.genome_tier === tierFilter;
    const matchSearch = !search || g.strategy_name.toLowerCase().includes(search.toLowerCase()) || g.family.toLowerCase().includes(search.toLowerCase());
    return matchTier && matchSearch;
  });

  return (
    <div className="space-y-3">
      {/* Tier distribution */}
      <div className="flex gap-2 flex-wrap">
        <button
          onClick={() => setTierFilter("ALL")}
          className={`px-2 py-0.5 rounded border text-[9px] font-bold uppercase transition-colors ${tierFilter === "ALL" ? "border-zinc-600 bg-zinc-800 text-zinc-300" : "border-zinc-800 text-zinc-600 hover:text-zinc-400"}`}
        >
          ALL {genomes.length}
        </button>
        {tierCounts.map(({ tier, count }) => {
          const cfg = GENOME_TIER_CONFIG[tier];
          return (
            <button
              key={tier}
              onClick={() => setTierFilter(tier)}
              className={`px-2 py-0.5 rounded border text-[9px] font-bold uppercase transition-colors ${tierFilter === tier ? `${cfg.bg} ${cfg.color}` : "border-zinc-800 text-zinc-600 hover:text-zinc-400"}`}
            >
              {tier} {count}
            </button>
          );
        })}
        <input
          type="text"
          placeholder="Search…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="ml-auto rounded border border-zinc-700 bg-zinc-900 px-2 py-0.5 text-[10px] text-zinc-300 placeholder-zinc-600 focus:outline-none w-32"
        />
      </div>

      {/* Cards grid */}
      <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
        {displayed.slice(0, 30).map((g) => (
          <GenomeCard key={g.strategy_id} genome={g} />
        ))}
      </div>
      {displayed.length > 30 && (
        <div className="text-center text-[10px] text-zinc-600">
          Showing 30 of {displayed.length} genomes
        </div>
      )}
    </div>
  );
}
