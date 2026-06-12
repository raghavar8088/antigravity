"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { TerminalCard, Metric } from "./TerminalCard";
import { PromotionTower } from "./PromotionTower";
import { PromotionFeed } from "./PromotionFeed";
import { CorrelationMatrix } from "./CorrelationMatrix";
import { AllocationView } from "./AllocationView";
import { RegimeIntelligence } from "./RegimeIntelligence";
import { CandidateQueue } from "./CandidateQueue";
import { StrategyGenome } from "./StrategyGenome";
import { PortfolioConstruction } from "./PortfolioConstruction";
import { FamilyLeaderboard } from "./FamilyLeaderboard";

// ── Types ─────────────────────────────────────────────────────────────────────

interface OverviewKpis {
  totalStrategies: number;
  mainEngine: number;
  candidateQueue: number;
  allocated: number;
  expectedPf: number;
  expectedSharpe: number;
  hhi: number;
  currentRegime: string | null;
  computedAt: string | null;
}

// ── Tab config ────────────────────────────────────────────────────────────────

const TABS = [
  { id: "overview", label: "Overview" },
  { id: "allocation", label: "Allocation" },
  { id: "correlation", label: "Correlation" },
  { id: "regimes", label: "Regimes" },
  { id: "candidate-queue", label: "Candidate Queue" },
  { id: "families", label: "Families" },
  { id: "portfolio", label: "Construction" },
  { id: "genome", label: "Genome" },
  { id: "tower", label: "Tower" },
] as const;

type TabId = (typeof TABS)[number]["id"];

// ── Compute button ────────────────────────────────────────────────────────────

function ComputeButton({ onComplete }: { onComplete: () => void }) {
  const [computing, setComputing] = useState(false);
  const [lastResult, setLastResult] = useState<string | null>(null);

  const handleCompute = async () => {
    setComputing(true);
    setLastResult(null);
    try {
      const res = await fetch("/api/strategy-authority/compute-portfolio", { method: "POST" });
      const data = await res.json();
      if (data.ok) {
        const r = data.result;
        setLastResult(`Done: ${r.correlations_computed} correlations, ${r.genomes_computed} genomes, ${r.candidates_evaluated} candidates (${r.elapsed_ms}ms)`);
        onComplete();
      } else {
        setLastResult(`Error: ${data.error ?? "unknown"}`);
      }
    } catch (e) {
      setLastResult("Network error");
    } finally {
      setComputing(false);
    }
  };

  return (
    <div className="flex flex-col items-end gap-0.5">
      <button
        onClick={handleCompute}
        disabled={computing}
        className="rounded border border-violet-700 bg-violet-900/30 px-3 py-1 text-[9px] font-bold uppercase tracking-wider text-violet-300 hover:bg-violet-800/40 disabled:opacity-40 transition-colors"
      >
        {computing ? "Computing Portfolio Intelligence…" : "Run APICAP Compute"}
      </button>
      {lastResult && <span className="text-[8px] text-zinc-600 max-w-64 text-right">{lastResult}</span>}
    </div>
  );
}

// ── Overview panel ────────────────────────────────────────────────────────────

