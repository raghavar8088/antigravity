"use client";

/**
 * BTC Pre-Live Engine — Phase 3 paper desk panel ($1,000, 50 qualified
 * strategies, 7-day evaluation).
 *
 * Polls the BTC pre-live instance through /api/btc-prelive/desk/* (session-
 * gated proxy). Field shapes match the Go engine JSON exactly as consumed by
 * the existing PreLiveEngineCenter — same engine binary, different instance.
 */

import { useCallback, useEffect, useState } from "react";

const POLL_MS = 5_000;

// Week-end promotion gate, agreed in the Phase 3 plan.
const GATE = { minTrades: 30, maxDrawdownPct: 10 };

type OpenPosition = {
  id: string;
  strategyName: string;
  side: "BUY" | "SELL" | "LONG" | "SHORT";
  openedAt: string | number;
  size: number;
  entryPrice: number;
  stopLoss: number;
  takeProfit: number;
};

type ClosedTrade = {
  id: string;
  strategyName: string;
  side: "BUY" | "SELL";
  entryPrice: number;
  exitPrice: number;
  netPnl: number;
  exitTime: string | number;
  reason: string;
};

type Stats = {
  balance: number;
  equity: number;
  initialBalance?: number;
  dailyPnl: number;
  openPositions: number;
  strategies: number;
  lastPrice?: number;
  aggregate?: {
    totalTrades?: number;
    totalWins?: number;
    winRate?: number;
    totalPnl?: number;
    profitFactor?: number;
    maxDrawdown?: number;
  };
};

function fmtUSD(v: number | undefined, digits = 2): string {
  if (v === undefined || !Number.isFinite(v)) return "—";
  return v.toLocaleString("en-US", { style: "currency", currency: "USD", maximumFractionDigits: digits });
}

