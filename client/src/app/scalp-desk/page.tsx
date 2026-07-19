"use client";

/**
 * Crypto Scalp Desk — dashboard for the scalp_prelive paper engine
 * (100 x 1m scalp strategies x 8 symbols = 800 live paper streams).
 *
 * Styling matches the TradingAI terminal: stacked breadcrumb + heavy heading
 * + description, clean white rounded-2xl cards with uppercase labels and
 * large monospace numbers, violet primary, amber CTA. Reads only through the
 * session-gated /api/scalp proxy. Paper only: the PRE-REGISTERED gate is the
 * sole route to a real-money discussion.
 */

import { useCallback, useEffect, useState } from "react";

type Stats = {
  trades: number;
  wins: number;
  missed_fills: number;
  open_positions: number;
  pending_orders: number;
  net_pnl_usd_at_100_notional: number;
  trades_per_symbol: Record<string, number>;
};
type Health = { ok: boolean; uptime_min: number; bars_processed: number; strategies: number; streams: number };
type LbRow = {
  strategy: string; symbol: string; n: number; wr_pct: number; pf: number;
  net_usd: number; max_dd_pct: number; missed: number; gate_pass: boolean;
};
type Trade = {
  time: string; symbol: string; strategy: string; dir: string; entry: number; exit: number;
  reason: string; ret_net: number; pnl_usd: number; profile: string; hold_min: number;
};

