"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type {
  PipelineState,
  StrategyLeaderboardEntry,
  StrategyStatus,
} from "@/lib/strategyAuthority/types";
import type { FamilyIntelligenceRow } from "@/lib/strategyAuthority/strategyAuthorityMongo";
import { computeAuthorityScore, TIER_COLOR } from "@/lib/strategyAuthority/authorityScore";
import { TerminalCard, Metric } from "./TerminalCard";
import { PromotionTower } from "./PromotionTower";
import { PromotionFeed } from "./PromotionFeed";
import { FamilyLeaderboard } from "./FamilyLeaderboard";
import { MainEngineSurvivors } from "./MainEngineSurvivors";
import { RetirementCenter } from "./RetirementCenter";

// ── Types ─────────────────────────────────────────────────────────────────────

interface TowerCounts {
  status: StrategyStatus;
  count: number;
  promotionEventsTotal: number;
  demotionEventsTotal: number;
}

interface PipelineCounts {
  total: number;
  byStatus: Partial<Record<StrategyStatus, number>>;
  promotionsToday: number;
  demotionsToday: number;
  retirementsToday: number;
}

type TabId = "overview" | "pipeline" | "families" | "main-engine" | "retirements" | "sep";

const TABS: { id: TabId; label: string }[] = [
  { id: "overview", label: "Overview" },
  { id: "pipeline", label: "Pipeline" },
  { id: "families", label: "Families" },
  { id: "main-engine", label: "Main Engine" },
  { id: "retirements", label: "Retirements" },
  { id: "sep", label: "SEP Intelligence" },
];

// ── Helpers ───────────────────────────────────────────────────────────────────

const STATUS_LABEL: Record<StrategyStatus, string> = {
  GRADE_5: "G5", GRADE_4: "G4", GRADE_3: "G3",
  GRADE_2: "G2", GRADE_1: "G1", MAIN_ENGINE: "MAIN", RETIRED: "RET",
};
const STATUS_COLOR: Record<StrategyStatus, string> = {
  GRADE_5: "text-zinc-400", GRADE_4: "text-sky-400", GRADE_3: "text-blue-400",
  GRADE_2: "text-violet-400", GRADE_1: "text-amber-400", MAIN_ENGINE: "text-emerald-400", RETIRED: "text-rose-400",
};

function fmt(n: number | undefined, dec = 2) {
  if (n == null || isNaN(n)) return "—";
  return n.toFixed(dec);
}

// ── Stage card (used in Pipeline tab) ─────────────────────────────────────────

function PipelineStageCard({
  stage, selected, onClick,
}: {
  stage: PipelineState["stages"][number];
  selected: boolean;
  onClick: () => void;
}) {
  const s = stage.status;
  return (
    <button
      onClick={onClick}
      className={`w-full rounded border p-3 text-left transition-all ${selected ? "ring-1 ring-white/20" : ""}
        ${s === "MAIN_ENGINE" ? "border-emerald-700 bg-emerald-950/40"
        : s === "GRADE_1" ? "border-amber-800 bg-amber-950/30"
        : s === "GRADE_2" ? "border-violet-800 bg-violet-950/30"
        : s === "GRADE_3" ? "border-blue-800 bg-blue-950/30"
        : s === "GRADE_4" ? "border-sky-800 bg-sky-950/30"
        : "border-zinc-700 bg-zinc-900/50"}`}
    >
      <div className="flex items-center justify-between">
        <span className={`text-[10px] font-bold uppercase tracking-widest ${STATUS_COLOR[s]}`}>
          {STATUS_LABEL[s]}
        </span>
        <span className={`text-2xl font-bold tabular-nums ${STATUS_COLOR[s]}`}>{stage.count}</span>
      </div>
      <div className="mt-0.5 text-[9px] text-zinc-500 truncate">{stage.label}</div>
      <div className="mt-2 flex gap-2 text-[9px] tabular-nums text-zinc-600">
        <span>WR {fmt(stage.avgWinRate, 1)}%</span>
        <span>PF {fmt(stage.avgProfitFactor)}</span>
        <span>E {fmt(stage.avgExpectancy)}</span>
      </div>
      <div className="mt-1.5 flex gap-1.5 flex-wrap">
        {stage.promotionCandidates > 0 && (
          <span className="text-[8px] rounded bg-emerald-900/40 px-1 py-0.5 text-emerald-400">↑{stage.promotionCandidates}</span>
        )}
        {stage.demotionRisk > 0 && (
          <span className="text-[8px] rounded bg-rose-900/30 px-1 py-0.5 text-rose-400">↓{stage.demotionRisk}</span>
        )}
      </div>
    </button>
  );
}

