"use client";

import { useEffect, useState } from "react";
import type { TerminalSnapshot } from "@/lib/terminal/terminalTypes";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { pct, usd } from "./format";
import { Metric, TerminalCard } from "./TerminalCard";

export function AnalyticsCenter({ snapshot }: { snapshot: TerminalSnapshot }) {
  const curve = snapshot.analytics.equityCurve;
  const hasCurve = curve.length > 1;
  const maxR = Math.max(1, ...snapshot.analytics.rMultipleBuckets.map((b) => b.count));

  return (
    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_360px]">
      <div className="space-y-3">
        <TerminalCard title="Equity Curve" subtitle="/api/paper-desk/equity">
          {!hasCurve ? (
            <TerminalNoData label="NO EQUITY CURVE DATA" />
          ) : (
            <div className="h-72 rounded-lg border border-zinc-800 bg-zinc-950/40 p-3">
              <svg viewBox="0 0 700 240" className="h-full w-full" role="img" aria-label="Equity curve">
                <polyline
                  fill="none"
                  stroke="rgb(52 211 153)"
                  strokeWidth="3"
                  points={curve.map((p, i) => {
                    const minEq = Math.min(...curve.map((c) => c.equity));
                    const maxEq = Math.max(...curve.map((c) => c.equity));
                    const range = maxEq - minEq || 1;
                    return `${40 + i * (660 / Math.max(1, curve.length - 1))},${210 - ((p.equity - minEq) / range) * 180}`;
                  }).join(" ")}
                />
              </svg>
            </div>
          )}
        </TerminalCard>
        <TerminalCard title="R-Multiple Distribution">
          {snapshot.analytics.rMultipleBuckets.length === 0 ? (
            <TerminalNoData />
          ) : (
            <div className="grid grid-cols-5 items-end gap-2">
              {snapshot.analytics.rMultipleBuckets.map((bucket) => (
                <div key={bucket.bucket} className="text-center">
                  <div className="mx-auto w-full rounded-t bg-sky-400/70" style={{ height: `${Math.max(18, (bucket.count / maxR) * 150)}px` }} />
                  <div className="mt-2 text-[10px] text-zinc-500">{bucket.bucket}</div>
                  <div className="font-mono text-xs text-zinc-300">{bucket.count}</div>
                </div>
              ))}
            </div>
          )}
        </TerminalCard>
      </div>
      <div className="space-y-3">
        <TerminalCard title="Rolling Performance" subtitle="Portfolio accounting authority">
          <div className="grid grid-cols-2 gap-2">
            <Metric label="Sharpe 30D" value={snapshot.analytics.rollingSharpe30d > 0 ? snapshot.analytics.rollingSharpe30d.toFixed(2) : "—"} />
            <Metric label="Sharpe 90D" value={snapshot.analytics.rollingSharpe90d > 0 ? snapshot.analytics.rollingSharpe90d.toFixed(2) : "—"} />
            <Metric label="PF Trend" value={snapshot.analytics.profitFactorTrend > 0 ? snapshot.analytics.profitFactorTrend.toFixed(2) : "—"} tone="positive" />
            <Metric label="Win Rate" value={snapshot.analytics.winRatePct > 0 ? pct(snapshot.analytics.winRatePct, 1) : "—"} />
          </div>
        </TerminalCard>
        <TerminalCard title="Fee Drag">
          <Metric label="Fee Drag" value={snapshot.analytics.feeDragUsd > 0 ? usd(-snapshot.analytics.feeDragUsd) : "—"} tone="negative" />
        </TerminalCard>
        <RegimeBreakdown />
      </div>
    </div>
  );
}

function RegimeBreakdown() {
  const [rows, setRows] = useState<Array<{ regime: string; trades: number; net_pnl: number; win_rate: number }>>([]);

  useEffect(() => {
    fetch("/api/paper-trades/regime-analysis")
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (data?.ok && Array.isArray(data.regimes)) setRows(data.regimes);
      })
      .catch(() => {});
  }, []);

  return (
    <TerminalCard title="Regime Breakdown" subtitle="/api/paper-trades/regime-analysis">
      {rows.length === 0 ? (
        <TerminalNoData />
      ) : (
        <div className="space-y-2">
          {rows.map((r) => (
            <div key={r.regime} className="flex items-center justify-between rounded-lg bg-zinc-950/40 px-3 py-2 text-xs">
              <span className="text-zinc-300">{r.regime}</span>
              <span className="font-mono text-emerald-300">{usd(r.net_pnl, { signed: true })} · {pct(r.win_rate * 100, 0)} · {r.trades}t</span>
            </div>
          ))}
        </div>
      )}
    </TerminalCard>
  );
}
