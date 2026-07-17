"use client";

/**
 * BTC Pre-Live Engine — module home (Phase 1: real Delta data + chart).
 *
 * Sits alongside (never inside) the existing NIFTY Pre-Live engine. Later
 * phases add tabs here: Qualification leaderboard (Phase 2), $1,000 paper
 * desk (Phase 3), gated real-money mirror (Phase 4). Fully additive page —
 * shares no state with any existing route or process.
 */

import { useState } from "react";
import BTCPreLiveChart from "@/components/btc-prelive/BTCPreLiveChart";
import QualificationBoard from "@/components/btc-prelive/QualificationBoard";
import BTCPaperDesk from "@/components/btc-prelive/BTCPaperDesk";

type TickerStats = { lastPrice: number; changePct24h: number; fundingRate: number };
type Tab = "chart" | "qualification" | "desk";

const PHASES = [
  { n: 1, label: "Data + Chart", state: "done" as const },
  { n: 2, label: "Qualification (50)", state: "done" as const },
  { n: 3, label: "$1,000 Paper Week", state: "active" as const },
  { n: 4, label: "Real Money (gated)", state: "pending" as const },
];

function fmtUSD(v: number): string {
  return v.toLocaleString("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 1 });
}

export default function BTCPreLivePage() {
  const [ticker, setTicker] = useState<TickerStats | null>(null);
  const [tab, setTab] = useState<Tab>("chart");

  const changeUp = (ticker?.changePct24h ?? 0) >= 0;

  return (
    <main className="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-6">
      {/* Header */}
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-zinc-400">
            Pre-Live Engine · BTC
          </div>
          <h1 className="text-xl font-bold text-zinc-900">BTC Pre-Live Engine</h1>
          <p className="mt-0.5 text-[12px] text-zinc-500">
            Real Delta Exchange market data · paper first, real money only after the 1-week gate
          </p>
        </div>

        <div className="flex items-center gap-4">
          {ticker && (
            <>
              <div className="text-right">
                <div className="text-[10px] uppercase tracking-wider text-zinc-400">BTC / USD</div>
                <div className="text-lg font-bold tabular-nums text-zinc-900">{fmtUSD(ticker.lastPrice)}</div>
              </div>
              <div className="text-right">
                <div className="text-[10px] uppercase tracking-wider text-zinc-400">24h</div>
                <div className={`text-sm font-semibold tabular-nums ${changeUp ? "text-emerald-600" : "text-red-600"}`}>
                  {changeUp ? "▲" : "▼"} {Math.abs(ticker.changePct24h).toFixed(2)}%
                </div>
              </div>
              <div className="text-right">
                <div className="text-[10px] uppercase tracking-wider text-zinc-400">Funding</div>
                <div className="text-sm font-semibold tabular-nums text-zinc-700">
                  {(ticker.fundingRate * 100).toFixed(4)}%
                </div>
              </div>
            </>
          )}
        </div>
      </div>

      {/* Phase progress strip */}
      <div className="flex flex-wrap items-center gap-2">
        {PHASES.map((p) => (
          <span
            key={p.n}
            className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[11px] font-semibold ${
              p.state === "active"
                ? "bg-emerald-100 text-emerald-800"
                : p.state === "done"
                  ? "bg-zinc-800 text-zinc-100"
                  : "bg-zinc-100 text-zinc-400"
            }`}
          >
            {p.state === "active" ? (
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-500" />
            ) : p.state === "done" ? (
              <span className="text-[9px]">✓</span>
            ) : (
              <span className="h-1.5 w-1.5 rounded-full bg-zinc-300" />
            )}
            Phase {p.n} · {p.label}
          </span>
        ))}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-zinc-200">
        {(
          [
            ["chart", "Chart"],
            ["qualification", "Qualification"],
            ["desk", "Paper Desk"],
          ] as [Tab, string][]
        ).map(([key, label]) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`-mb-px border-b-2 px-4 py-2 text-[13px] font-semibold transition-colors ${
              tab === key
                ? "border-zinc-900 text-zinc-900"
                : "border-transparent text-zinc-400 hover:text-zinc-600"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Tab content — chart stays mounted so its live feed and loaded history
          survive tab switches */}
      <div className={tab === "chart" ? "" : "hidden"}>
        <BTCPreLiveChart onTicker={setTicker} />
      </div>
      {tab === "qualification" && <QualificationBoard />}
      {tab === "desk" && <BTCPaperDesk />}

      {/* Phase 1 footnote */}
      <div className="rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-[12px] leading-relaxed text-zinc-500">
        <span className="font-semibold text-zinc-700">Phase 1 — Data &amp; Chart.</span>{" "}
        Candles are live from Delta Exchange&apos;s public REST API (BTCUSD perpetual); pan left for up to one year of
        history on any timeframe. A full-year candle cache (1m→1d, verified gap-free) is already stored engine-side for
        Phase 2&apos;s strategy qualification run — the top-25 leaderboard, $1,000 paper desk, and the gated real-money
        mirror will appear on this page as their phases land. This module is fully isolated from the existing NIFTY
        Pre-Live Engine.
      </div>
    </main>
  );
}