function OverviewPanel({ kpis, towerCounts, onRefresh }: {
  kpis: OverviewKpis;
  towerCounts: any[];
  onRefresh: () => void;
}) {
  return (
    <div className="space-y-3">
      {/* Intelligence KPIs */}
      <div className="grid grid-cols-4 gap-2 md:grid-cols-8">
        {[
          { label: "Strategies", value: String(kpis.totalStrategies) },
          { label: "Main Engine", value: String(kpis.mainEngine), color: "text-emerald-400" },
          { label: "Candidate Queue", value: String(kpis.candidateQueue), color: "text-sky-400" },
          { label: "Allocated", value: String(kpis.allocated), color: "text-violet-400" },
          { label: "Portfolio PF", value: kpis.expectedPf > 0 ? kpis.expectedPf.toFixed(3) : "—", color: "text-sky-400" },
          { label: "Portfolio Sharpe", value: kpis.expectedSharpe > 0 ? kpis.expectedSharpe.toFixed(3) : "—", color: "text-blue-400" },
          { label: "HHI", value: kpis.hhi > 0 ? String(Math.round(kpis.hhi)) : "—", color: kpis.hhi < 1000 && kpis.hhi > 0 ? "text-emerald-400" : "text-amber-400" },
          { label: "Regime", value: kpis.currentRegime ? kpis.currentRegime.replace("_", " ").slice(0, 8) : "—", color: "text-zinc-400" },
        ].map((k) => (
          <div key={k.label} className="rounded border border-zinc-800 bg-zinc-900/40 px-2 py-1.5 text-center">
            <div className="text-[9px] uppercase text-zinc-600">{k.label}</div>
            <div className={`text-sm font-bold tabular-nums ${k.color ?? "text-zinc-200"}`}>{k.value}</div>
          </div>
        ))}
      </div>

      {/* Architecture diagram */}
      <div className="rounded border border-zinc-800 bg-zinc-900/20 px-4 py-3">
        <div className="text-[9px] uppercase text-zinc-600 mb-2">APICAP Target Architecture</div>
        <div className="flex flex-wrap items-center gap-1 text-[9px]">
          {[
            { label: "305 Strategies", color: "border-zinc-700 text-zinc-400" },
            { label: "Promotion Engine", color: "border-sky-800 text-sky-400" },
            { label: "Portfolio Intelligence", color: "border-violet-700 text-violet-400" },
            { label: "Correlation Engine", color: "border-blue-800 text-blue-400" },
            { label: "Allocation Engine", color: "border-emerald-800 text-emerald-400" },
            { label: "Candidate Queue", color: "border-amber-800 text-amber-400" },
            { label: "Main Engine", color: "border-emerald-700 text-emerald-300" },
          ].map((s, i) => (
            <div key={s.label} className="flex items-center gap-1">
              {i > 0 && <span className="text-zinc-700">↓</span>}
              <span className={`rounded border px-1.5 py-0.5 ${s.color}`}>{s.label}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Tower + Feed */}
      <div className="grid gap-3 xl:grid-cols-5">
        <div className="xl:col-span-2">
          <TerminalCard title="Promotion Tower" subtitle="Strategy pipeline population">
            <PromotionTower
              layers={towerCounts.filter((t) => t.status !== "RETIRED").map((t: any) => ({
                status: t.status,
                count: t.count,
                promotionEventsTotal: t.promotionEventsTotal,
                demotionEventsTotal: t.demotionEventsTotal,
              }))}
              retiredCount={towerCounts.find((t: any) => t.status === "RETIRED")?.count ?? 0}
              totalStrategies={kpis.totalStrategies || 305}
            />
          </TerminalCard>
        </div>
        <div className="xl:col-span-3">
          <TerminalCard title="Live Activity Feed" subtitle="Promotions, demotions, retirements">
            <PromotionFeed maxHeight="max-h-80" />
          </TerminalCard>
        </div>
      </div>

      {kpis.computedAt && (
        <div className="text-[9px] text-zinc-700">Portfolio intelligence last computed: {new Date(kpis.computedAt).toLocaleString()}</div>
      )}
    </div>
  );
}

// ── Main Component ────────────────────────────────────────────────────────────

export function PortfolioIntelligenceCenter() {
  const [activeTab, setActiveTab] = useState<TabId>("overview");
  const [overviewKpis, setOverviewKpis] = useState<OverviewKpis>({
    totalStrategies: 305,
    mainEngine: 0,
    candidateQueue: 0,
    allocated: 0,
    expectedPf: 0,
    expectedSharpe: 0,
    hhi: 0,
    currentRegime: null,
    computedAt: null,
  });
  const [towerCounts, setTowerCounts] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [needsCompute, setNeedsCompute] = useState(false);
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchOverview = useCallback(async () => {
    try {
      const [countsRes, allocRes, regimeRes, queueRes, constructRes] = await Promise.allSettled([
        fetch("/api/strategy-authority/counts"),
        fetch("/api/strategy-authority/allocation"),
        fetch("/api/strategy-authority/regimes"),
        fetch("/api/strategy-authority/candidate-queue"),
        fetch("/api/strategy-authority/portfolio-construction"),
      ]);

      const counts = countsRes.status === "fulfilled" ? await countsRes.value.json() : null;
      const alloc = allocRes.status === "fulfilled" ? await allocRes.value.json() : null;
      const regime = regimeRes.status === "fulfilled" ? await regimeRes.value.json() : null;
      const queue = queueRes.status === "fulfilled" ? await queueRes.value.json() : null;
      const construct = constructRes.status === "fulfilled" ? await constructRes.value.json() : null;

      if (counts?.ok) setTowerCounts(counts.tower ?? []);

      const hasComputed = (alloc?.ok && alloc?.allocation?.total_allocated_strategies > 0) ||
        (construct?.ok);
      setNeedsCompute(!hasComputed);

      setOverviewKpis({
        totalStrategies: counts?.counts?.total ?? 305,
        mainEngine: counts?.counts?.byStatus?.MAIN_ENGINE ?? 0,
        candidateQueue: queue?.ok ? queue.queue.total_candidates : 0,
        allocated: alloc?.ok ? alloc.allocation.total_allocated_strategies : 0,
        expectedPf: construct?.ok ? construct.construction.expected_portfolio_pf : 0,
        expectedSharpe: construct?.ok ? construct.construction.expected_portfolio_sharpe : 0,
        hhi: construct?.ok ? construct.construction.hhi : 0,
        currentRegime: regime?.ok ? regime.currentRegime : null,
        computedAt: construct?.ok ? construct.construction.computed_at : null,
      });
    } catch {
      // Silent fail — partial data is fine
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchOverview();
    pollingRef.current = setInterval(fetchOverview, 30_000);
    return () => { if (pollingRef.current) clearInterval(pollingRef.current); };
  }, [fetchOverview]);

  return (
    <div className="m3-page-stack">
      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-2">
        <div>
          <div className="text-[10px] uppercase tracking-widest text-violet-500">APICAP</div>
          <div className="text-[9px] text-zinc-600">Autonomous Portfolio Intelligence & Capital Allocation Program</div>
        </div>
        <ComputeButton onComplete={fetchOverview} />
      </div>

      {/* First-run prompt */}
      {!loading && needsCompute && (
        <div className="rounded border border-violet-800/50 bg-violet-950/20 px-4 py-3 text-xs text-violet-300">
          <span className="font-bold">Portfolio Intelligence Not Computed</span>
          <span className="text-zinc-400 ml-2">Click "Run APICAP Compute" above to compute correlation matrix, allocations, regime profiles, genomes, and candidate queue.</span>
        </div>
      )}

      {/* KPI Strip */}
      <div className="m3-kpi-strip">
        <Metric label="Total" value={String(overviewKpis.totalStrategies)} />
        <Metric label="Main Engine" value={String(overviewKpis.mainEngine)} tone={overviewKpis.mainEngine > 0 ? "positive" : "neutral"} />
        <Metric label="Candidates" value={String(overviewKpis.candidateQueue)} tone={overviewKpis.candidateQueue > 0 ? "positive" : "neutral"} />
        <Metric label="Allocated" value={String(overviewKpis.allocated)} tone={overviewKpis.allocated > 0 ? "positive" : "neutral"} />
        <Metric label="Portfolio PF" value={overviewKpis.expectedPf > 0 ? overviewKpis.expectedPf.toFixed(3) : "—"} tone={overviewKpis.expectedPf >= 1.2 ? "positive" : "neutral"} />
        <Metric label="Portfolio Sharpe" value={overviewKpis.expectedSharpe > 0 ? overviewKpis.expectedSharpe.toFixed(3) : "—"} />
        <Metric label="HHI" value={overviewKpis.hhi > 0 ? String(Math.round(overviewKpis.hhi)) : "—"} tone={overviewKpis.hhi > 0 && overviewKpis.hhi < 1000 ? "positive" : overviewKpis.hhi >= 2500 ? "negative" : "neutral"} />
        <Metric label="Regime" value={overviewKpis.currentRegime?.replace("_BREAKOUT", "").replace("_CHOP", "").slice(0, 8) ?? "—"} />
      </div>

      {/* Tab bar */}
      <div className="flex border-b border-zinc-800 overflow-x-auto">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`px-4 py-2 text-[10px] font-bold uppercase tracking-widest whitespace-nowrap border-b-2 -mb-px transition-colors flex-shrink-0 ${
              activeTab === tab.id
                ? "border-violet-500 text-violet-400"
                : "border-transparent text-zinc-500 hover:text-zinc-300"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {activeTab === "overview" && (
        <OverviewPanel kpis={overviewKpis} towerCounts={towerCounts} onRefresh={fetchOverview} />
      )}

      {activeTab === "allocation" && (
        <TerminalCard title="Capital Allocation Authority" subtitle="Risk-parity × authority × diversification weights">
          <AllocationView />
        </TerminalCard>
      )}

      {activeTab === "correlation" && (
        <TerminalCard title="Correlation Intelligence" subtitle="Pairwise PnL correlation · diversification scoring">
          <CorrelationMatrix />
        </TerminalCard>
      )}

      {activeTab === "regimes" && (
        <TerminalCard title="Regime Intelligence" subtitle="Strategy performance conditioned on market regime">
          <RegimeIntelligence />
        </TerminalCard>
      )}

      {activeTab === "candidate-queue" && (
        <TerminalCard title="Candidate Queue" subtitle="5-gate portfolio admission system for Grade 1 → Main Engine">
          <CandidateQueue />
        </TerminalCard>
      )}

      {activeTab === "families" && (
        <TerminalCard title="Family Survival Intelligence" subtitle="Alpha families ranked by collective portfolio contribution">
          <FamilyLeaderboard />
        </TerminalCard>
      )}

      {activeTab === "portfolio" && (
        <TerminalCard title="Portfolio Construction" subtitle="Optimal composition · HHI concentration · family exposure">
          <PortfolioConstruction />
        </TerminalCard>
      )}

      {activeTab === "genome" && (
        <TerminalCard title="Strategy Genome" subtitle="Full per-strategy profile: authority + diversification + regime + allocation">
          <StrategyGenome />
        </TerminalCard>
      )}

      {activeTab === "tower" && (
        <TerminalCard title="Promotion Tower" subtitle="Full strategy ecosystem — Grade 5 to Main Engine">
          <PromotionTower
            layers={towerCounts.filter((t: any) => t.status !== "RETIRED").map((t: any) => ({
              status: t.status,
              count: t.count,
              promotionEventsTotal: t.promotionEventsTotal,
              demotionEventsTotal: t.demotionEventsTotal,
            }))}
            retiredCount={towerCounts.find((t: any) => t.status === "RETIRED")?.count ?? 0}
            totalStrategies={overviewKpis.totalStrategies || 305}
          />
        </TerminalCard>
      )}

      <p className="text-[10px] uppercase tracking-wider text-zinc-700">
        APICAP v3 · Portfolio Intelligence · {overviewKpis.totalStrategies} strategies
      </p>
    </div>
  );
}
