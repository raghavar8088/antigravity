"use client";

/**
 * Crypto Scalp Desk — dashboard for the scalp_prelive paper engine
 * (100 x 1m scalp strategies x 8 symbols = 800 live paper streams).
 *
 * Visual language: JioFinance (heavy display headings, generous whitespace,
 * warm gold accent, pill controls) on the TradingAI dashboard structure.
 * Reads exclusively through the session-gated /api/scalp proxy. Paper only:
 * the PRE-REGISTERED gate is the sole route to a real-money discussion.
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

type Health = {
  ok: boolean;
  uptime_min: number;
  bars_processed: number;
  strategies: number;
  streams: number;
};

type LbRow = {
  strategy: string;
  symbol: string;
  n: number;
  wr_pct: number;
  pf: number;
  net_usd: number;
  max_dd_pct: number;
  missed: number;
  gate_pass: boolean;
};

type Trade = {
  time: string;
  symbol: string;
  strategy: string;
  dir: string;
  entry: number;
  exit: number;
  reason: string;
  ret_net: number;
  pnl_usd: number;
  profile: string;
  hold_min: number;
};

const SYMBOLS = ["ALL", "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "AVAXUSDT"];

const GATE_RULES = ["≥ 200 live trades", "PF ≥ 1.2", "max DD ≤ 25%", "both halves net-positive"];

const sora = { fontFamily: "var(--font-sora), system-ui, sans-serif" };
const mono = { fontFamily: "var(--font-jetbrains-mono), ui-monospace, SFMono-Regular, monospace" };

function fmtUSD(v: number): string {
  const sign = v < 0 ? "-" : "+";
  return `${sign}$${Math.abs(v).toFixed(2)}`;
}

function pnlClass(v: number): string {
  if (v > 0) return "text-emerald-600";
  if (v < 0) return "text-red-600";
  return "text-zinc-500";
}

function fmtPrice(v: number): string {
  if (v >= 1000) return v.toLocaleString("en-US", { maximumFractionDigits: 1 });
  if (v >= 1) return v.toFixed(3);
  return v.toFixed(5);
}

function Chip({ tone, children }: { tone: "ok" | "bad" | "muted"; children: React.ReactNode }) {
  const cls =
    tone === "ok"
      ? "bg-emerald-50 text-emerald-700"
      : tone === "bad"
        ? "bg-red-50 text-red-700"
        : "bg-zinc-100 text-zinc-600";
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-3.5 py-1.5 text-[12px] font-semibold ${cls}`}>
      {children}
    </span>
  );
}

export default function ScalpDeskPage() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [health, setHealth] = useState<Health | null>(null);
  const [rows, setRows] = useState<LbRow[]>([]);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [symbol, setSymbol] = useState<string>("ALL");
  const [minTrades, setMinTrades] = useState<boolean>(false);
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

  const filtered = rows
    .filter((r) => symbol === "ALL" || r.symbol === symbol)
    .filter((r) => !minTrades || r.n >= 20);
  const gatePassers = rows.filter((r) => r.gate_pass);
  const winRate = stats && stats.trades > 0 ? (100 * stats.wins) / stats.trades : null;
  const fillRate =
    stats && stats.trades + stats.missed_fills > 0
      ? (100 * stats.trades) / (stats.trades + stats.missed_fills)
      : null;

  return (
    <main className="mx-auto flex max-w-6xl flex-col gap-10 px-8 py-10">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-[14px]">
        <a href="/terminal" className="font-semibold text-amber-700 hover:text-amber-800">
          Home
        </a>
        <span className="text-zinc-300">›</span>
        <span className="font-semibold text-zinc-700">Scalp Desk</span>
      </div>

      {/* Hero — two-column like JioFinance product pages */}
      <div className="grid items-start gap-8 lg:grid-cols-[1.05fr_1fr]">
        <h1 className="text-[44px] font-extrabold leading-[1.08] tracking-tight text-zinc-900" style={sora}>
          Crypto Scalp Desk
        </h1>
        <div className="flex flex-col gap-4 lg:pt-2">
          <p className="text-[15.5px] leading-relaxed text-zinc-600">
            100 one-minute strategies across 8 major cryptos — 800 live paper streams trading around the clock on your
            AWS engine, with a maker-fill model identical to the backtest. Every stream must survive the pre-registered
            gate before real money is even discussed.
          </p>
          <div className="flex flex-wrap items-center gap-2">
            <Chip tone={health?.ok ? "ok" : "bad"}>
              <span className={`h-1.5 w-1.5 rounded-full ${health?.ok ? "animate-pulse bg-emerald-500" : "bg-red-500"}`} />
              Engine {health ? (health.ok ? "UP" : "DOWN") : "…"}
            </Chip>
            <Chip tone="muted">
              {health ? `${Math.floor(health.uptime_min / 60)}h ${health.uptime_min % 60}m uptime` : "—"}
            </Chip>
            <Chip tone="muted">{health ? `${health.streams} streams` : "—"}</Chip>
            {updatedAt && <Chip tone="muted">updated {updatedAt}</Chip>}
          </div>
          <div>
            <a
              href="/api/scalp/scalp/leaderboard"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-2 rounded-full bg-amber-500 px-6 py-3 text-[14px] font-bold text-white shadow-sm transition-colors hover:bg-amber-600"
            >
              View raw leaderboard
              <span className="text-[11px]">↗</span>
            </a>
          </div>
        </div>
      </div>

      {error && (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-6 py-3.5 text-[13.5px] font-medium text-amber-800">
          {error} — retrying every 30s
        </div>
      )}

      {/* Big stat cards */}
      <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
        <div className="rounded-3xl border border-zinc-200 bg-white px-8 py-7 shadow-[0_1px_3px_rgba(0,0,0,0.04)]">
          <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-zinc-400">Net P&amp;L</div>
          <div
            className={`mt-3 text-[38px] font-bold leading-none tabular-nums ${pnlClass(stats?.net_pnl_usd_at_100_notional ?? 0)}`}
            style={mono}
          >
            {stats ? fmtUSD(stats.net_pnl_usd_at_100_notional) : "—"}
          </div>
          <div className="mt-3 text-[12.5px] text-zinc-400">$100 notional per trade</div>
        </div>
        <div className="rounded-3xl border border-zinc-200 bg-white px-8 py-7 shadow-[0_1px_3px_rgba(0,0,0,0.04)]">
          <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-zinc-400">Closed Trades</div>
          <div className="mt-3 text-[38px] font-bold leading-none tabular-nums text-zinc-900" style={mono}>
            {stats?.trades ?? "—"}
          </div>
          <div className="mt-3 text-[12.5px] font-semibold text-emerald-600" style={mono}>
            {winRate === null ? "win rate —" : `win rate ${winRate.toFixed(1)}%`}
          </div>
        </div>
        <div className="rounded-3xl border border-zinc-200 bg-white px-8 py-7 shadow-[0_1px_3px_rgba(0,0,0,0.04)]">
          <div className="text-[11px] font-bold uppercase tracking-[0.16em] text-zinc-400">Open / Pending</div>
          <div className="mt-3 text-[38px] font-bold leading-none tabular-nums text-zinc-900" style={mono}>
            {stats ? `${stats.open_positions} / ${stats.pending_orders}` : "—"}
          </div>
          <div className="mt-3 text-[12.5px] text-zinc-400">positions / resting post-only orders</div>
        </div>
      </div>

      {/* Desk analytics */}
      <section className="rounded-3xl border border-zinc-200 bg-white shadow-[0_1px_3px_rgba(0,0,0,0.04)]">
        <div className="flex items-center justify-between border-b border-zinc-100 px-8 py-5">
          <h2 className="text-[21px] font-bold tracking-tight text-zinc-900" style={sora}>
            Desk Analytics
          </h2>
          <span className="text-[12px] text-zinc-400" style={mono}>
            {updatedAt ? `computed ${updatedAt}` : "—"}
          </span>
        </div>
        <div className="grid grid-cols-2 gap-x-8 gap-y-6 px-8 py-6 md:grid-cols-6">
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Engine</div>
            <div className={`mt-1.5 text-[22px] font-bold ${health?.ok ? "text-emerald-600" : "text-red-600"}`} style={mono}>
              {health ? (health.ok ? "UP" : "DOWN") : "—"}
            </div>
          </div>
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Uptime</div>
            <div className="mt-1.5 text-[22px] font-bold text-zinc-900" style={mono}>
              {health ? `${Math.floor(health.uptime_min / 60)}h ${health.uptime_min % 60}m` : "—"}
            </div>
          </div>
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Streams</div>
            <div className="mt-1.5 text-[22px] font-bold text-zinc-900" style={mono}>
              {health?.streams ?? "—"}
            </div>
          </div>
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Bars Processed</div>
            <div className="mt-1.5 text-[22px] font-bold text-zinc-900" style={mono}>
              {health?.bars_processed?.toLocaleString() ?? "—"}
            </div>
          </div>
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Maker Fill Rate</div>
            <div className="mt-1.5 text-[22px] font-bold text-zinc-900" style={mono}>
              {fillRate === null ? "—" : `${fillRate.toFixed(0)}%`}
            </div>
            <div className="mt-1 text-[10px] leading-snug text-zinc-400">
              filled vs missed post-only entries — missed fills counted, never assumed
            </div>
          </div>
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Gate Passers</div>
            <div
              className={`mt-1.5 text-[22px] font-bold ${gatePassers.length > 0 ? "text-emerald-600" : "text-zinc-900"}`}
              style={mono}
            >
              {rows.length ? gatePassers.length : "—"}
            </div>
          </div>
        </div>
      </section>

      {/* Promotion gate */}
      <section className="rounded-3xl border border-zinc-200 bg-white shadow-[0_1px_3px_rgba(0,0,0,0.04)]">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-zinc-100 px-8 py-5">
          <h2 className="text-[21px] font-bold tracking-tight text-zinc-900" style={sora}>
            Promotion Gate
          </h2>
          <span
            className={`inline-flex items-center rounded-full px-4 py-1.5 text-[12px] font-bold ${
              gatePassers.length > 0 ? "bg-emerald-100 text-emerald-800" : "bg-zinc-100 text-zinc-500"
            }`}
          >
            {rows.length ? `${gatePassers.length} of ${rows.length} streams pass` : "—"}
          </span>
        </div>
        <div className="px-8 py-6">
          <div className="flex flex-wrap items-center gap-2.5">
            {GATE_RULES.map((g) => (
              <span
                key={g}
                className="inline-flex items-center gap-2 rounded-full border border-amber-200 bg-amber-50 px-4 py-2 text-[13px] font-semibold text-amber-900"
              >
                <span className="h-1.5 w-1.5 rounded-full bg-amber-500" />
                {g}
              </span>
            ))}
          </div>
          <p className="mt-4 max-w-3xl text-[13px] leading-relaxed text-zinc-500">
            Pre-registered before the first live trade: all 100 strategies failed offline qualification (0/400), so with
            800 streams a few days of trading is expected to produce lucky leaders by variance alone. Leaderboard
            position alone never justifies real money — only gate survivors earn a go-live discussion.
          </p>
        </div>
      </section>

      {/* Leaderboard */}
      <section className="rounded-3xl border border-zinc-200 bg-white shadow-[0_1px_3px_rgba(0,0,0,0.04)]">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-zinc-100 px-8 py-5">
          <h2 className="text-[21px] font-bold tracking-tight text-zinc-900" style={sora}>
            Strategy Leaderboard
          </h2>
          <div className="flex items-center gap-3">
            <label className="flex cursor-pointer items-center gap-2 rounded-full border border-zinc-200 px-4 py-2 text-[12.5px] font-semibold text-zinc-600 hover:border-zinc-300">
              <input
                type="checkbox"
                checked={minTrades}
                onChange={(e) => setMinTrades(e.target.checked)}
                className="h-3.5 w-3.5 accent-amber-600"
              />
              ≥ 20 trades
            </label>
            <select
              value={symbol}
              onChange={(e) => setSymbol(e.target.value)}
              className="cursor-pointer rounded-full border border-zinc-200 bg-white px-4 py-2 text-[12.5px] font-semibold text-zinc-700 hover:border-zinc-300"
            >
              {SYMBOLS.map((s) => (
                <option key={s} value={s}>
                  {s === "ALL" ? "All symbols" : s.replace("USDT", "")}
                </option>
              ))}
            </select>
            <span className="text-[12px] font-medium text-zinc-400" style={mono}>
              {filtered.length} streams
            </span>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-[13.5px]">
            <thead>
              <tr className="border-b border-zinc-100 text-left text-[10.5px] uppercase tracking-[0.1em] text-zinc-400">
                <th className="px-8 py-3.5 font-bold">Strategy</th>
                <th className="px-3 py-3.5 font-bold">Symbol</th>
                <th className="px-3 py-3.5 text-right font-bold">Trades</th>
                <th className="px-3 py-3.5 text-right font-bold">WR %</th>
                <th className="px-3 py-3.5 text-right font-bold">PF</th>
                <th className="px-3 py-3.5 text-right font-bold">Net $</th>
                <th className="px-3 py-3.5 text-right font-bold">Max DD %</th>
                <th className="px-3 py-3.5 text-right font-bold">Missed</th>
                <th className="px-8 py-3.5 text-right font-bold">Gate</th>
              </tr>
            </thead>
            <tbody style={mono}>
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={9} className="px-8 py-14 text-center font-sans text-[14px] text-zinc-400">
                    No closed trades yet on these filters — the desk trades 24/7; check back soon.
                  </td>
                </tr>
              )}
              {filtered.slice(0, 100).map((r) => (
                <tr key={`${r.strategy}|${r.symbol}`} className="border-b border-zinc-50 transition-colors hover:bg-amber-50/40">
                  <td className="px-8 py-2.5 font-sans font-semibold text-zinc-800">{r.strategy}</td>
                  <td className="px-3 py-2.5 text-zinc-500">{r.symbol.replace("USDT", "")}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{r.n}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{r.wr_pct.toFixed(1)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{r.pf.toFixed(2)}</td>
                  <td className={`px-3 py-2.5 text-right font-semibold tabular-nums ${pnlClass(r.net_usd)}`}>
                    {fmtUSD(r.net_usd)}
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{r.max_dd_pct.toFixed(1)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-500">{r.missed}</td>
                  <td className="px-8 py-2.5 text-right">
                    {r.gate_pass ? (
                      <span className="inline-flex rounded-full bg-emerald-100 px-2.5 py-1 font-sans text-[10px] font-bold text-emerald-800">
                        PASS
                      </span>
                    ) : (
                      <span className="inline-flex rounded-full bg-zinc-100 px-2.5 py-1 font-sans text-[10px] font-semibold text-zinc-400">
                        —
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* Recent trades */}
      <section className="rounded-3xl border border-zinc-200 bg-white shadow-[0_1px_3px_rgba(0,0,0,0.04)]">
        <div className="flex items-center justify-between border-b border-zinc-100 px-8 py-5">
          <h2 className="text-[21px] font-bold tracking-tight text-zinc-900" style={sora}>
            Recent Trades
          </h2>
          <span className="text-[12px] font-medium text-zinc-400" style={mono}>
            last {trades.length}
          </span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-[13.5px]">
            <thead>
              <tr className="border-b border-zinc-100 text-left text-[10.5px] uppercase tracking-[0.1em] text-zinc-400">
                <th className="px-8 py-3.5 font-bold">Time (UTC)</th>
                <th className="px-3 py-3.5 font-bold">Symbol</th>
                <th className="px-3 py-3.5 font-bold">Strategy</th>
                <th className="px-3 py-3.5 font-bold">Dir</th>
                <th className="px-3 py-3.5 text-right font-bold">Entry</th>
                <th className="px-3 py-3.5 text-right font-bold">Exit</th>
                <th className="px-3 py-3.5 font-bold">Reason</th>
                <th className="px-3 py-3.5 text-right font-bold">Hold</th>
                <th className="px-8 py-3.5 text-right font-bold">P&amp;L</th>
              </tr>
            </thead>
            <tbody style={mono}>
              {trades.length === 0 && (
                <tr>
                  <td colSpan={9} className="px-8 py-14 text-center font-sans text-[14px] text-zinc-400">
                    No closed trades yet — the desk trades 24/7; check back soon.
                  </td>
                </tr>
              )}
              {[...trades].reverse().map((t, i) => (
                <tr key={`${t.time}-${t.strategy}-${i}`} className="border-b border-zinc-50 transition-colors hover:bg-amber-50/40">
                  <td className="px-8 py-2.5 tabular-nums text-zinc-500">
                    {new Date(t.time).toISOString().slice(5, 16).replace("T", " ")}
                  </td>
                  <td className="px-3 py-2.5 text-zinc-700">{t.symbol.replace("USDT", "")}</td>
                  <td className="px-3 py-2.5 font-sans font-semibold text-zinc-800">{t.strategy}</td>
                  <td className="px-3 py-2.5">
                    <span className={`text-[11.5px] font-bold ${t.dir === "LONG" ? "text-emerald-600" : "text-red-600"}`}>
                      {t.dir}
                    </span>
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{fmtPrice(t.entry)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-700">{fmtPrice(t.exit)}</td>
                  <td className="px-3 py-2.5">
                    <span
                      className={`inline-flex rounded-full px-2.5 py-1 font-sans text-[10px] font-bold ${
                        t.reason === "TP"
                          ? "bg-emerald-50 text-emerald-700"
                          : t.reason === "SL"
                            ? "bg-red-50 text-red-700"
                            : "bg-zinc-100 text-zinc-500"
                      }`}
                    >
                      {t.reason}
                    </span>
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-zinc-500">{t.hold_min}m</td>
                  <td className={`px-8 py-2.5 text-right font-semibold tabular-nums ${pnlClass(t.pnl_usd)}`}>
                    {fmtUSD(t.pnl_usd)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <p className="pb-4 text-center text-[11.5px] text-zinc-400">
        Paper trading only · $100 notional per trade · maker post-only fill model with missed fills counted · state
        persists on the engine host
      </p>
    </main>
  );
}
