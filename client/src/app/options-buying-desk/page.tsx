"use client";

/**
 * Crypto Options Buying Desk — dashboard for the options (long-premium) paper
 * engine (50 BTC option-buying strategies: scalp / intraday / swing).
 *
 * Styling matches the TradingAI terminal, mirroring Crypto Options Selling:
 * clean white rounded-2xl cards, uppercase labels, large monospace numbers,
 * violet primary, amber CTA. Reads only through the session-gated
 * /api/options-buying proxy. Paper only — long calls/puts against the engine's
 * synthetic BTC chain, no real money.
 */

import { useCallback, useEffect, useState } from "react";

type StrategyStatus = {
  strategyId: number;
  name: string;
  category: string;
  optionType: string;
  rosterState: "ACTIVE" | "WATCHLIST" | "DISABLED";
  status: string;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  shadowTrades: number;
  shadowWins: number;
  shadowLosses: number;
  shadowPnl: number;
  shadowWinRate: number;
  score: number;
  regime: string;
  regimeFit: number;
  allocationUsd: number;
  sizeMultiplier: number;
  disableReason?: string;
  hasPosition: boolean;
  hasShadowPosition: boolean;
};

type OptionTrade = {
  id: string;
  strategyId: number;
  strategyName: string;
  optionType: string;
  strike: number;
  expiryMins: number;
  entryPremium: number;
  exitPremium: number;
  quantity: number;
  costBasis: number;
  netPnl: number;
  returnPct: number;
  entryBtcPrice: number;
  exitBtcPrice: number;
  entryTime: string;
  exitTime: string;
  exitReason: string;
};

type OptionPosition = {
  id: string;
  strategyId: number;
  strategyName: string;
  optionType: string;
  strike: number;
  expiryTime: string;
  entryPremium: number;
  currentPremium: number;
  quantity: number;
  costBasis: number;
  entryBtcPrice: number;
  entryTime: string;
  unrealizedPnl: number;
  peakGainPct: number;
  iv: number;
  delta: number;
};

type AggregateStats = {
  balance: number;
  equity: number;
  totalTrades: number;
  openPositions: number;
  totalWins: number;
  totalLosses: number;
  winRate: number;
  totalPnl: number;
  totalPremiumSpent: number;
  unrealizedPnl: number;
};

const CATEGORIES = ["ALL", "Momentum", "Mean Reversion", "Breakout", "Capitulation", "Hybrid"];
const ROSTER_STATES = ["ALL", "ACTIVE", "WATCHLIST", "DISABLED"] as const;

type SortKey = "totalTrades" | "winRate" | "totalPnl" | "score" | "allocationUsd";

const sora = { fontFamily: "var(--font-sora), system-ui, sans-serif" };
const mono = { fontFamily: "var(--font-jetbrains-mono), ui-monospace, SFMono-Regular, monospace" };

function fmtUSD(v: number): string {
  return `${v < 0 ? "-" : "+"}$${Math.abs(v).toFixed(2)}`;
}
function pnlClass(v: number): string {
  return v > 0 ? "text-emerald-600" : v < 0 ? "text-red-600" : "text-zinc-500";
}
function fmtPrice(v: number): string {
  return v.toLocaleString("en-US", { maximumFractionDigits: 0 });
}
function holdingBucket(name: string): string {
  if (name.startsWith("Scalp_")) return "Scalp";
  if (name.startsWith("Intraday_")) return "Intraday";
  if (name.startsWith("Swing_")) return "Swing";
  return "Other";
}

function StatCard({ label, value, sub, valueClass }: { label: string; value: string; sub?: React.ReactNode; valueClass?: string }) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-white px-6 py-5">
      <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-400">{label}</div>
      <div className={`mt-2.5 text-[32px] font-bold leading-none tabular-nums ${valueClass ?? "text-zinc-900"}`} style={mono}>
        {value}
      </div>
      {sub && <div className="mt-3 text-[12px]">{sub}</div>}
    </div>
  );
}

function CardHead({ title, right }: { title: string; right?: React.ReactNode }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 border-b border-zinc-100 px-6 py-4">
      <h2 className="text-[17px] font-bold tracking-tight text-zinc-900" style={sora}>{title}</h2>
      {right}
    </div>
  );
}

const selectPill = "cursor-pointer rounded-full border border-zinc-200 bg-white px-3.5 py-1.5 text-[12px] font-semibold text-zinc-700 hover:border-zinc-300";

