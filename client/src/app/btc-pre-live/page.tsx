"use client";

/**
 * BTC Pre-Live Engine — module home (Phase 1: real Delta data + chart).
 *
 * Sits alongside (never inside) the existing NIFTY Pre-Live engine. Later
 * phases add tabs here: Qualification leaderboard (Phase 2), $1,000 paper
 * desk (Phase 3), gated real-money mirror (Phase 4). Fully additive page —
 * shares no state with any existing route or process. Chrome is built on
 * the shared desk primitives (components/desk/ui) and --desk-* tokens.
 */

import { useState } from "react";
import BTCPreLiveChart from "@/components/btc-prelive/BTCPreLiveChart";
import QualificationBoard from "@/components/btc-prelive/QualificationBoard";
import BTCPaperDesk from "@/components/btc-prelive/BTCPaperDesk";
import { DeskBanner, DeskChip } from "@/components/desk/ui";

type TickerStats = { lastPrice: number; changePct24h: number; fundingRate: number };
type Tab = "chart" | "qualification" | "desk" | "ethdesk";

const PHASES = [
  { n: 1, label: "Data + Chart", state: "done" as const },
  { n: 2, label: "Qualification (50)", state: "done" as const },
  { n: 3, label: "$1,000 Paper Week", state: "active" as const },
  { n: 4, label: "Real Money (gated)", state: "pending" as const },
];

const TABS: [Tab, string][] = [
  ["chart", "Chart"],
  ["qualification", "Qualification"],
  ["desk", "BTC Paper Desk"],
  ["ethdesk", "ETH Paper Desk"],
];

function fmtUSD(v: number): string {
  return v.toLocaleString("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 1 });
}

function PhaseChip({ n, label, state }: { n: number; label: string; state: "done" | "active" | "pending" }) {
  const tone = state === "active" ? "success" : state === "done" ? "primary" : "default";
  return (
    <DeskChip tone={tone} style={{ fontWeight: 700 }}>
      {state === "active" ? (
        <span aria-hidden style={{ width: 6, height: 6, borderRadius: "50%", background: "currentColor" }} />
      ) : state === "done" ? (
        <span aria-hidden style={{ fontSize: 9 }}>✓</span>
      ) : (
        <span aria-hidden style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--desk-outline)" }} />
      )}
      Phase {n} · {label}
    </DeskChip>
  );
}

export default function BTCPreLivePage() {
  const [ticker, setTicker] = useState<TickerStats | null>(null);
  const [tab, setTab] = useState<Tab>("chart");

  const changeUp = (ticker?.changePct24h ?? 0) >= 0;

  return (
    <div style={{ minHeight: "100%", background: "var(--desk-surface-dim)" }}>
      <main className="desk-page" style={{ maxWidth: 1100 }}>
        {/* Header */}
        <div style={{ display: "flex", flexWrap: "wrap", alignItems: "flex-end", justifyContent: "space-between", gap: 12 }}>
          <div>
            <div className="desk-label-md">Pre-Live Engine · BTC</div>
            <h1 className="desk-title-md" style={{ fontSize: "1.25rem", marginTop: 2 }}>BTC Pre-Live Engine</h1>
            <p className="desk-label-md" style={{ marginTop: 4, fontWeight: 400 }}>
              Real Delta Exchange market data · paper first, real money only after the 1-week gate
            </p>
          </div>

          {ticker && (
            <div style={{ display: "flex", alignItems: "center", gap: 20 }}>
              <div style={{ textAlign: "right" }}>
                <div className="desk-label-md">BTC / USD</div>
                <div className="desk-mono desk-title-md">{fmtUSD(ticker.lastPrice)}</div>
              </div>
              <div style={{ textAlign: "right" }}>
                <div className="desk-label-md">24h</div>
                <div className={`desk-mono ${changeUp ? "desk-pnl-positive" : "desk-pnl-negative"}`} style={{ fontWeight: 700, fontSize: "0.875rem" }}>
                  {changeUp ? "▲" : "▼"} {Math.abs(ticker.changePct24h).toFixed(2)}%
                </div>
              </div>
              <div style={{ textAlign: "right" }}>
                <div className="desk-label-md">Funding</div>
                <div className="desk-mono" style={{ fontWeight: 600, fontSize: "0.875rem", color: "var(--desk-on-surface)" }}>
                  {(ticker.fundingRate * 100).toFixed(4)}%
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Phase progress strip */}
        <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
          {PHASES.map((p) => (
            <PhaseChip key={p.n} n={p.n} label={p.label} state={p.state} />
          ))}
        </div>

        {/* Tabs */}
        <div style={{ display: "flex", gap: 4, borderBottom: "1px solid var(--desk-outline)" }}>
          {TABS.map(([key, label]) => (
            <button
              key={key}
              onClick={() => setTab(key)}
              style={{
                marginBottom: -1,
                borderBottom: `2px solid ${tab === key ? "var(--desk-primary)" : "transparent"}`,
                padding: "8px 16px",
                fontSize: "0.8125rem",
                fontWeight: 600,
                color: tab === key ? "var(--desk-on-surface)" : "var(--desk-on-surface-variant)",
                background: "none",
                cursor: "pointer",
              }}
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
        {tab === "ethdesk" && <BTCPaperDesk basePath="/api/eth-prelive/desk" />}

        {/* Phase 1 footnote */}
        <DeskBanner variant="info" title="Phase 1 — Data & Chart">
          Candles are live from Delta Exchange&apos;s public REST API (BTCUSD perpetual); pan left for up to one year
          of history on any timeframe. A full-year candle cache (1m→1d, verified gap-free) is already stored
          engine-side for Phase 2&apos;s strategy qualification run — the top-25 leaderboard, $1,000 paper desk, and
          the gated real-money mirror will appear on this page as their phases land. This module is fully isolated
          from the existing NIFTY Pre-Live Engine.
        </DeskBanner>
      </main>
    </div>
  );
}