const SYMBOLS = ["ALL", "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "AVAXUSDT"];
const GATE_RULES = ["≥ 200 live trades", "PF ≥ 1.2", "max DD ≤ 25%", "both halves net-positive"];
const MIN_N_OPTS = [0, 5, 20, 50, 100, 200];

type SortKey = "n" | "wr_pct" | "pf" | "net_usd" | "max_dd_pct" | "missed";
type Side = "ALL" | "LONG" | "SHORT";

const sora = { fontFamily: "var(--font-sora), system-ui, sans-serif" };
const mono = { fontFamily: "var(--font-jetbrains-mono), ui-monospace, SFMono-Regular, monospace" };

function fmtUSD(v: number): string {
  return `${v < 0 ? "-" : "+"}$${Math.abs(v).toFixed(2)}`;
}
function pnlClass(v: number): string {
  return v > 0 ? "text-emerald-600" : v < 0 ? "text-red-600" : "text-zinc-500";
}
function fmtPrice(v: number): string {
  if (v >= 1000) return v.toLocaleString("en-US", { maximumFractionDigits: 1 });
  if (v >= 1) return v.toFixed(3);
  return v.toFixed(5);
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

function Metric({ label, value, valueClass, hint }: { label: string; value: string; valueClass?: string; hint?: string }) {
  return (
    <div>
      <div className="text-[10.5px] font-semibold uppercase tracking-wider text-zinc-400">{label}</div>
      <div className={`mt-1.5 text-[19px] font-bold tabular-nums ${valueClass ?? "text-zinc-900"}`} style={mono}>
        {value}
      </div>
      {hint && <div className="mt-1 text-[10px] leading-snug text-zinc-400">{hint}</div>}
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

const pillBase =
  "cursor-pointer rounded-full border px-3.5 py-1.5 text-[12px] font-semibold transition-colors";
const selectPill =
  "cursor-pointer rounded-full border border-zinc-200 bg-white px-3.5 py-1.5 text-[12px] font-semibold text-zinc-700 hover:border-zinc-300";

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

export default function ScalpDeskPage() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [health, setHealth] = useState<Health | null>(null);
  const [rows, setRows] = useState<LbRow[]>([]);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [symbol, setSymbol] = useState<string>("ALL");
  const [query, setQuery] = useState<string>("");
  const [side, setSide] = useState<Side>("ALL");
  const [minN, setMinN] = useState<number>(0);
  const [gateOnly, setGateOnly] = useState<boolean>(false);
  const [profitOnly, setProfitOnly] = useState<boolean>(false);
  const [sortKey, setSortKey] = useState<SortKey>("net_usd");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [error, setError] = useState<string>("");
  const [updatedAt, setUpdatedAt] = useState<string>("");

  const refresh = useCallback(async () => {
    try {
      const [h, s, lb, tr] = await Promise.all([
        fetch("/api/scalp/scalp/health", { cache: "no-store" }),
        fetch("/api/scalp/scalp/stats", { cache: "no-store" }),
        fetch("/api/scalp/scalp/leaderboard", { cache: "no-store" }),
        fetch("/api/scalp/scalp/trades?n=50", { cache: "no-store" }),
      ]);
      if (!h.ok || !s.ok || !lb.ok || !tr.ok) {
        const bad = [h, s, lb, tr].find((r) => !r.ok);
        setError(`desk unreachable (HTTP ${bad?.status})`);
        return;
      }
      setHealth(await h.json());
      setStats(await s.json());
      setRows(((await lb.json()) as { rows: LbRow[] }).rows ?? []);
      setTrades((await tr.json()) as Trade[]);
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
  const filtered = rows
    .filter((r) => symbol === "ALL" || r.symbol === symbol)
    .filter((r) => r.n >= minN)
    .filter((r) => !gateOnly || r.gate_pass)
    .filter((r) => !profitOnly || r.net_usd > 0)
    .filter((r) =>
      side === "ALL" ? true : side === "LONG" ? r.strategy.endsWith("_Long") : r.strategy.endsWith("_Short"),
    )
    .filter((r) => q === "" || r.strategy.toLowerCase().includes(q) || r.symbol.toLowerCase().includes(q))
    .slice()
    .sort((a, b) => {
      const d = a[sortKey] - b[sortKey];
      return sortDir === "desc" ? -d : d;
    });
  const filtersActive =
    symbol !== "ALL" || q !== "" || side !== "ALL" || minN !== 0 || gateOnly || profitOnly;
  const resetFilters = () => {
    setSymbol("ALL");
    setQuery("");
    setSide("ALL");
    setMinN(0);
    setGateOnly(false);
    setProfitOnly(false);
  };
  const toggleSort = (k: SortKey) => {
    if (sortKey === k) setSortDir((d) => (d === "desc" ? "asc" : "desc"));
    else {
      setSortKey(k);
      setSortDir("desc");
    }
  };
  const gatePassers = rows.filter((r) => r.gate_pass);
  const winRate = stats && stats.trades > 0 ? (100 * stats.wins) / stats.trades : null;
  const fillRate =
    stats && stats.trades + stats.missed_fills > 0
      ? (100 * stats.trades) / (stats.trades + stats.missed_fills)
      : null;

  return (
    <main className="mx-auto flex max-w-6xl flex-col gap-6 px-8 py-8">
      {/* Breadcrumb + heading (TradingAI style) */}
      <div>
        <div className="flex items-center gap-1.5 text-[13px]">
          <a href="/terminal" className="text-zinc-400 underline-offset-2 hover:text-zinc-700 hover:underline">Home</a>
          <span className="text-zinc-300">›</span>
          <span className="font-medium text-zinc-600">Scalp Desk</span>
        </div>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <h1 className="text-[36px] font-extrabold leading-tight tracking-tight text-zinc-900" style={sora}>
            Scalp Desk
          </h1>
          <span
            className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[11px] font-bold ${
              health?.ok ? "bg-emerald-50 text-emerald-700" : "bg-red-50 text-red-700"
            }`}
          >
            <span className={`h-1.5 w-1.5 rounded-full ${health?.ok ? "animate-pulse bg-emerald-500" : "bg-red-500"}`} />
            {health ? (health.ok ? "ENGINE UP" : "ENGINE DOWN") : "…"}
          </span>
        </div>
        <p className="mt-1.5 max-w-3xl text-[14px] leading-relaxed text-zinc-500">
          100 one-minute strategies across 8 major cryptos — 800 live paper streams from your scalp engine on AWS, with
          a maker-fill model identical to the backtest. Real money only via the pre-registered gate.
        </p>
      </div>

      {error && (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-5 py-3 text-[13px] font-medium text-amber-800">
          {error} — retrying every 30s
        </div>
      )}

      {/* Stat cards */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <StatCard
          label="Net P&L"
          value={stats ? fmtUSD(stats.net_pnl_usd_at_100_notional) : "—"}
          valueClass={pnlClass(stats?.net_pnl_usd_at_100_notional ?? 0)}
          sub={<span className="text-zinc-400">$100 notional per trade</span>}
        />
        <StatCard
          label="Closed Trades"
          value={stats ? String(stats.trades) : "—"}
          sub={
            <span className="font-semibold text-emerald-600" style={mono}>
              {winRate === null ? "win rate —" : `win rate ${winRate.toFixed(1)}%`}
            </span>
          }
        />
        <StatCard
          label="Open / Pending"
          value={stats ? `${stats.open_positions} / ${stats.pending_orders}` : "—"}
          sub={<span className="text-zinc-400">positions / resting post-only orders</span>}
        />
      </div>

      {/* Desk analytics */}
      <section className="rounded-2xl border border-zinc-200 bg-white">
        <CardHead
          title="Desk Analytics"
          right={<span className="text-[12px] text-zinc-400" style={mono}>{updatedAt ? `computed ${updatedAt}` : "—"}</span>}
        />
        <div className="grid grid-cols-2 gap-x-8 gap-y-6 px-6 py-6 md:grid-cols-6">
          <Metric label="Engine" value={health ? (health.ok ? "UP" : "DOWN") : "—"} valueClass={health?.ok ? "text-emerald-600" : "text-red-600"} />
          <Metric label="Uptime" value={health ? `${Math.floor(health.uptime_min / 60)}h ${health.uptime_min % 60}m` : "—"} />
          <Metric label="Streams" value={health?.streams != null ? String(health.streams) : "—"} />
          <Metric label="Bars Processed" value={health?.bars_processed != null ? health.bars_processed.toLocaleString() : "—"} />
          <Metric
            label="Maker Fill Rate"
            value={fillRate === null ? "—" : `${fillRate.toFixed(0)}%`}
            hint="filled vs missed post-only entries — missed fills counted, never assumed"
          />
          <Metric label="Gate Passers" value={rows.length ? String(gatePassers.length) : "—"} valueClass={gatePassers.length > 0 ? "text-emerald-600" : "text-zinc-900"} />
        </div>
      </section>

      {/* Promotion gate */}
      <section className="rounded-2xl border border-zinc-200 bg-white">
        <CardHead
          title="Promotion Gate"
          right={
            <span className={`inline-flex items-center rounded-full px-3 py-1 text-[11px] font-bold ${gatePassers.length > 0 ? "bg-emerald-100 text-emerald-800" : "bg-zinc-100 text-zinc-500"}`}>
              {rows.length ? `${gatePassers.length} of ${rows.length} streams pass` : "—"}
            </span>
          }
        />
        <div className="px-6 py-5">
          <div className="flex flex-wrap items-center gap-2">
            {GATE_RULES.map((g) => (
              <span key={g} className="inline-flex items-center gap-2 rounded-full bg-violet-50 px-3.5 py-1.5 text-[12.5px] font-semibold text-violet-700">
                <span className="h-1.5 w-1.5 rounded-full bg-violet-500" />
                {g}
              </span>
            ))}
          </div>
          <p className="mt-3.5 max-w-3xl text-[12.5px] leading-relaxed text-zinc-500">
            Pre-registered before the first live trade: all 100 strategies failed offline qualification (0/400), so with
            800 streams a few days of trading is expected to produce lucky leaders by variance alone. Leaderboard
            position alone never justifies real money — only gate survivors earn a go-live discussion.
          </p>
        </div>
      </section>

      {/* Leaderboard */}
      <section className="rounded-2xl border border-zinc-200 bg-white">
        <CardHead
          title="Strategy Leaderboard"
          right={
            <span className="text-[12px] font-medium text-zinc-400" style={mono}>
              {filtered.length} of {rows.length} streams
            </span>
          }
        />

        {/* Filter toolbar */}
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
              className="w-[190px] rounded-full border border-zinc-200 bg-white py-1.5 pl-9 pr-3 text-[12px] font-medium text-zinc-700 placeholder:text-zinc-400 focus:border-violet-400 focus:outline-none"
            />
          </div>

          <select value={symbol} onChange={(e) => setSymbol(e.target.value)} className={selectPill}>
            {SYMBOLS.map((s) => (
              <option key={s} value={s}>{s === "ALL" ? "All symbols" : s.replace("USDT", "")}</option>
            ))}
          </select>

          {/* Direction segmented control */}
          <div className="inline-flex rounded-full border border-zinc-200 p-0.5">
            {(["ALL", "LONG", "SHORT"] as Side[]).map((s) => (
              <button
                key={s}
                type="button"
                onClick={() => setSide(s)}
                className={`rounded-full px-3 py-1 text-[11.5px] font-bold transition-colors ${
                  side === s
                    ? s === "LONG"
                      ? "bg-emerald-100 text-emerald-700"
                      : s === "SHORT"
                        ? "bg-red-100 text-red-700"
                        : "bg-violet-100 text-violet-700"
                    : "text-zinc-500 hover:text-zinc-800"
                }`}
              >
                {s === "ALL" ? "Both" : s === "LONG" ? "Long" : "Short"}
              </button>
            ))}
          </div>

          <select value={minN} onChange={(e) => setMinN(Number(e.target.value))} className={selectPill}>
            {MIN_N_OPTS.map((n) => (
              <option key={n} value={n}>{n === 0 ? "Any trades" : `≥ ${n} trades`}</option>
            ))}
          </select>

          <button
            type="button"
            onClick={() => setGateOnly((v) => !v)}
            className={`${pillBase} ${gateOnly ? "border-emerald-300 bg-emerald-50 text-emerald-700" : "border-zinc-200 text-zinc-600 hover:border-zinc-300"}`}
          >
            Gate pass only
          </button>
          <button
            type="button"
            onClick={() => setProfitOnly((v) => !v)}
            className={`${pillBase} ${profitOnly ? "border-emerald-300 bg-emerald-50 text-emerald-700" : "border-zinc-200 text-zinc-600 hover:border-zinc-300"}`}
          >
            Profitable only
          </button>

          {filtersActive && (
            <button
              type="button"
              onClick={resetFilters}
              className="ml-auto rounded-full px-3 py-1.5 text-[12px] font-semibold text-violet-600 hover:text-violet-800"
            >
              Reset
            </button>
          )}
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-[13px]">
            <thead>
              <tr className="border-b border-zinc-100 text-left text-[10.5px] uppercase tracking-[0.08em] text-zinc-400">
                <th className="px-6 py-3 font-bold">Strategy</th>
                <th className="px-3 py-3 font-bold">Symbol</th>
                <SortTh label="Trades" k="n" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortTh label="WR %" k="wr_pct" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortTh label="PF" k="pf" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortTh label="Net $" k="net_usd" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortTh label="Max DD %" k="max_dd_pct" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <SortTh label="Missed" k="missed" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />
                <th className="px-6 py-3 text-right font-bold">Gate</th>
              </tr>
            </thead>
            <tbody style={mono}>
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={9} className="px-6 py-12 text-center font-sans text-[13.5px] text-zinc-400">
                    {rows.length === 0
                      ? "No closed trades yet — the desk trades 24/7; check back soon."
                      : "No streams match these filters."}
                  </td>
                </tr>
              )}
              {filtered.slice(0, 150).map((r) => (
                <tr key={`${r.strategy}|${r.symbol}`} className="border-b border-zinc-50 transition-colors hover:bg-zinc-50">
                  <td className="px-6 py-2.5 font-sans font-semibold text-zinc-800">{r.strategy}</td>
                  <td className="px-3 py-2.5 text-zinc-500">{r.symbol.replace("USDT", "")}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{r.n}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{r.wr_pct.toFixed(1)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{r.pf.toFixed(2)}</td>
                  <td className={`px-3 py-2.5 text-right font-semibold tabular-nums ${pnlClass(r.net_usd)}`}>{fmtUSD(r.net_usd)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{r.max_dd_pct.toFixed(1)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-500">{r.missed}</td>
                  <td className="px-6 py-2.5 text-right">
                    {r.gate_pass ? (
                      <span className="inline-flex rounded-full bg-emerald-100 px-2.5 py-0.5 font-sans text-[10px] font-bold text-emerald-800">PASS</span>
                    ) : (
                      <span className="inline-flex rounded-full bg-zinc-100 px-2.5 py-0.5 font-sans text-[10px] font-semibold text-zinc-400">—</span>
                    )}
                  </td>
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
                <th className="px-6 py-3 font-bold">Time (UTC)</th>
                <th className="px-3 py-3 font-bold">Symbol</th>
                <th className="px-3 py-3 font-bold">Strategy</th>
                <th className="px-3 py-3 font-bold">Dir</th>
                <th className="px-3 py-3 text-right font-bold">Entry</th>
                <th className="px-3 py-3 text-right font-bold">Exit</th>
                <th className="px-3 py-3 font-bold">Reason</th>
                <th className="px-3 py-3 text-right font-bold">Hold</th>
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
              {[...trades].reverse().map((t, i) => (
                <tr key={`${t.time}-${t.strategy}-${i}`} className="border-b border-zinc-50 transition-colors hover:bg-zinc-50">
                  <td className="px-6 py-2.5 tabular-nums text-zinc-500">{new Date(t.time).toISOString().slice(5, 16).replace("T", " ")}</td>
                  <td className="px-3 py-2.5 text-zinc-700">{t.symbol.replace("USDT", "")}</td>
                  <td className="px-3 py-2.5 font-sans font-semibold text-zinc-800">{t.strategy}</td>
                  <td className="px-3 py-2.5">
                    <span className={`text-[11px] font-bold ${t.dir === "LONG" ? "text-emerald-600" : "text-red-600"}`}>{t.dir}</span>
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{fmtPrice(t.entry)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{fmtPrice(t.exit)}</td>
                  <td className="px-3 py-2.5">
                    <span className={`inline-flex rounded-full px-2.5 py-0.5 font-sans text-[10px] font-bold ${
                      t.reason === "TP" ? "bg-emerald-50 text-emerald-700" : t.reason === "SL" ? "bg-red-50 text-red-700" : "bg-zinc-100 text-zinc-500"
                    }`}>{t.reason}</span>
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-500">{t.hold_min}m</td>
                  <td className={`px-6 py-2.5 text-right font-semibold tabular-nums ${pnlClass(t.pnl_usd)}`}>{fmtUSD(t.pnl_usd)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <p className="pb-2 text-center text-[11px] text-zinc-400">
        Paper trading only · $100 notional per trade · maker post-only fill model with missed fills counted · state
        persists on the engine host
      </p>
    </main>
  );
}