function SortTh({
  label, k, sortKey, sortDir, onSort,
}: {
  label: string; k: SortKey; sortKey: SortKey; sortDir: "asc" | "desc"; onSort: (k: SortKey) => void;
}) {
  const active = sortKey === k;
  return (
    <th className="px-3 py-3 text-right font-bold">
      <button
        type="button"
        onClick={() => onSort(k)}
        className={`ml-auto inline-flex items-center gap-1 uppercase tracking-[0.08em] transition-colors ${
          active ? "text-violet-600" : "text-zinc-400 hover:text-zinc-700"
        }`}
      >
        {label}
        <span className="text-[8px] leading-none">{active ? (sortDir === "desc" ? "▼" : "▲") : "▲▼"}</span>
      </button>
    </th>
  );
}

function RosterBadge({ state }: { state: string }) {
  const cls =
    state === "ACTIVE"
      ? "bg-emerald-100 text-emerald-800"
      : state === "WATCHLIST"
        ? "bg-amber-50 text-amber-700"
        : "bg-zinc-100 text-zinc-500";
  return <span className={`inline-flex rounded-full px-2.5 py-0.5 font-sans text-[10px] font-bold ${cls}`}>{state}</span>;
}

export default function OptionsBuyingDeskPage() {
  const [stats, setStats] = useState<AggregateStats | null>(null);
  const [strategies, setStrategies] = useState<StrategyStatus[]>([]);
  const [positions, setPositions] = useState<OptionPosition[]>([]);
  const [trades, setTrades] = useState<OptionTrade[]>([]);
  const [category, setCategory] = useState<string>("ALL");
  const [rosterFilter, setRosterFilter] = useState<(typeof ROSTER_STATES)[number]>("ALL");
  const [query, setQuery] = useState<string>("");
  const [sortKey, setSortKey] = useState<SortKey>("score");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [error, setError] = useState<string>("");
  const [updatedAt, setUpdatedAt] = useState<string>("");

  const refresh = useCallback(async () => {
    try {
      const [s, st, p, tr] = await Promise.all([
        fetch("/api/options-buying/stats", { cache: "no-store" }),
        fetch("/api/options-buying/strategies", { cache: "no-store" }),
        fetch("/api/options-buying/positions", { cache: "no-store" }),
        fetch("/api/options-buying/trades", { cache: "no-store" }),
      ]);
      if (!s.ok || !st.ok || !p.ok || !tr.ok) {
        const bad = [s, st, p, tr].find((r) => !r.ok);
        setError(`desk unreachable (HTTP ${bad?.status})`);
        return;
      }
      setStats(await s.json());
      setStrategies((await st.json()) as StrategyStatus[]);
      setPositions((await p.json()) as OptionPosition[]);
      setTrades((await tr.json()) as OptionTrade[]);
      setError("");
      setUpdatedAt(new Date().toLocaleTimeString());
    } catch {
      setError("desk unreachable");
    }
  }, []);

  useEffect(() => {
    void refresh();
    const t = setInterval(() => void refresh(), 30_000);
    return () => clearInterval(t);
  }, [refresh]);

  const q = query.trim().toLowerCase();
  const filtered = strategies
    .filter((r) => category === "ALL" || r.category === category)
    .filter((r) => rosterFilter === "ALL" || r.rosterState === rosterFilter)
    .filter((r) => q === "" || r.name.toLowerCase().includes(q))
    .slice()
    .sort((a, b) => {
      const d = a[sortKey] - b[sortKey];
      return sortDir === "desc" ? -d : d;
    });
  const filtersActive = category !== "ALL" || rosterFilter !== "ALL" || q !== "";
  const resetFilters = () => {
    setCategory("ALL");
    setRosterFilter("ALL");
    setQuery("");
  };
  const toggleSort = (k: SortKey) => {
    if (sortKey === k) setSortDir((d) => (d === "desc" ? "asc" : "desc"));
    else {
      setSortKey(k);
      setSortDir("desc");
    }
  };

  const activeCount = strategies.filter((r) => r.rosterState === "ACTIVE").length;
  const categoryCounts = CATEGORIES.slice(1).map((c) => ({
    category: c,
    total: strategies.filter((r) => r.category === c).length,
    active: strategies.filter((r) => r.category === c && r.rosterState === "ACTIVE").length,
  }));
  const engineUp = strategies.length > 0 && !error;

  return (
    <main className="mx-auto flex max-w-6xl flex-col gap-6 px-8 py-8">
      {/* Breadcrumb + heading */}
      <div>
        <div className="flex items-center gap-1.5 text-[13px]">
          <a href="/terminal" className="text-zinc-400 underline-offset-2 hover:text-zinc-700 hover:underline">Home</a>
          <span className="text-zinc-300">›</span>
          <span className="font-medium text-zinc-600">Options Buying Desk</span>
        </div>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <h1 className="text-[36px] font-extrabold leading-tight tracking-tight text-zinc-900" style={sora}>
            Crypto Options Buying
          </h1>
          <span
            className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[11px] font-bold ${
              engineUp ? "bg-emerald-50 text-emerald-700" : "bg-red-50 text-red-700"
            }`}
          >
            <span className={`h-1.5 w-1.5 rounded-full ${engineUp ? "animate-pulse bg-emerald-500" : "bg-red-500"}`} />
            {strategies.length ? (engineUp ? "ENGINE UP" : "ENGINE DOWN") : "…"}
          </span>
        </div>
        <p className="mt-1.5 max-w-3xl text-[14px] leading-relaxed text-zinc-500">
          50 BTC long-premium strategies spanning scalp, intraday and swing holding periods — buying calls and puts
          against synthetic BTC option premiums, sized by a roster that rotates only the top strategies into live
          paper capital at a time. Paper trading only, no real money.
        </p>
      </div>

      {error && (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-5 py-3 text-[13px] font-medium text-amber-800">
          {error} — retrying every 30s
        </div>
      )}

      {/* Stat cards */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <StatCard
          label="Net P&L"
          value={stats ? fmtUSD(stats.totalPnl) : "—"}
          valueClass={pnlClass(stats?.totalPnl ?? 0)}
          sub={<span className="text-zinc-400">realized + unrealized {stats ? fmtUSD(stats.unrealizedPnl) : "—"}</span>}
        />
        <StatCard
          label="Closed Trades"
          value={stats ? String(stats.totalTrades) : "—"}
          sub={
            <span className="font-semibold text-emerald-600" style={mono}>
              {stats ? `win rate ${stats.winRate.toFixed(1)}%` : "win rate —"}
            </span>
          }
        />
        <StatCard
          label="Open Positions"
          value={stats ? String(stats.openPositions) : "—"}
          sub={<span className="text-zinc-400">long option positions live</span>}
        />
        <StatCard
          label="Active Roster"
          value={strategies.length ? `${activeCount} / ${strategies.length}` : "—"}
          sub={<span className="text-zinc-400">of {strategies.length} strategies, capped per category</span>}
        />
      </div>

      {/* Category mix */}
      <section className="rounded-2xl border border-zinc-200 bg-white">
        <CardHead title="Category Mix" right={<span className="text-[12px] text-zinc-400" style={mono}>{updatedAt ? `computed ${updatedAt}` : "—"}</span>} />
        <div className="grid grid-cols-2 gap-x-8 gap-y-6 px-6 py-6 md:grid-cols-5">
          {categoryCounts.map((c) => (
            <div key={c.category}>
              <div className="text-[10.5px] font-semibold uppercase tracking-wider text-zinc-400">{c.category}</div>
              <div className="mt-1.5 text-[19px] font-bold tabular-nums text-zinc-900" style={mono}>
                {c.total}
              </div>
              <div className="mt-1 text-[10px] leading-snug text-zinc-400">{c.active} active</div>
            </div>
          ))}
        </div>
      </section>

      {/* Strategy leaderboard */}
      <section className="rounded-2xl border border-zinc-200 bg-white">
        <CardHead
          title="Strategy Leaderboard"
          right={<span className="text-[12px] font-medium text-zinc-400" style={mono}>{filtered.length} of {strategies.length} strategies</span>}
        />

        <div className="flex flex-wrap items-center gap-2.5 border-b border-zinc-100 px-6 py-3.5">
          <div className="relative">
            <svg
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400"
              width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"
            >
              <circle cx="11" cy="11" r="8" /><path d="m21 21-4.3-4.3" />
            </svg>
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search strategy…"
              className="w-[220px] rounded-full border border-zinc-200 bg-white py-1.5 pl-9 pr-3 text-[12px] font-medium text-zinc-700 placeholder:text-zinc-400 focus:border-violet-400 focus:outline-none"
            />
          </div>

          <select value={category} onChange={(e) => setCategory(e.target.value)} className={selectPill}>
            {CATEGORIES.map((c) => (
              <option key={c} value={c}>{c === "ALL" ? "All categories" : c}</option>
            ))}
          </select>

          <div className="inline-flex rounded-full border border-zinc-200 p-0.5">
            {ROSTER_STATES.map((s) => (
              <button
                key={s}
                type="button"
                onClick={() => setRosterFilter(s)}
                className={`rounded-full px-3 py-1 text-[11.5px] font-bold transition-colors ${
                  rosterFilter === s
                    ? s === "ACTIVE"
                      ? "bg-emerald-100 text-emerald-700"
                      : s === "WATCHLIST"
                        ? "bg-amber-100 text-amber-700"
                        : s === "DISABLED"
                          ? "bg-red-100 text-red-700"
                          : "bg-violet-100 text-violet-700"
                    : "text-zinc-500 hover:text-zinc-800"
                }`}
              >
                {s === "ALL" ? "All" : s.charAt(0) + s.slice(1).toLowerCase()}
              </button>
            ))}
          </div>

          {filtersActive && (
            <button type="button" onClick={resetFilters} className="ml-auto rounded-full px-3 py-1.5 text-[12px] font-semibold text-violet-600 hover:text-violet-800">
              Reset
            </button>
          )}
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-[13px]">
            <thead>
              <tr className="border-b border-zinc-100 text-left text-[10.5px] uppercase tracking-[0.08em] text-zinc-400">
                <th className="px-6 py-3 font-bold">Strategy</th>
                <th className="px-3 py-3 font-bold">Bucket</th>
                <th className="px-3 py-3 font-bold">Category</th>
                <th className="px-3 py-3 font-bold">Type</th>
                <SortTh label="Trades" k="totalTrades" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortTh label="WR %" k="winRate" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortTh label="Net $" k="totalPnl" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortTh label="Score" k="score" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortTh label="Alloc $" k="allocationUsd" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <th className="px-6 py-3 text-right font-bold">Roster</th>
              </tr>
            </thead>
            <tbody style={mono}>
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={10} className="px-6 py-12 text-center font-sans text-[13.5px] text-zinc-400">
                    {strategies.length === 0 ? "No strategy data yet — the engine trades 24/7; check back soon." : "No strategies match these filters."}
                  </td>
                </tr>
              )}
              {filtered.map((r) => (
                <tr key={r.strategyId} className="border-b border-zinc-50 transition-colors hover:bg-zinc-50">
                  <td className="px-6 py-2.5 font-sans font-semibold text-zinc-800">{r.name}</td>
                  <td className="px-3 py-2.5 text-zinc-500">{holdingBucket(r.name)}</td>
                  <td className="px-3 py-2.5 text-zinc-500">{r.category}</td>
                  <td className="px-3 py-2.5">
                    <span className={`text-[11px] font-bold ${r.optionType === "CALL" ? "text-emerald-600" : "text-red-600"}`}>{r.optionType}</span>
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{r.totalTrades}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{r.winRate.toFixed(1)}</td>
                  <td className={`px-3 py-2.5 text-right font-semibold tabular-nums ${pnlClass(r.totalPnl)}`}>{fmtUSD(r.totalPnl)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{r.score.toFixed(1)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-500">{r.allocationUsd.toFixed(0)}</td>
                  <td className="px-6 py-2.5 text-right"><RosterBadge state={r.rosterState} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* Open positions */}
      <section className="rounded-2xl border border-zinc-200 bg-white">
        <CardHead title="Open Positions" right={<span className="text-[12px] font-medium text-zinc-400" style={mono}>{positions.length} live</span>} />
        <div className="overflow-x-auto">
          <table className="w-full text-[13px]">
            <thead>
              <tr className="border-b border-zinc-100 text-left text-[10.5px] uppercase tracking-[0.08em] text-zinc-400">
                <th className="px-6 py-3 font-bold">Strategy</th>
                <th className="px-3 py-3 font-bold">Type</th>
                <th className="px-3 py-3 text-right font-bold">Strike</th>
                <th className="px-3 py-3 text-right font-bold">Entry Premium</th>
                <th className="px-3 py-3 text-right font-bold">Current Premium</th>
                <th className="px-3 py-3 text-right font-bold">Cost Basis</th>
                <th className="px-6 py-3 text-right font-bold">Unrealized</th>
              </tr>
            </thead>
            <tbody style={mono}>
              {positions.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-6 py-12 text-center font-sans text-[13.5px] text-zinc-400">
                    No open positions right now.
                  </td>
                </tr>
              )}
              {positions.map((p) => (
                <tr key={p.id} className="border-b border-zinc-50 transition-colors hover:bg-zinc-50">
                  <td className="px-6 py-2.5 font-sans font-semibold text-zinc-800">{p.strategyName}</td>
                  <td className="px-3 py-2.5">
                    <span className={`text-[11px] font-bold ${p.optionType === "CALL" ? "text-emerald-600" : "text-red-600"}`}>{p.optionType}</span>
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{fmtPrice(p.strike)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{p.entryPremium.toFixed(2)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{p.currentPremium.toFixed(2)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-500">{fmtUSD(p.costBasis)}</td>
                  <td className={`px-6 py-2.5 text-right font-semibold tabular-nums ${pnlClass(p.unrealizedPnl)}`}>{fmtUSD(p.unrealizedPnl)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* Recent trades */}
      <section className="rounded-2xl border border-zinc-200 bg-white">
        <CardHead title="Recent Trades" right={<span className="text-[12px] font-medium text-zinc-400" style={mono}>last {trades.length}</span>} />
        <div className="overflow-x-auto">
          <table className="w-full text-[13px]">
            <thead>
              <tr className="border-b border-zinc-100 text-left text-[10.5px] uppercase tracking-[0.08em] text-zinc-400">
                <th className="px-6 py-3 font-bold">Exit Time (UTC)</th>
                <th className="px-3 py-3 font-bold">Strategy</th>
                <th className="px-3 py-3 font-bold">Type</th>
                <th className="px-3 py-3 text-right font-bold">Strike</th>
                <th className="px-3 py-3 text-right font-bold">Entry</th>
                <th className="px-3 py-3 text-right font-bold">Exit</th>
                <th className="px-3 py-3 font-bold">Reason</th>
                <th className="px-3 py-3 text-right font-bold">Ret %</th>
                <th className="px-6 py-3 text-right font-bold">P&L</th>
              </tr>
            </thead>
            <tbody style={mono}>
              {trades.length === 0 && (
                <tr>
                  <td colSpan={9} className="px-6 py-12 text-center font-sans text-[13.5px] text-zinc-400">
                    No closed trades yet — the desk trades 24/7; check back soon.
                  </td>
                </tr>
              )}
              {trades.slice(0, 150).map((t) => (
                <tr key={t.id} className="border-b border-zinc-50 transition-colors hover:bg-zinc-50">
                  <td className="px-6 py-2.5 tabular-nums text-zinc-500">{new Date(t.exitTime).toISOString().slice(5, 16).replace("T", " ")}</td>
                  <td className="px-3 py-2.5 font-sans font-semibold text-zinc-800">{t.strategyName}</td>
                  <td className="px-3 py-2.5">
                    <span className={`text-[11px] font-bold ${t.optionType === "CALL" ? "text-emerald-600" : "text-red-600"}`}>{t.optionType}</span>
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{fmtPrice(t.strike)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{t.entryPremium.toFixed(2)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{t.exitPremium.toFixed(2)}</td>
                  <td className="px-3 py-2.5">
                    <span className={`inline-flex rounded-full px-2.5 py-0.5 font-sans text-[10px] font-bold ${
                      t.exitReason === "TP" ? "bg-emerald-50 text-emerald-700" : t.exitReason === "SL" ? "bg-red-50 text-red-700" : "bg-zinc-100 text-zinc-500"
                    }`}>{t.exitReason}</span>
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-500">{(t.returnPct * 100).toFixed(1)}</td>
                  <td className={`px-6 py-2.5 text-right font-semibold tabular-nums ${pnlClass(t.netPnl)}`}>{fmtUSD(t.netPnl)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <p className="pb-2 text-center text-[11px] text-zinc-400">
        Paper trading only · long premium against synthetic BTC option chain · top strategies rotated live at a
        time · state persists on the engine host
      </p>
    </main>
  );
}
