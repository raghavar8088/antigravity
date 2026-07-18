"use client";

/**
 * Crypto Scalp Desk — dashboard for the scalp_prelive paper engine
 * (100 x 1m scalp strategies x 8 symbols = 800 live paper streams).
 *
 * TradingAI-style layout: breadcrumb header, large monospace stat cards,
 * desk-analytics row, promotion-gate card, leaderboard and trades tables.
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
    <main className="mx-auto flex max-w-6xl flex-col gap-5 px-6 py-6">
      {/* Breadcrumb + title */}
      <div>
        <div className="flex items-center gap-1.5 text-[13px]">
          <a href="/terminal" className="text-zinc-400 underline-offset-2 hover:text-zinc-600 hover:underline">
            Home
          </a>
          <span className="text-zinc-300">›</span>
          <span className="font-medium text-zinc-600">Scalp Desk</span>
        </div>
        <h1 className="mt-1 text-[34px] font-extrabold leading-tight tracking-tight text-zinc-900">Scalp Desk</h1>
        <p className="mt-0.5 text-[13.5px] text-zinc-500">
          100 one-minute strategies × 8 majors — 800 live paper streams from your scalp engine on AWS.
        </p>
      </div>

      {error && (
        <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-2.5 text-[13px] text-amber-800">
          {error} — retrying every 30s
        </div>
      )}

      {/* Big stat cards */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div className="rounded-2xl border border-zinc-200 bg-white px-6 py-5">
          <div className="text-[11px] font-bold uppercase tracking-[0.14em] text-zinc-400">Net P&amp;L</div>
          <div
            className={`mt-2 text-[34px] font-bold leading-none tabular-nums ${pnlClass(stats?.net_pnl_usd_at_100_notional ?? 0)}`}
            style={mono}
          >
            {stats ? fmtUSD(stats.net_pnl_usd_at_100_notional) : "—"}
          </div>
          <div className="mt-2.5 text-[12px] text-zinc-400" style={mono}>
            $100 notional per trade
          </div>
        </div>
        <div className="rounded-2xl border border-zinc-200 bg-white px-6 py-5">
          <div className="text-[11px] font-bold uppercase tracking-[0.14em] text-zinc-400">Closed Trades</div>
          <div className="mt-2 text-[34px] font-bold leading-none tabular-nums text-zinc-900" style={mono}>
            {stats?.trades ?? "—"}
          </div>
          <div className="mt-2.5 text-[12px] text-emerald-600" style={mono}>
            {winRate === null ? "win rate —" : `win rate ${winRate.toFixed(1)}%`}
          </div>
        </div>
        <div className="rounded-2xl border border-zinc-200 bg-white px-6 py-5">
          <div className="text-[11px] font-bold uppercase tracking-[0.14em] text-zinc-400">Open / Pending</div>
          <div className="mt-2 text-[34px] font-bold leading-none tabular-nums text-zinc-900" style={mono}>
            {stats ? `${stats.open_positions} / ${stats.pending_orders}` : "—"}
          </div>
          <div className="mt-2.5 text-[12px] text-zinc-400" style={mono}>
            positions / resting post-only orders
          </div>
        </div>
      </div>

      {/* Desk analytics row */}
      <section className="rounded-2xl border border-zinc-200 bg-white">
        <div className="flex items-center justify-between border-b border-zinc-100 px-6 py-4">
          <h2 className="text-[17px] font-bold text-zinc-900">Desk Analytics</h2>
          <span className="text-[12px] text-zinc-400">{updatedAt ? `computed ${updatedAt}` : "—"}</span>
        </div>
        <div className="grid grid-cols-2 gap-x-6 gap-y-4 px-6 py-4 md:grid-cols-6">
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Engine</div>
            <div className={`mt-1 text-[20px] font-bold ${health?.ok ? "text-emerald-600" : "text-red-600"}`} style={mono}>
              {health ? (health.ok ? "UP" : "DOWN") : "—"}
            </div>
          </div>
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Uptime</div>
            <div className="mt-1 text-[20px] font-bold text-zinc-900" style={mono}>
              {health ? `${Math.floor(health.uptime_min / 60)}h ${health.uptime_min % 60}m` : "—"}
            </div>
          </div>
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Streams</div>
            <div className="mt-1 text-[20px] font-bold text-zinc-900" style={mono}>
              {health?.streams ?? "—"}
            </div>
          </div>
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Bars Processed</div>
            <div className="mt-1 text-[20px] font-bold text-zinc-900" style={mono}>
              {health?.bars_processed?.toLocaleString() ?? "—"}
            </div>
          </div>
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Maker Fill Rate</div>
            <div className="mt-1 text-[20px] font-bold text-zinc-900" style={mono}>
              {fillRate === null ? "—" : `${fillRate.toFixed(0)}%`}
            </div>
            <div className="mt-0.5 text-[10px] leading-snug text-zinc-400">
              filled vs missed post-only entries — missed fills are counted, never assumed
            </div>
          </div>
          <div>
            <div className="text-[10.5px] font-bold uppercase tracking-wider text-zinc-400">Gate Passers</div>
            <div
              className={`mt-1 text-[20px] font-bold ${gatePassers.length > 0 ? "text-emerald-600" : "text-zinc-900"}`}
              style={mono}
            >
              {rows.length ? gatePassers.length : "—"}
            </div>
          </div>
        </div>
      </section>

      {/* Promotion gate */}
      <section className="rounded-2xl border border-zinc-200 bg-white">
        <div className="flex items-center justify-between border-b border-zinc-100 px-6 py-4">
          <h2 className="text-[17px] font-bold text-zinc-900">Promotion Gate</h2>
          <span
            className={`inline-flex items-center rounded-full px-3 py-1 text-[11px] font-bold ${
              gatePassers.length > 0 ? "bg-emerald-100 text-emerald-800" : "bg-zinc-100 text-zinc-500"
            }`}
          >
            {rows.length ? `${gatePassers.length} of ${rows.length} streams pass` : "—"}
          </span>
        </div>
        <div className="px-6 py-4">
          <div className="flex flex-wrap items-center gap-2">
            {GATE_RULES.map((g) => (
              <span
                key={g}
                className="inline-flex items-center gap-1.5 rounded-full bg-zinc-100 px-3 py-1.5 text-[12px] font-semibold text-zinc-600"
              >
                <span className="h-1.5 w-1.5 rounded-full bg-violet-500" />
                {g}
              </span>
            ))}
          </div>
          <p className="mt-3 text-[12px] leading-relaxed text-zinc-400">
            Pre-registered before the first live trade: all 100 strategies failed offline qualification (0/400), so with
            800 streams a few days of trading is expected to produce lucky leaders by variance alone. Leaderboard
            position alone never justifies real money — only gate survivors earn a go-live discussion.
          </p>
        </div>
      </section>

      {/* Leaderboard */}
      <section className="rounded-2xl border border-zinc-200 bg-white">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-zinc-100 px-6 py-4">
          <h2 className="text-[17px] font-bold text-zinc-900">Strategy Leaderboard</h2>
          <div className="flex items-center gap-3">
            <label className="flex items-center gap-1.5 text-[12px] text-zinc-500">
              <input
                type="checkbox"
                checked={minTrades}
                onChange={(e) => setMinTrades(e.target.checked)}
                className="h-3.5 w-3.5 accent-violet-600"
              />
              ≥ 20 trades
            </label>
            <select
              value={symbol}
              onChange={(e) => setSymbol(e.target.value)}
              className="rounded-lg border border-zinc-200 bg-white px-2.5 py-1.5 text-[12px] font-medium text-zinc-700"
            >
              {SYMBOLS.map((s) => (
                <option key={s} value={s}>
                  {s === "ALL" ? "All symbols" : s.replace("USDT", "")}
                </option>
              ))}
            </select>
            <span className="text-[12px] text-zinc-400" style={mono}>
              {filtered.length} streams
            </span>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-[13px]">
            <thead>
              <tr className="border-b border-zinc-100 text-left text-[10.5px] uppercase tracking-wider text-zinc-400">
                <th className="px-6 py-2.5 font-semibold">Strategy</th>
                <th className="px-3 py-2.5 font-semibold">Symbol</th>
                <th className="px-3 py-2.5 text-right font-semibold">Trades</th>
                <th className="px-3 py-2.5 text-right font-semibold">WR %</th>
                <th className="px-3 py-2.5 text-right font-semibold">PF</th>
                <th className="px-3 py-2.5 text-right font-semibold">Net $</th>
                <th className="px-3 py-2.5 text-right font-semibold">Max DD %</th>
                <th className="px-3 py-2.5 text-right font-semibold">Missed</th>
                <th className="px-6 py-2.5 text-right font-semibold">Gate</th>
              </tr>
            </thead>
            <tbody style={mono}>
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={9} className="px-6 py-10 text-center font-sans text-zinc-400">
                    No closed trades yet on these filters — the desk trades 24/7; check back soon.
                  </td>
                </tr>
              )}
              {filtered.slice(0, 100).map((r) => (
                <tr key={`${r.strategy}|${r.symbol}`} className="border-b border-zinc-50 hover:bg-zinc-50">
                  <td className="px-6 py-2 font-sans font-semibold text-zinc-800">{r.strategy}</td>
                  <td className="px-3 py-2 text-zinc-500">{r.symbol.replace("USDT", "")}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-zinc-700">{r.n}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-zinc-700">{r.wr_pct.toFixed(1)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-zinc-700">{r.pf.toFixed(2)}</td>
                  <td className={`px-3 py-2 text-right font-semibold tabular-nums ${pnlClass(r.net_usd)}`}>
                    {fmtUSD(r.net_usd)}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums text-zinc-700">{r.max_dd_pct.toFixed(1)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-zinc-500">{r.missed}</td>
                  <td className="px-6 py-2 text-right">
                    {r.gate_pass ? (
                      <span className="inline-flex rounded-full bg-emerald-100 px-2 py-0.5 font-sans text-[10px] font-bold text-emerald-800">
                        PASS
                      </span>
                    ) : (
                      <span className="inline-flex rounded-full bg-zinc-100 px-2 py-0.5 font-sans text-[10px] font-semibold text-zinc-400">
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
      <section className="rounded-2xl border border-zinc-200 bg-white">
        <div className="flex items-center justify-between border-b border-zinc-100 px-6 py-4">
          <h2 className="text-[17px] font-bold text-zinc-900">Recent Trades</h2>
          <span className="text-[12px] text-zinc-400" style={mono}>
            last {trades.length}
          </span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-[13px]">
            <thead>
              <tr className="border-b border-zinc-100 text-left text-[10.5px] uppercase tracking-wider text-zinc-400">
                <th className="px-6 py-2.5 font-semibold">Time (UTC)</th>
                <th className="px-3 py-2.5 font-semibold">Symbol</th>
                <th className="px-3 py-2.5 font-semibold">Strategy</th>
                <th className="px-3 py-2.5 font-semibold">Dir</th>
                <th className="px-3 py-2.5 text-right font-semibold">Entry</th>
                <th className="px-3 py-2.5 text-right font-semibold">Exit</th>
                <th className="px-3 py-2.5 font-semibold">Reason</th>
                <th className="px-3 py-2.5 text-right font-semibold">Hold</th>
                <th className="px-6 py-2.5 text-right font-semibold">P&amp;L</th>
              </tr>
            </thead>
            <tbody style={mono}>
              {trades.length === 0 && (
                <tr>
                  <td colSpan={9} className="px-6 py-10 text-center font-sans text-zinc-400">
                    No closed trades yet — the desk trades 24/7; check back soon.
                  </td>
                </tr>
              )}
              {[...trades].reverse().map((t, i) => (
                <tr key={`${t.time}-${t.strategy}-${i}`} className="border-b border-zinc-50 hover:bg-zinc-50">
                  <td className="px-6 py-2 tabular-nums text-zinc-500">
                    {new Date(t.time).toISOString().slice(5, 16).replace("T", " ")}
                  </td>
                  <td className="px-3 py-2 text-zinc-700">{t.symbol.replace("USDT", "")}</td>
                  <td className="px-3 py-2 font-sans font-semibold text-zinc-800">{t.strategy}</td>
                  <td className="px-3 py-2">
                    <span className={`text-[11px] font-bold ${t.dir === "LONG" ? "text-emerald-600" : "text-red-600"}`}>
                      {t.dir}
                    </span>
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums text-zinc-700">{fmtPrice(t.entry)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-zinc-700">{fmtPrice(t.exit)}</td>
                  <td className="px-3 py-2">
                    <span
                      className={`inline-flex rounded-full px-2 py-0.5 font-sans text-[10px] font-semibold ${
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
                  <td className="px-3 py-2 text-right tabular-nums text-zinc-500">{t.hold_min}m</td>
                  <td className={`px-6 py-2 text-right font-semibold tabular-nums ${pnlClass(t.pnl_usd)}`}>
                    {fmtUSD(t.pnl_usd)}
                  </td>
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
