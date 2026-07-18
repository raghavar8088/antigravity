"use client";

/**
 * Crypto Scalp Desk — dashboard for the scalp_prelive paper engine
 * (100 x 1m scalp strategies x 8 symbols = 800 live paper streams).
 *
 * Reads exclusively through the session-gated /api/scalp proxy. The desk is
 * paper-only; the PRE-REGISTERED promotion gate shown in the banner is the
 * only route by which any stream can be nominated for a real-money
 * discussion. Fully additive page — shares no state with any other module.
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

const GATE_RULES = [
  { label: "≥ 200 live trades", key: "n" },
  { label: "PF ≥ 1.2", key: "pf" },
  { label: "max DD ≤ 25%", key: "dd" },
  { label: "both halves net-positive", key: "halves" },
];

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

  return (
    <main className="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-6">
      {/* Header */}
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-zinc-400">
            Scalp Lane · 8 Symbols · Paper
          </div>
          <h1 className="text-xl font-bold text-zinc-900">Crypto Scalp Desk</h1>
          <p className="mt-0.5 text-[12px] text-zinc-500">
            100 one-minute strategies × 8 majors = 800 live paper streams · maker-fill model identical to the backtest ·
            real money only via the pre-registered gate
          </p>
        </div>
        <div className="flex items-center gap-4">
          {health && (
            <>
              <div className="text-right">
                <div className="text-[10px] uppercase tracking-wider text-zinc-400">Engine</div>
                <div className={`text-sm font-semibold ${health.ok ? "text-emerald-600" : "text-red-600"}`}>
                  {health.ok ? "UP" : "DOWN"}
                </div>
              </div>
              <div className="text-right">
                <div className="text-[10px] uppercase tracking-wider text-zinc-400">Uptime</div>
                <div className="text-sm font-semibold tabular-nums text-zinc-700">
                  {Math.floor(health.uptime_min / 60)}h {health.uptime_min % 60}m
                </div>
              </div>
              <div className="text-right">
                <div className="text-[10px] uppercase tracking-wider text-zinc-400">Streams</div>
                <div className="text-sm font-semibold tabular-nums text-zinc-700">{health.streams}</div>
              </div>
            </>
          )}
          {updatedAt && (
            <div className="text-right">
              <div className="text-[10px] uppercase tracking-wider text-zinc-400">Updated</div>
              <div className="text-sm font-semibold tabular-nums text-zinc-500">{updatedAt}</div>
            </div>
          )}
        </div>
      </div>

      {error && (
        <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-2 text-[13px] text-amber-800">
          {error} — retrying every 30s
        </div>
      )}

      {/* Stat cards */}
      <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
        <div className="rounded-xl border border-zinc-200 bg-white px-4 py-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Closed Trades</div>
          <div className="mt-1 text-2xl font-bold tabular-nums text-zinc-900">{stats?.trades ?? "—"}</div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white px-4 py-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Net P&amp;L ($100/trade)</div>
          <div className={`mt-1 text-2xl font-bold tabular-nums ${pnlClass(stats?.net_pnl_usd_at_100_notional ?? 0)}`}>
            {stats ? fmtUSD(stats.net_pnl_usd_at_100_notional) : "—"}
          </div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white px-4 py-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Win Rate</div>
          <div className="mt-1 text-2xl font-bold tabular-nums text-zinc-900">
            {winRate === null ? "—" : `${winRate.toFixed(1)}%`}
          </div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white px-4 py-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Open / Pending</div>
          <div className="mt-1 text-2xl font-bold tabular-nums text-zinc-900">
            {stats ? `${stats.open_positions} / ${stats.pending_orders}` : "—"}
          </div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white px-4 py-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Missed Fills</div>
          <div className="mt-1 text-2xl font-bold tabular-nums text-zinc-900">{stats?.missed_fills ?? "—"}</div>
        </div>
      </div>

      {/* Promotion gate strip */}
      <div className="rounded-xl border border-zinc-200 bg-white px-4 py-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-[11px] font-bold uppercase tracking-[0.18em] text-zinc-500">Promotion Gate</span>
            {GATE_RULES.map((g) => (
              <span
                key={g.key}
                className="inline-flex items-center gap-1.5 rounded-full bg-zinc-100 px-3 py-1 text-[11px] font-semibold text-zinc-600"
              >
                <span className="h-1.5 w-1.5 rounded-full bg-zinc-400" />
                {g.label}
              </span>
            ))}
          </div>
          <span
            className={`inline-flex items-center rounded-full px-3 py-1 text-[11px] font-bold ${
              gatePassers.length > 0 ? "bg-emerald-100 text-emerald-800" : "bg-zinc-100 text-zinc-500"
            }`}
          >
            {gatePassers.length} of {rows.length || "—"} streams pass
          </span>
        </div>
        <p className="mt-1.5 text-[11px] leading-relaxed text-zinc-400">
          Pre-registered before the first live trade: all 100 strategies failed offline qualification (0/400), so with
          800 streams a few days of trading is expected to produce lucky leaders by variance alone. Leaderboard position
          alone never justifies real money — only gate survivors earn a go-live discussion.
        </p>
      </div>

      {/* Leaderboard */}
      <section className="rounded-xl border border-zinc-200 bg-white">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-zinc-100 px-4 py-3">
          <h2 className="text-[15px] font-bold text-zinc-900">Strategy Leaderboard</h2>
          <div className="flex items-center gap-2">
            <label className="flex items-center gap-1.5 text-[12px] text-zinc-500">
              <input
                type="checkbox"
                checked={minTrades}
                onChange={(e) => setMinTrades(e.target.checked)}
                className="h-3.5 w-3.5 accent-zinc-800"
              />
              ≥ 20 trades
            </label>
            <select
              value={symbol}
              onChange={(e) => setSymbol(e.target.value)}
              className="rounded-lg border border-zinc-200 bg-white px-2 py-1 text-[12px] font-medium text-zinc-700"
            >
              {SYMBOLS.map((s) => (
                <option key={s} value={s}>
                  {s === "ALL" ? "All symbols" : s.replace("USDT", "")}
                </option>
              ))}
            </select>
            <span className="text-[11px] tabular-nums text-zinc-400">{filtered.length} streams</span>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-[12px]">
            <thead>
              <tr className="border-b border-zinc-100 text-left text-[10px] uppercase tracking-wider text-zinc-400">
                <th className="px-4 py-2 font-semibold">Strategy</th>
                <th className="px-2 py-2 font-semibold">Symbol</th>
                <th className="px-2 py-2 text-right font-semibold">Trades</th>
                <th className="px-2 py-2 text-right font-semibold">WR %</th>
                <th className="px-2 py-2 text-right font-semibold">PF</th>
                <th className="px-2 py-2 text-right font-semibold">Net $</th>
                <th className="px-2 py-2 text-right font-semibold">Max DD %</th>
                <th className="px-2 py-2 text-right font-semibold">Missed</th>
                <th className="px-4 py-2 text-right font-semibold">Gate</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={9} className="px-4 py-8 text-center text-zinc-400">
                    No closed trades yet on these filters — the desk trades 24/7; check back soon.
                  </td>
                </tr>
              )}
              {filtered.slice(0, 100).map((r) => (
                <tr key={`${r.strategy}|${r.symbol}`} className="border-b border-zinc-50 hover:bg-zinc-50">
                  <td className="px-4 py-1.5 font-medium text-zinc-800">{r.strategy}</td>
                  <td className="px-2 py-1.5 text-zinc-500">{r.symbol.replace("USDT", "")}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums text-zinc-700">{r.n}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums text-zinc-700">{r.wr_pct.toFixed(1)}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums text-zinc-700">{r.pf.toFixed(2)}</td>
                  <td className={`px-2 py-1.5 text-right font-semibold tabular-nums ${pnlClass(r.net_usd)}`}>
                    {fmtUSD(r.net_usd)}
                  </td>
                  <td className="px-2 py-1.5 text-right tabular-nums text-zinc-700">{r.max_dd_pct.toFixed(1)}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums text-zinc-500">{r.missed}</td>
                  <td className="px-4 py-1.5 text-right">
                    {r.gate_pass ? (
                      <span className="inline-flex rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-bold text-emerald-800">
                        PASS
                      </span>
                    ) : (
                      <span className="inline-flex rounded-full bg-zinc-100 px-2 py-0.5 text-[10px] font-semibold text-zinc-400">
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
      <section className="rounded-xl border border-zinc-200 bg-white">
        <div className="border-b border-zinc-100 px-4 py-3">
          <h2 className="text-[15px] font-bold text-zinc-900">Recent Trades</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-[12px]">
            <thead>
              <tr className="border-b border-zinc-100 text-left text-[10px] uppercase tracking-wider text-zinc-400">
                <th className="px-4 py-2 font-semibold">Time (UTC)</th>
                <th className="px-2 py-2 font-semibold">Symbol</th>
                <th className="px-2 py-2 font-semibold">Strategy</th>
                <th className="px-2 py-2 font-semibold">Dir</th>
                <th className="px-2 py-2 text-right font-semibold">Entry</th>
                <th className="px-2 py-2 text-right font-semibold">Exit</th>
                <th className="px-2 py-2 font-semibold">Reason</th>
                <th className="px-2 py-2 text-right font-semibold">Hold</th>
                <th className="px-4 py-2 text-right font-semibold">P&amp;L</th>
              </tr>
            </thead>
            <tbody>
              {trades.length === 0 && (
                <tr>
                  <td colSpan={9} className="px-4 py-8 text-center text-zinc-400">
                    No closed trades yet — the desk trades 24/7; check back soon.
                  </td>
                </tr>
              )}
              {[...trades].reverse().map((t, i) => (
                <tr key={`${t.time}-${t.strategy}-${i}`} className="border-b border-zinc-50 hover:bg-zinc-50">
                  <td className="px-4 py-1.5 tabular-nums text-zinc-500">
                    {new Date(t.time).toISOString().slice(5, 16).replace("T", " ")}
                  </td>
                  <td className="px-2 py-1.5 text-zinc-700">{t.symbol.replace("USDT", "")}</td>
                  <td className="px-2 py-1.5 font-medium text-zinc-800">{t.strategy}</td>
                  <td className="px-2 py-1.5">
                    <span
                      className={`text-[11px] font-bold ${t.dir === "LONG" ? "text-emerald-600" : "text-red-600"}`}
                    >
                      {t.dir}
                    </span>
                  </td>
                  <td className="px-2 py-1.5 text-right tabular-nums text-zinc-700">{fmtPrice(t.entry)}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums text-zinc-700">{fmtPrice(t.exit)}</td>
                  <td className="px-2 py-1.5">
                    <span
                      className={`inline-flex rounded-full px-2 py-0.5 text-[10px] font-semibold ${
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
                  <td className="px-2 py-1.5 text-right tabular-nums text-zinc-500">{t.hold_min}m</td>
                  <td className={`px-4 py-1.5 text-right font-semibold tabular-nums ${pnlClass(t.pnl_usd)}`}>
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