// ── Stage detail panel ────────────────────────────────────────────────────────

function StageDetail({ stage }: { stage: PipelineState["stages"][number] }) {
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-2">
        <Metric label="Strategies" value={String(stage.count)} />
        <Metric label="Avg PF" value={fmt(stage.avgProfitFactor)} tone={stage.avgProfitFactor >= 1.2 ? "positive" : "warning"} />
        <Metric label="Avg Win Rate" value={`${fmt(stage.avgWinRate, 1)}%`} tone={stage.avgWinRate >= 55 ? "positive" : "warning"} />
        <Metric label="Avg Drawdown" value={`${fmt(stage.avgDrawdown, 1)}%`} tone={stage.avgDrawdown < 15 ? "positive" : "warning"} />
      </div>
      {stage.topPerformers.length > 0 && (
        <div>
          <div className="mb-1 text-[9px] uppercase text-zinc-600">Top Performers</div>
          {stage.topPerformers.slice(0, 3).map((s) => (
            <div key={s.strategy_id} className="flex items-center justify-between rounded border border-zinc-800 px-2 py-1 text-[10px] mb-1">
              <span className="truncate text-zinc-300 max-w-[160px]">{s.strategy_name}</span>
              <span className="text-emerald-400 tabular-nums ml-2">PF {fmt(s.metrics.profitFactor)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ── Leaderboard row ───────────────────────────────────────────────────────────

function LeaderboardRow({ s }: { s: StrategyLeaderboardEntry }) {
  const score = computeAuthorityScore(s.metrics, s.current_status);
  return (
    <tr className="border-t border-zinc-800 hover:bg-zinc-900/30">
      <td className="py-1.5 px-2 text-[10px] tabular-nums text-zinc-600">{s.rank}</td>
      <td className="py-1.5 px-2 text-xs text-zinc-200 max-w-[180px] truncate font-medium">{s.strategy_name}</td>
      <td className="py-1.5 px-2 text-[10px] text-zinc-500">{s.family}</td>
      <td className="py-1.5 px-2">
        <span className={`text-[9px] font-bold uppercase ${STATUS_COLOR[s.current_status]}`}>
          {STATUS_LABEL[s.current_status]}
        </span>
      </td>
      <td className="py-1.5 px-2 text-center">
        <span className={`text-[9px] font-bold tabular-nums ${TIER_COLOR[score.tier]}`}>{score.total}</span>
      </td>
      <td className={`py-1.5 px-2 tabular-nums text-xs text-right ${s.metrics.winRate >= 55 ? "text-emerald-400" : "text-amber-400"}`}>
        {fmt(s.metrics.winRate, 1)}%
      </td>
      <td className={`py-1.5 px-2 tabular-nums text-xs text-right ${s.metrics.profitFactor >= 1.2 ? "text-emerald-400" : "text-amber-400"}`}>
        {fmt(s.metrics.profitFactor)}
      </td>
      <td className={`py-1.5 px-2 tabular-nums text-xs text-right ${s.metrics.expectancy > 0 ? "text-emerald-400" : "text-rose-400"}`}>
        {fmt(s.metrics.expectancy)}
      </td>
      <td className="py-1.5 px-2 tabular-nums text-xs text-right text-zinc-500">{s.metrics.closedTrades}</td>
      <td className="py-1.5 px-2 text-center text-[9px]">
        {s.promotionEligible ? <span className="text-emerald-400">↑</span>
          : s.demotionRisk ? <span className="text-rose-400">↓</span>
          : <span className="text-zinc-700">—</span>}
      </td>
    </tr>
  );
}

// ── SEP Intelligence panel ────────────────────────────────────────────────────

function SepIntelligencePanel() {
  return (
    <div className="space-y-3">
      <div className="rounded border border-zinc-800 bg-zinc-900/30 px-4 py-8 text-center">
        <div className="text-sm font-semibold text-zinc-500">SEP Evidence Integration</div>
        <div className="mt-2 text-xs text-zinc-600 max-w-md mx-auto">
          SEP evidence scores are sourced from <code className="text-zinc-400">SEP_REPORTS_DIR/strategy_evidence.json</code>.
          When available, they contribute up to 15 points to each strategy&apos;s Authority Score.
        </div>
        <div className="mt-4 grid grid-cols-3 gap-2 max-w-sm mx-auto text-[10px]">
          {[
            { range: "0–5", label: "Insufficient evidence" },
            { range: "5–10", label: "Growing evidence" },
            { range: "10–15", label: "Strong evidence" },
          ].map((r) => (
            <div key={r.range} className="rounded border border-zinc-800 px-2 py-2">
              <div className="font-bold text-zinc-400">{r.range}</div>
              <div className="text-zinc-600 mt-0.5">{r.label}</div>
            </div>
          ))}
        </div>
        <div className="mt-4 text-[10px] text-zinc-700">
          SEP data is matched by strategy name. If no SEP file is found, evidence score defaults to 0.
        </div>
      </div>
    </div>
  );
}

// ── Main Component ────────────────────────────────────────────────────────────

export function StrategyAuthorityCenter() {
  const [activeTab, setActiveTab] = useState<TabId>("overview");
  const [pipeline, setPipeline] = useState<PipelineState | null>(null);
  const [leaderboard, setLeaderboard] = useState<StrategyLeaderboardEntry[]>([]);
  const [towerCounts, setTowerCounts] = useState<TowerCounts[]>([]);
  const [pipelineCounts, setPipelineCounts] = useState<PipelineCounts | null>(null);
  const [migrationStatus, setMigrationStatus] = useState<{ migrated: boolean; totalInDb: number } | null>(null);
  const [loading, setLoading] = useState(true);
  const [migrating, setMigrating] = useState(false);
  const [evaluating, setEvaluating] = useState(false);
  const [evalResult, setEvalResult] = useState<{ promoted: number; demoted: number; retired: number } | null>(null);
  const [selectedStage, setSelectedStage] = useState<StrategyStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchOverview = useCallback(async () => {
    try {
      const [countsRes, migrRes] = await Promise.all([
        fetch("/api/strategy-authority/counts"),
        fetch("/api/strategy-authority/migrate"),
      ]);
      const [cData, mData] = await Promise.all([countsRes.json(), migrRes.json()]);
      if (cData.ok) {
        setTowerCounts(cData.tower ?? []);
        setPipelineCounts(cData.counts ?? null);
      }
      if (mData.ok) setMigrationStatus({ migrated: mData.migrated, totalInDb: mData.totalInDb });
      setError(null);
    } catch {
      setError("Failed to load strategy authority data");
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchPipeline = useCallback(async () => {
    const [pRes, lRes] = await Promise.all([
      fetch("/api/strategy-authority/stages"),
      fetch("/api/strategy-authority/leaderboard?limit=100"),
    ]);
    const [pData, lData] = await Promise.all([pRes.json(), lRes.json()]);
    if (pData.ok) setPipeline(pData.pipeline);
    if (lData.ok) setLeaderboard(lData.leaderboard);
  }, []);

  useEffect(() => {
    fetchOverview();
    pollingRef.current = setInterval(fetchOverview, 30_000);
    return () => { if (pollingRef.current) clearInterval(pollingRef.current); };
  }, [fetchOverview]);

  useEffect(() => {
    if (activeTab === "pipeline") fetchPipeline();
  }, [activeTab, fetchPipeline]);

  const handleMigrate = async () => {
    setMigrating(true);
    try {
      await fetch("/api/strategy-authority/migrate", { method: "POST" });
      await fetchOverview();
    } finally {
      setMigrating(false);
    }
  };

  const handleEvaluate = async () => {
    setEvaluating(true);
    setEvalResult(null);
    try {
      const res = await fetch("/api/strategy-authority/evaluate", { method: "POST" });
      const data = await res.json();
      if (data.ok) setEvalResult(data.result);
      await fetchOverview();
      if (activeTab === "pipeline") await fetchPipeline();
    } finally {
      setEvaluating(false);
    }
  };

  const needsMigration = migrationStatus && !migrationStatus.migrated;
  const counts = pipelineCounts;

  return (
    <div className="m3-page-stack">
      {/* Migration alert */}
      {needsMigration && (
        <div className="rounded border border-amber-800/60 bg-amber-950/30 px-4 py-3 flex items-center justify-between">
          <div>
            <div className="text-xs font-bold text-amber-400">ISPAP Not Initialized</div>
            <div className="text-[11px] text-zinc-400 mt-0.5">305 strategies must be seeded into Grade 5 before the pipeline is operational.</div>
          </div>
          <button
            onClick={handleMigrate}
            disabled={migrating}
            className="rounded border border-amber-700 bg-amber-900/40 px-3 py-1.5 text-xs font-semibold text-amber-300 hover:bg-amber-800/40 disabled:opacity-50 transition-colors"
          >
            {migrating ? "Migrating…" : "Run Grade 5 Migration"}
          </button>
        </div>
      )}

      {/* Evaluation result toast */}
      {evalResult && (
        <div className="rounded border border-sky-800/50 bg-sky-950/20 px-3 py-2 flex items-center gap-4 text-[10px]">
          <span className="text-sky-400 font-bold uppercase">Evaluation Complete</span>
          <span className="text-emerald-400">↑{evalResult.promoted} promoted</span>
          <span className="text-rose-400">↓{evalResult.demoted} demoted</span>
          <span className="text-rose-600">✕{evalResult.retired} retired</span>
          <button onClick={() => setEvalResult(null)} className="ml-auto text-zinc-600 hover:text-zinc-400">✕</button>
        </div>
      )}

      {/* KPI Strip */}
      <div className="m3-kpi-strip">
        <Metric label="Total" value={String(counts?.total ?? 305)} />
        <Metric label="Active" value={String(counts ? (counts.total - (counts.byStatus.RETIRED ?? 0)) : "—")} tone="positive" />
        <Metric label="Main Engine" value={String(counts?.byStatus.MAIN_ENGINE ?? 0)} tone={(counts?.byStatus.MAIN_ENGINE ?? 0) > 0 ? "positive" : "neutral"} />
        <Metric label="Retired" value={String(counts?.byStatus.RETIRED ?? 0)} tone={(counts?.byStatus.RETIRED ?? 0) > 0 ? "warning" : "neutral"} />
        <Metric label="Promoted Today" value={String(counts?.promotionsToday ?? 0)} tone={(counts?.promotionsToday ?? 0) > 0 ? "positive" : "neutral"} />
        <Metric label="Demoted Today" value={String(counts?.demotionsToday ?? 0)} tone={(counts?.demotionsToday ?? 0) > 0 ? "negative" : "neutral"} />
        <Metric label="Retired Today" value={String(counts?.retirementsToday ?? 0)} tone={(counts?.retirementsToday ?? 0) > 0 ? "negative" : "neutral"} />
        <Metric label="In DB" value={String(migrationStatus?.totalInDb ?? "—")} />
      </div>

      {/* Tab bar */}
      <div className="flex items-center gap-0 border-b border-zinc-800 overflow-x-auto">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`px-4 py-2 text-[10px] font-bold uppercase tracking-widest whitespace-nowrap border-b-2 -mb-px transition-colors ${
              activeTab === tab.id
                ? "border-sky-500 text-sky-400"
                : "border-transparent text-zinc-500 hover:text-zinc-300"
            }`}
          >
            {tab.label}
          </button>
        ))}
        {/* Evaluate button */}
        <div className="ml-auto flex-shrink-0 flex items-center gap-2 pb-1 pl-4">
          <button
            onClick={handleEvaluate}
            disabled={evaluating || !migrationStatus?.migrated}
            className="rounded border border-zinc-700 px-3 py-1 text-[9px] font-bold uppercase tracking-wider text-zinc-400 hover:border-sky-700 hover:text-sky-400 disabled:opacity-40 transition-colors"
          >
            {evaluating ? "Evaluating…" : "Run Evaluation"}
          </button>
        </div>
      </div>

      {error && (
        <div className="rounded border border-rose-800 bg-rose-950/30 px-3 py-2 text-xs text-rose-400">{error}</div>
      )}

      {/* ── OVERVIEW TAB ── */}
      {activeTab === "overview" && (
        <div className="grid gap-3 xl:grid-cols-3">
          {/* Promotion Tower */}
          <div className="xl:col-span-1">
            {loading ? (
              <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading tower…</div>
            ) : (
              <PromotionTower
                layers={towerCounts.filter((t) => t.status !== "RETIRED").map((t) => ({
                  status: t.status,
                  count: t.count,
                  promotionEventsTotal: t.promotionEventsTotal,
                  demotionEventsTotal: t.demotionEventsTotal,
                }))}
                retiredCount={towerCounts.find((t) => t.status === "RETIRED")?.count ?? 0}
                onSelectStatus={setSelectedStage}
                selectedStatus={selectedStage}
                totalStrategies={counts?.total ?? 305}
              />
            )}
          </div>

          {/* Live Feed */}
          <div className="xl:col-span-2">
            <TerminalCard title="Live Promotion Feed" subtitle="Last 500 events — refreshes every 15s">
              <PromotionFeed maxHeight="max-h-[440px]" />
            </TerminalCard>
          </div>
        </div>
      )}

      {/* ── PIPELINE TAB ── */}
      {activeTab === "pipeline" && (
        <div className="grid gap-3 xl:grid-cols-3">
          <div className="xl:col-span-2 space-y-3">
            <TerminalCard title="Promotion Pipeline" subtitle="All 6 grades — click to inspect">
              {!pipeline ? (
                <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading pipeline…</div>
              ) : (
                <div className="space-y-2">
                  <div className="grid grid-cols-3 gap-2">
                    {pipeline.stages.filter((s) => s.status !== "MAIN_ENGINE").map((stage) => (
                      <PipelineStageCard
                        key={stage.status}
                        stage={stage}
                        selected={selectedStage === stage.status}
                        onClick={() => setSelectedStage(selectedStage === stage.status ? null : stage.status)}
                      />
                    ))}
                  </div>
                  {pipeline.stages.filter((s) => s.status === "MAIN_ENGINE").map((stage) => (
                    <PipelineStageCard
                      key="main"
                      stage={stage}
                      selected={selectedStage === "MAIN_ENGINE"}
                      onClick={() => setSelectedStage(selectedStage === "MAIN_ENGINE" ? null : "MAIN_ENGINE")}
                    />
                  ))}
                </div>
              )}
            </TerminalCard>

            {/* Strategy intelligence table */}
            <TerminalCard title="Strategy Intelligence" subtitle="All active strategies ranked by profit factor">
              {leaderboard.length === 0 ? (
                <div className="py-6 text-center text-xs text-zinc-600">No data — run evaluation first</div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="text-left text-[9px] uppercase tracking-wider text-zinc-500 border-b border-zinc-800">
                        <th className="py-2 px-2">#</th>
                        <th className="py-2 px-2">Strategy</th>
                        <th className="py-2 px-2">Family</th>
                        <th className="py-2 px-2">Grade</th>
                        <th className="py-2 px-2 text-center">Auth</th>
                        <th className="py-2 px-2 text-right">WR%</th>
                        <th className="py-2 px-2 text-right">PF</th>
                        <th className="py-2 px-2 text-right">E</th>
                        <th className="py-2 px-2 text-right">T</th>
                        <th className="py-2 px-2 text-center">↕</th>
                      </tr>
                    </thead>
                    <tbody>
                      {leaderboard.map((s) => <LeaderboardRow key={s.strategy_id} s={s} />)}
                    </tbody>
                  </table>
                </div>
              )}
            </TerminalCard>
          </div>

          {/* Stage detail */}
          <div className="space-y-3">
            <TerminalCard
              title={pipeline?.stages.find((s) => s.status === selectedStage)?.label ?? "Stage Details"}
              subtitle={selectedStage ? "Click another stage to switch" : "Select a stage above"}
            >
              {selectedStage && pipeline ? (
                <StageDetail stage={pipeline.stages.find((s) => s.status === selectedStage)!} />
              ) : (
                <div className="py-8 text-center text-[11px] text-zinc-600">Click any stage card to inspect it</div>
              )}
            </TerminalCard>
          </div>
        </div>
      )}

      {/* ── FAMILIES TAB ── */}
      {activeTab === "families" && (
        <TerminalCard title="Family Intelligence Dashboard" subtitle="Strategy families ranked by avg profit factor — click to expand">
          <FamilyLeaderboard />
        </TerminalCard>
      )}

      {/* ── MAIN ENGINE TAB ── */}
      {activeTab === "main-engine" && <MainEngineSurvivors />}

      {/* ── RETIREMENTS TAB ── */}
      {activeTab === "retirements" && <RetirementCenter />}

      {/* ── SEP TAB ── */}
      {activeTab === "sep" && <SepIntelligencePanel />}

      {/* Footer */}
      <p className="text-[10px] uppercase tracking-wider text-zinc-700">
        ISPAP v2 · Institutional Strategy Promotion Authority Program · {counts?.total ?? 305} strategies tracked
      </p>
    </div>
  );
}
