"use client";

import type { TerminalSnapshot } from "@/lib/terminal/terminalTypes";
import { pct, usd } from "./format";
import { Metric, TerminalCard } from "./TerminalCard";

const families = [
  { name: "Funding", exposure: 24, heat: 1.1 },
  { name: "Order Flow", exposure: 19, heat: 0.8 },
  { name: "Smart Money", exposure: 13, heat: 0.6 },
  { name: "Volume Profile", exposure: 11, heat: 0.5 },
];

export function RiskModule({ snapshot }: { snapshot: TerminalSnapshot }) {
  return (
    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_420px]">
      <div className="space-y-3">
        <div className="grid gap-3 md:grid-cols-4">
          <Metric label="VaR 95" value={usd(-snapshot.risk.var95Usd)} tone="warning" />
          <Metric label="VaR 99" value={usd(-snapshot.risk.var99Usd)} tone="negative" />
          <Metric label="CVaR 95" value={usd(-snapshot.risk.cvar95Usd)} tone="negative" />
          <Metric label="Drawdown" value={`${snapshot.risk.drawdownPct.toFixed(2)}%`} tone="positive" />
        </div>
        <TerminalCard title="Portfolio Heat" subtitle="0-4 normal · 4-6 reduce · 6+ block">
          <div className="h-5 overflow-hidden rounded-full bg-zinc-950">
            <div className="h-full rounded-full bg-emerald-400" style={{ width: `${Math.min(100, snapshot.risk.heatPct * 10)}%` }} />
          </div>
          <div className="mt-3 grid gap-2 md:grid-cols-4">
            <Metric label="Current Heat" value={`${snapshot.risk.heatPct.toFixed(1)}%`} />
            <Metric label="Margin Usage" value={`${snapshot.risk.marginUsagePct.toFixed(1)}%`} />
            <Metric label="Long Exp" value={usd(snapshot.risk.longExposureUsd, { compact: true })} />
            <Metric label="Short Exp" value={usd(snapshot.risk.shortExposureUsd, { compact: true })} />
          </div>
        </TerminalCard>
        <TerminalCard title="Correlation Heatmap" subtitle="Strategy clustering and hidden BTC concentration">
          <div className="grid grid-cols-5 gap-1 text-center font-mono text-[11px]">
            {["Funding", "CVD", "Sweep", "VP", "MSS"].map((row, rowIdx) =>
              ["Funding", "CVD", "Sweep", "VP", "MSS"].map((col, colIdx) => {
                const value = rowIdx === colIdx ? 1 : Math.max(0.12, 0.82 - Math.abs(rowIdx - colIdx) * 0.18);
                return (
                  <div key={`${row}-${col}`} className={`rounded px-1 py-3 ${value > 0.7 ? "bg-rose-500/20 text-rose-200" : value > 0.45 ? "bg-amber-500/15 text-amber-200" : "bg-emerald-500/10 text-emerald-200"}`}>
                    {value.toFixed(2)}
                  </div>
                );
              }),
            )}
          </div>
        </TerminalCard>
        <TerminalCard title="Exposure By Family">
          <div className="space-y-3">
            {families.map((family) => (
              <div key={family.name}>
                <div className="mb-1 flex justify-between text-xs">
                  <span className="text-zinc-300">{family.name}</span>
                  <span className="font-mono text-zinc-500">{family.exposure}% allocation · heat {family.heat.toFixed(1)}%</span>
                </div>
                <div className="h-2 rounded-full bg-zinc-950">
                  <div className="h-2 rounded-full bg-sky-400" style={{ width: `${family.exposure}%` }} />
                </div>
              </div>
            ))}
          </div>
        </TerminalCard>
      </div>
      <div className="space-y-3">
        <TerminalCard title="Kelly Sizing Matrix">
          <div className="space-y-2">
            {snapshot.strategies.slice(0, 4).map((strategy) => (
              <div key={strategy.id} className="rounded-lg border border-zinc-800 bg-zinc-950/40 p-3">
                <div className="flex justify-between gap-3">
                  <span className="text-xs font-medium text-zinc-200">{strategy.name}</span>
                  <span className="font-mono text-xs text-emerald-300">{Math.max(0.1, strategy.promotionScore / 100).toFixed(2)}%</span>
                </div>
                <div className="mt-1 text-[11px] text-zinc-500">PF {strategy.oosProfitFactor.toFixed(2)} · Sharpe {strategy.sharpe.toFixed(2)} · DD {strategy.maxDrawdownPct.toFixed(1)}%</div>
              </div>
            ))}
          </div>
        </TerminalCard>
        <TerminalCard title="Funding Exposure">
          <div className="grid grid-cols-2 gap-2">
            <Metric label="Paid" value={usd(-snapshot.risk.fundingPaidUsd)} tone="negative" />
            <Metric label="Received" value={usd(snapshot.risk.fundingReceivedUsd)} tone="positive" />
          </div>
          <div className="mt-3 space-y-2">
            {snapshot.positions.map((position) => (
              <div key={position.id} className="flex justify-between rounded-lg bg-zinc-950/40 px-3 py-2 text-xs">
                <span className="text-zinc-300">{position.strategy}</span>
                <span className="font-mono text-amber-300">{pct(position.fundingRate * 100, 4)} · {usd(position.fundingPnl, { signed: true })}</span>
              </div>
            ))}
          </div>
        </TerminalCard>
      </div>
    </div>
  );
}