function tsLabel(v: string | number): string {
  const d = typeof v === "number" ? new Date(v) : new Date(String(v));
  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

export default function BTCPaperDesk({ basePath = "/api/btc-prelive/desk" }: { basePath?: string } = {}) {
  const [stats, setStats] = useState<Stats | null>(null);
  const [positions, setPositions] = useState<OpenPosition[]>([]);
  const [trades, setTrades] = useState<ClosedTrade[]>([]);
  const [error, setError] = useState<string | null>(null);

  const poll = useCallback(async () => {
    try {
      const [sRes, pRes, tRes] = await Promise.all([
        fetch(`${basePath}/api/stats`, { cache: "no-store" }),
        fetch(`${basePath}/api/positions`, { cache: "no-store" }),
        fetch(`${basePath}/api/trades`, { cache: "no-store" }),
      ]);
      if (!sRes.ok) {
        setError(`desk unreachable (HTTP ${sRes.status}) — is the pre-live container running?`);
        return;
      }
      setStats((await sRes.json()) as Stats);
      if (pRes.ok) setPositions(((await pRes.json()) as OpenPosition[]) ?? []);
      if (tRes.ok) setTrades(((await tRes.json()) as ClosedTrade[]) ?? []);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "poll failed");
    }
  }, [basePath]);

  useEffect(() => {
    void poll();
    const id = setInterval(() => void poll(), POLL_MS);
    return () => clearInterval(id);
  }, [poll]);

  const initial = stats?.initialBalance ?? 1000;
  const equity = stats?.equity ?? initial;
  const netPct = ((equity - initial) / initial) * 100;
  const agg = stats?.aggregate;
  const totalTrades = agg?.totalTrades ?? 0;
  const winRatePct = (agg?.winRate ?? 0) * 100;
  const ddPct = ((agg?.maxDrawdown ?? 0) / initial) * 100;

  const gateTrades = totalTrades >= GATE.minTrades;
  const gateNet = equity > initial;
  const gateDD = ddPct < GATE.maxDrawdownPct;

  return (
    <div className="flex flex-col gap-3">
      {error && (
        <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-[12px] text-amber-800">
          {error}
        </div>
      )}

      {/* Stat tiles */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        {[
          ["Equity", fmtUSD(equity), netPct >= 0 ? "text-emerald-600" : "text-red-600"],
          ["Start", fmtUSD(initial, 0), "text-zinc-900"],
          [
            "Net P&L",
            `${netPct >= 0 ? "+" : ""}${netPct.toFixed(2)}%`,
            netPct >= 0 ? "text-emerald-600" : "text-red-600",
          ],
          ["Open", String(stats?.openPositions ?? 0), "text-zinc-900"],
          ["Trades", String(totalTrades), "text-zinc-900"],
          ["Win rate", totalTrades ? `${winRatePct.toFixed(1)}%` : "—", "text-zinc-900"],
        ].map(([label, value, cls]) => (
          <div key={label} className="glass-panel rounded-2xl px-4 py-3">
            <div className="text-[10px] uppercase tracking-wider text-zinc-400">{label}</div>
            <div className={`mt-0.5 text-lg font-bold tabular-nums ${cls}`}>{value}</div>
          </div>
        ))}
      </div>

      {/* Promotion gate */}
      <div className="glass-panel flex flex-wrap items-center gap-x-6 gap-y-2 rounded-2xl px-5 py-3">
        <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
          Week-end gate
        </span>
        {[
          [`≥ ${GATE.minTrades} trades`, gateTrades, `${totalTrades}/${GATE.minTrades}`],
          ["net positive after fees", gateNet, `${netPct >= 0 ? "+" : ""}${netPct.toFixed(2)}%`],
          [`max DD < ${GATE.maxDrawdownPct}%`, gateDD, `${ddPct.toFixed(1)}%`],
        ].map(([label, ok, detail]) => (
          <span key={String(label)} className="inline-flex items-center gap-1.5 text-[12px]">
            <span className={`h-2 w-2 rounded-full ${ok ? "bg-emerald-500" : "bg-zinc-300"}`} />
            <span className={ok ? "text-zinc-800" : "text-zinc-400"}>
              {label} <span className="tabular-nums text-zinc-400">({detail})</span>
            </span>
          </span>
        ))}
        <span className="ml-auto text-[11px] text-zinc-400">
          real money (Phase 4) only after all three hold for the full week
        </span>
      </div>

      {/* Open positions */}
      <div className="glass-panel overflow-x-auto rounded-2xl">
        <div className="px-5 pt-3 text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
          Open positions ({positions.length})
        </div>
        {positions.length === 0 ? (
          <div className="px-5 py-6 text-center text-[12px] text-zinc-400">No open positions</div>
        ) : (
          <table className="w-full min-w-[640px] text-left text-[12px]">
            <thead>
              <tr className="border-b border-zinc-200 text-[10px] uppercase tracking-wider text-zinc-400">
                <th className="px-5 py-2">Strategy</th>
                <th className="px-2 py-2">Side</th>
                <th className="px-2 py-2 text-right">Size</th>
                <th className="px-2 py-2 text-right">Entry</th>
                <th className="px-2 py-2 text-right">SL</th>
                <th className="px-2 py-2 text-right">TP</th>
                <th className="px-5 py-2 text-right">Opened</th>
              </tr>
            </thead>
            <tbody>
              {positions.map((p) => (
                <tr key={p.id} className="border-b border-zinc-100 last:border-0">
                  <td className="max-w-[220px] truncate px-5 py-2 font-medium text-zinc-800" title={p.strategyName}>
                    {p.strategyName}
                  </td>
                  <td className="px-2 py-2">
                    <span
                      className={`rounded px-1.5 py-0.5 text-[10px] font-bold ${
                        p.side === "BUY" || p.side === "LONG"
                          ? "bg-emerald-100 text-emerald-700"
                          : "bg-red-100 text-red-700"
                      }`}
                    >
                      {p.side}
                    </span>
                  </td>
                  <td className="px-2 py-2 text-right tabular-nums">{p.size}</td>
                  <td className="px-2 py-2 text-right tabular-nums">{fmtUSD(p.entryPrice, 1)}</td>
                  <td className="px-2 py-2 text-right tabular-nums text-red-500">{fmtUSD(p.stopLoss, 1)}</td>
                  <td className="px-2 py-2 text-right tabular-nums text-emerald-600">{fmtUSD(p.takeProfit, 1)}</td>
                  <td className="px-5 py-2 text-right tabular-nums text-zinc-400">{tsLabel(p.openedAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Trade blotter */}
      <div className="glass-panel overflow-x-auto rounded-2xl">
        <div className="px-5 pt-3 text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
          Recent trades ({trades.length})
        </div>
        {trades.length === 0 ? (
          <div className="px-5 py-6 text-center text-[12px] text-zinc-400">
            No closed trades yet — the desk trades 24/7; check back soon.
          </div>
        ) : (
          <table className="w-full min-w-[640px] text-left text-[12px]">
            <thead>
              <tr className="border-b border-zinc-200 text-[10px] uppercase tracking-wider text-zinc-400">
                <th className="px-5 py-2">Strategy</th>
                <th className="px-2 py-2">Side</th>
                <th className="px-2 py-2 text-right">Entry</th>
                <th className="px-2 py-2 text-right">Exit</th>
                <th className="px-2 py-2 text-right">Net P&L</th>
                <th className="px-2 py-2">Reason</th>
                <th className="px-5 py-2 text-right">Closed</th>
              </tr>
            </thead>
            <tbody>
              {trades.slice(0, 30).map((t) => (
                <tr key={t.id} className="border-b border-zinc-100 last:border-0">
                  <td className="max-w-[220px] truncate px-5 py-2 font-medium text-zinc-800" title={t.strategyName}>
                    {t.strategyName}
                  </td>
                  <td className="px-2 py-2">
                    <span
                      className={`rounded px-1.5 py-0.5 text-[10px] font-bold ${
                        t.side === "BUY" ? "bg-emerald-100 text-emerald-700" : "bg-red-100 text-red-700"
                      }`}
                    >
                      {t.side}
                    </span>
                  </td>
                  <td className="px-2 py-2 text-right tabular-nums">{fmtUSD(t.entryPrice, 1)}</td>
                  <td className="px-2 py-2 text-right tabular-nums">{fmtUSD(t.exitPrice, 1)}</td>
                  <td
                    className={`px-2 py-2 text-right font-semibold tabular-nums ${
                      t.netPnl >= 0 ? "text-emerald-600" : "text-red-600"
                    }`}
                  >
                    {t.netPnl >= 0 ? "+" : ""}
                    {fmtUSD(t.netPnl)}
                  </td>
                  <td className="px-2 py-2 text-[11px] text-zinc-500">{t.reason}</td>
                  <td className="px-5 py-2 text-right tabular-nums text-zinc-400">{tsLabel(t.exitTime)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
