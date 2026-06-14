"use client";

import { useEffect, useState } from "react";
import type { StrategyWithMetrics } from "@/lib/strategyAuthority/types";
import { computeAuthorityScore, TIER_COLOR, TIER_BG } from "@/lib/strategyAuthority/authorityScore";
import { TerminalCard, Metric } from "./TerminalCard";

function fmt(n: number | undefined, dec = 2) {
  if (n == null || isNaN(n)) return "—";
  return n.toFixed(dec);
}

function AuthorityBadge({ score, tier }: { score: number; tier: string }) {
  return (
    <div className={`inline-flex flex-col items-center rounded border px-1.5 py-0.5 ${TIER_BG[tier]}`}>
      <span className={`text-[10px] font-bold tabular-nums leading-none ${TIER_COLOR[tier]}`}>{score}</span>
      <span className={`text-[7px] font-bold uppercase ${TIER_COLOR[tier]}`}>T{tier}</span>
    </div>
  );
}

function MiniGauge({ value, max, label, colorClass }: {
  value: number; max: number; label: string; colorClass: string;
}) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  return (
    <div className="flex flex-col gap-0.5">
      <div className="flex justify-between text-[8px]">
        <span className="text-zinc-600 uppercase">{label}</span>
        <span className="tabular-nums text-zinc-400">{value.toFixed(1)}</span>
      </div>
      <div className="h-1 rounded bg-zinc-800">
        <div className={`h-full rounded ${colorClass}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

export function MainEngineSurvivors() {
  const [strategies, setStrategies] = useState<StrategyWithMetrics[]>([]);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/strategy-authority/main-engine")
      .then((r) => r.json())
      .then((d) => {
        if (d.ok) setStrategies(d.strategies);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  const totalPnl = strategies.reduce((a, s) => a + s.metrics.totalPnl, 0);
  const avgPF = strategies.length
    ? strategies.reduce((a, s) => a + s.metrics.profitFactor, 0) / strategies.length
    : 0;
  const avgWR = strategies.length
    ? strategies.reduce((a, s) => a + s.metrics.winRate, 0) / strategies.length
    : 0;

  const scores = strategies.map((s) => computeAuthorityScore(s.metrics, "MAIN_ENGINE"));
  const avgAuthority = scores.length
    ? Math.round(scores.reduce((a, s) => a + s.total, 0) / scores.length)
    : 0;

  const sTier = scores.filter((s) => s.tier === "S").length;
  const atRisk = strategies.filter((s) => s.demotionRisk).length;

  return (
    <div className="space-y-3">
      {/* KPI strip */}
      <div className="grid grid-cols-3 gap-2 md:grid-cols-6">
        {[
          { label: "Survivors", value: String(strategies.length), color: "text-emerald-400" },
          { label: "Combined PnL", value: strategies.length ? `$${totalPnl.toLocaleString(undefined, { maximumFractionDigits: 0 })}` : "—", color: totalPnl > 0 ? "text-emerald-400" : "text-rose-400" },
          { label: "Avg PF", value: strategies.length ? fmt(avgPF) : "—", color: avgPF >= 1.3 ? "text-emerald-400" : "text-amber-400" },
          { label: "Avg Win Rate", value: strategies.length ? `${fmt(avgWR, 1)}%` : "—", color: "text-sky-400" },
          { label: "Avg Authority", value: String(avgAuthority), color: "text-amber-400" },
          { label: "S-Tier", value: String(sTier), color: "text-emerald-300" },
        ].map((k) => (
          <div key={k.label} className="rounded border border-zinc-800 bg-zinc-900/40 px-2 py-1.5 text-center">
            <div className="text-[9px] uppercase text-zinc-600">{k.label}</div>
            <div className={`text-lg font-bold tabular-nums ${k.color}`}>{k.value}</div>
          </div>
        ))}
      </div>

      {atRisk > 0 && (
        <div className="rounded border border-rose-900/50 bg-rose-950/20 px-3 py-2 text-[10px] text-rose-400">
          ⚠ {atRisk} Trade Engine {atRisk === 1 ? "strategy" : "strategies"} flagged for risk review — rolling 50-trade window below thresholds
        </div>
      )}

      {loading ? (
        <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading main engine strategies…</div>
      ) : strategies.length === 0 ? (
        <div className="py-12 text-center space-y-2">
          <div className="text-sm font-semibold text-zinc-500">No Strategies in Trade Engine</div>
          <div className="text-xs text-zinc-600 max-w-md mx-auto">
            Strategies must maintain positive expectancy and PF {">"} 1 before remaining active.
          </div>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-[9px] uppercase tracking-wider text-zinc-500 border-b border-zinc-800">
                <th className="py-2 px-2">#</th>
                <th className="py-2 px-2">Strategy</th>
                <th className="py-2 px-2">Family</th>
                <th className="py-2 px-2 text-center">Authority</th>
                <th className="py-2 px-2 text-right">PnL</th>
                <th className="py-2 px-2 text-right">PF</th>
                <th className="py-2 px-2 text-right">WR%</th>
                <th className="py-2 px-2 text-right">E</th>
                <th className="py-2 px-2 text-right">Sharpe</th>
                <th className="py-2 px-2 text-right">DD%</th>
                <th className="py-2 px-2 text-right">Trades</th>
                <th className="py-2 px-2 text-center">State</th>
              </tr>
            </thead>
            <tbody>
              {strategies.map((s, i) => {
                const score = computeAuthorityScore(s.metrics, "MAIN_ENGINE");
                const isExpanded = expanded === s.strategy_id;
                return (
                  <>
                    <tr
                      key={s.strategy_id}
                      className={`border-t border-zinc-800 cursor-pointer transition-colors ${isExpanded ? "bg-zinc-800/30" : "hover:bg-zinc-900/30"}`}
                      onClick={() => setExpanded(isExpanded ? null : s.strategy_id)}
                    >
                      <td className="py-1.5 px-2 tabular-nums text-zinc-600 text-[10px]">{i + 1}</td>
                      <td className="py-1.5 px-2 font-semibold text-emerald-300 max-w-[180px] truncate">{s.strategy_name}</td>
                      <td className="py-1.5 px-2 text-[10px] text-zinc-500">{s.family}</td>
                      <td className="py-1.5 px-2 text-center">
                        <AuthorityBadge score={score.total} tier={score.tier} />
                      </td>
                      <td className={`py-1.5 px-2 tabular-nums text-right font-mono ${s.metrics.totalPnl >= 0 ? "text-emerald-400" : "text-rose-400"}`}>
                        ${fmt(s.metrics.totalPnl, 0)}
                      </td>
                      <td className="py-1.5 px-2 tabular-nums text-right text-emerald-400 font-semibold">
                        {fmt(s.metrics.profitFactor)}
                      </td>
                      <td className="py-1.5 px-2 tabular-nums text-right text-emerald-400">
                        {fmt(s.metrics.winRate, 1)}%
                      </td>
                      <td className="py-1.5 px-2 tabular-nums text-right text-emerald-400">
                        {fmt(s.metrics.expectancy)}
                      </td>
                      <td className="py-1.5 px-2 tabular-nums text-right text-sky-400">
                        {fmt(s.metrics.sharpeRatio)}
                      </td>
                      <td className="py-1.5 px-2 tabular-nums text-right text-zinc-400">
                        {fmt(s.metrics.maxDrawdown, 1)}%
                      </td>
                      <td className="py-1.5 px-2 tabular-nums text-right text-zinc-400">
                        {s.metrics.closedTrades}
                      </td>
                      <td className="py-1.5 px-2 text-center">
                        {s.demotionRisk
                          ? <span className="text-[9px] font-bold text-rose-400">↓ RISK</span>
                          : <span className="text-[9px] text-emerald-400">ACTIVE</span>}
                      </td>
                    </tr>
                    {isExpanded && (
                      <tr key={`${s.strategy_id}-detail`} className="border-t border-zinc-800/40 bg-zinc-900/60">
                        <td colSpan={12} className="px-4 py-3">
                          <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
                            {/* Score breakdown */}
                            <div>
                              <div className="text-[9px] uppercase text-zinc-600 mb-1.5">Authority Score Breakdown</div>
                              <div className="space-y-1.5">
                                <MiniGauge label="Win Rate" value={score.winRateScore} max={20} colorClass="bg-emerald-700" />
                                <MiniGauge label="PF" value={score.pfScore} max={20} colorClass="bg-sky-700" />
                                <MiniGauge label="Expectancy" value={score.expectancyScore} max={15} colorClass="bg-blue-700" />
                                <MiniGauge label="Drawdown" value={score.drawdownScore} max={15} colorClass="bg-violet-700" />
                                <MiniGauge label="Sharpe" value={score.sharpeScore} max={15} colorClass="bg-amber-700" />
                                <MiniGauge label="Engine" value={score.gradeScore} max={15} colorClass="bg-emerald-900" />
                              </div>
                            </div>
                            {/* Lifecycle */}
                            <div>
                              <div className="text-[9px] uppercase text-zinc-600 mb-1.5">Lifecycle</div>
                              <div className="space-y-0.5 text-[9px]">
                                {[
                                  { label: "Review Events", value: String(s.promotion_count) },
                                  { label: "Risk Events", value: String(s.demotion_count) },
                                  { label: "Admitted", value: s.migrated_at ? new Date(s.migrated_at).toLocaleDateString() : "—" },
                                  { label: "Last Evaluated", value: s.last_evaluated_at ? new Date(s.last_evaluated_at).toLocaleDateString() : "—" },
                                  { label: "Timeframe", value: s.timeframe },
                                  { label: "Category", value: s.category },
                                ].map((r) => (
                                  <div key={r.label} className="flex justify-between">
                                    <span className="text-zinc-600">{r.label}</span>
                                    <span className="text-zinc-300 tabular-nums">{r.value}</span>
                                  </div>
                                ))}
                              </div>
                            </div>
                            {/* Full metrics */}
                            <div>
                              <div className="text-[9px] uppercase text-zinc-600 mb-1.5">Full Metrics</div>
                              <div className="space-y-0.5 text-[9px]">
                                {[
                                  { label: "Total Trades", value: String(s.metrics.closedTrades) },
                                  { label: "Win Rate", value: `${fmt(s.metrics.winRate, 1)}%` },
                                  { label: "Profit Factor", value: fmt(s.metrics.profitFactor) },
                                  { label: "Expectancy", value: fmt(s.metrics.expectancy) },
                                  { label: "Sharpe Ratio", value: fmt(s.metrics.sharpeRatio) },
                                  { label: "Max Drawdown", value: `${fmt(s.metrics.maxDrawdown, 1)}%` },
                                ].map((r) => (
                                  <div key={r.label} className="flex justify-between">
                                    <span className="text-zinc-600">{r.label}</span>
                                    <span className="text-zinc-300 tabular-nums">{r.value}</span>
                                  </div>
                                ))}
                              </div>
                            </div>
                            {/* Authority tier */}
                            <div className="flex flex-col items-center justify-center">
                              <div className={`text-6xl font-black tabular-nums ${TIER_COLOR[score.tier]}`}>
                                {score.total}
                              </div>
                              <div className={`text-sm font-bold uppercase ${TIER_COLOR[score.tier]}`}>
                                Tier {score.tier}
                              </div>
                              <div className="mt-2 text-[9px] text-zinc-600 text-center">
                                Authority Score<br />out of 100
                              </div>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <TerminalCard title="Admission Thresholds" subtitle="All gates required at every grade">
        <div style={{ overflowX: "auto" }}>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <thead>
              <tr>
                {["Stage", "T", "PF", "WR", "DD", "Sh"].map((header, index) => (
                  <th
                    key={header}
                    style={{
                      padding: "7px 8px",
                      textAlign: index === 0 ? "left" : "right",
                      color: "var(--text-muted)",
                      fontSize: 9,
                      fontWeight: 800,
                      letterSpacing: "0.08em",
                      textTransform: "uppercase",
                      borderBottom: "1px solid var(--border)",
                    }}
                  >
                    {header}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {[
                { label: "G5→4", trades: "50", pf: "≥1.10", wr: "≥55%", dd: "—", sh: "—" },
                { label: "G4→3", trades: "75", pf: "≥1.15", wr: "≥55%", dd: "<25%", sh: "—" },
                { label: "G3→2", trades: "100", pf: "≥1.20", wr: "≥55%", dd: "<20%", sh: "—" },
                { label: "G2→1", trades: "150", pf: "≥1.25", wr: "≥55%", dd: "<15%", sh: "≥0.50" },
                { label: "G1→MAIN", trades: "200", pf: "≥1.30", wr: "≥55%", dd: "<10%", sh: "≥0.75", gold: true },
                { label: "MAIN ENGINE SURVIVORS", trades: "575", pf: "≥1.30", wr: "≥55%", dd: "<10%", sh: "≥0.75", survivor: true },
              ].map((row, index) => {
                const color = row.gold ? "var(--gold)" : "var(--text-secondary)";
                return (
                  <tr
                    key={row.label}
                    style={{
                      background: index % 2 === 0 ? "var(--surface)" : "var(--surface-2)",
                      borderTop: row.survivor ? "1px solid var(--gold)" : "1px solid var(--border-subtle, var(--border))",
                    }}
                  >
                    <td
                      style={{
                        padding: "7px 8px",
                        color,
                        fontSize: 12,
                        fontWeight: row.gold || row.survivor ? 700 : 600,
                        letterSpacing: "0.04em",
                        textTransform: "uppercase",
                        whiteSpace: "nowrap",
                      }}
                    >
                      {row.label}
                    </td>
                    {[row.trades, row.pf, row.wr, row.dd, row.sh].map((value, valueIndex) => (
                      <td
                        key={`${row.label}-${valueIndex}`}
                        style={{
                          padding: "7px 8px",
                          textAlign: "right",
                          color,
                          fontFamily: "var(--font-mono)",
                          fontSize: 12,
                          fontVariantNumeric: "tabular-nums",
                          fontWeight: row.gold ? 700 : 500,
                          whiteSpace: "nowrap",
                        }}
                      >
                        {value}
                      </td>
                    ))}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </TerminalCard>

      <p className="text-[10px] uppercase tracking-wider text-zinc-600">
        Trade Engine Survivors — elite only — no live capital at risk
      </p>
    </div>
  );
}
