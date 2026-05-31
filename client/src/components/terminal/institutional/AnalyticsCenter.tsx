"use client";

import type { TerminalSnapshot } from "@/lib/terminal/terminalTypes";
import { pct, usd } from "./format";
import { Metric, TerminalCard } from "./TerminalCard";

export function AnalyticsCenter({ snapshot }: { snapshot: TerminalSnapshot }) {
  const maxR = Math.max(...snapshot.analytics.rMultipleBuckets.map((b) => b.count));
  return (
    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_360px]">
      <div className="space-y-3">
        <TerminalCard title="Equity Curve vs BTC Benchmark" subtitle="Desk equity compared to passive BTC exposure">
          <div className="h-72 rounded-lg border border-zinc-800 bg-zinc-950/40 p-3">
            <svg viewBox="0 0 700 240" className="h-full w-full" role="img" aria-label="Equity curve compared to BTC benchmark">
              <polyline
                fill="none"
                stroke="rgb(52 211 153)"
                strokeWidth="3"
                points={snapshot.analytics.equityCurve.map((p, i) => `${40 + i * 120},${210 - (p.equity - 100000) / 8}`).join(" ")}
              />
              <polyline
                fill="none"
                stroke="rgb(56 189 248)"
                strokeWidth="2"
                strokeDasharray="6 6"
                points={snapshot.analytics.equityCurve.map((p, i) => `${40 + i * 120},${210 - (p.btcBenchmark - 100000) / 8}`).join(" ")}
              />
              {snapshot.analytics.equityCurve.map((p, i) => (
                <text key={p.time} x={40 + i * 120} y="232" fill="rgb(113 113 122)" fontSize="11" textAnchor="middle">
                  {p.time}
                </text>
              ))}
            </svg>
          </div>
          <div className="mt-2 flex gap-4 text-xs">
            <span className="text-emerald-300">Desk equity</span>
            <span className="text-sky-300">BTC buy & hold</span>
          </div>
        </TerminalCard>
        <TerminalCard title="R-Multiple Distribution">
          <div className="grid grid-cols-5 items-end gap-2">
            {snapshot.analytics.rMultipleBuckets.map((bucket) => (
              <div key={bucket.bucket} className="text-center">
                <div className="mx-auto w-full rounded-t bg-sky-400/70" style={{ height: `${Math.max(18, (bucket.count / maxR) * 150)}px` }} />
                <div className="mt-2 text-[10px] text-zinc-500">{bucket.bucket}</div>
                <div className="font-mono text-xs text-zinc-300">{bucket.count}</div>
              </div>
            ))}
          </div>
        </TerminalCard>
      </div>
      <div className="space-y-3">
        <TerminalCard title="Rolling Performance">
          <div className="grid grid-cols-2 gap-2">
            <Metric label="Sharpe 30D" value={snapshot.analytics.rollingSharpe30d.toFixed(2)} />
            <Metric label="Sharpe 90D" value={snapshot.analytics.rollingSharpe90d.toFixed(2)} />
            <Metric label="PF Trend" value={snapshot.analytics.profitFactorTrend.toFixed(2)} tone="positive" />
            <Metric label="Win Rate" value={pct(snapshot.analytics.winRatePct, 1)} />
          </div>
        </TerminalCard>
        <TerminalCard title="Fee & Funding Drag">
          <Metric label="Fee Drag" value={usd(-snapshot.analytics.feeDragUsd)} tone="negative" />
          <div className="mt-3 text-xs text-zinc-400">
            Fee drag is displayed beside funding drag in execution and risk modules so expectancy remains net-of-cost.
          </div>
        </TerminalCard>
        <TerminalCard title="Family Breakdown">
          <div className="space-y-2">
            {["Funding", "Order Flow", "Smart Money", "Volume Profile"].map((family, i) => (
              <div key={family} className="flex items-center justify-between rounded-lg bg-zinc-950/40 px-3 py-2 text-xs">
                <span className="text-zinc-300">{family}</span>
                <span className="font-mono text-emerald-300">{usd(320 - i * 78, { signed: true })}</span>
              </div>
            ))}
          </div>
        </TerminalCard>
      </div>
    </div>
  );
}
