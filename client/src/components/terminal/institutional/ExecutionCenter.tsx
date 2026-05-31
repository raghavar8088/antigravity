"use client";

import { InstitutionalChart } from "@/components/terminal/InstitutionalChart";
import type { TerminalSnapshot } from "@/lib/terminal/terminalTypes";
import { pct, pnlClass, px, usd } from "./format";
import { Metric, TerminalCard } from "./TerminalCard";

export function ExecutionCenter({ snapshot }: { snapshot: TerminalSnapshot }) {
  const maxDepth = Math.max(...snapshot.bids.map((l) => l.total), ...snapshot.asks.map((l) => l.total));

  return (
    <div className="grid gap-3 xl:grid-cols-[280px_minmax(0,1fr)_320px]">
      <div className="space-y-3">
        <TerminalCard title="Order Book" subtitle="Top depth · DOM ladder">
          <div className="space-y-1">
            {[...snapshot.asks].reverse().slice(-10).map((level) => (
              <DepthRow key={`ask-${level.price}`} level={level} maxDepth={maxDepth} side="ask" />
            ))}
            <div className="my-2 rounded-md bg-zinc-900 px-2 py-2 text-center font-mono text-sm text-zinc-100">
              ${px(snapshot.price)}
            </div>
            {snapshot.bids.slice(0, 10).map((level) => (
              <DepthRow key={`bid-${level.price}`} level={level} maxDepth={maxDepth} side="bid" />
            ))}
          </div>
        </TerminalCard>
        <TerminalCard title="Portfolio Summary">
          <div className="grid grid-cols-2 gap-2">
            <Metric label="Net Exp" value={usd(snapshot.risk.netExposureUsd, { compact: true })} />
            <Metric label="Gross Exp" value={usd(snapshot.risk.grossExposureUsd, { compact: true })} />
            <Metric label="Margin" value={`${snapshot.risk.marginUsagePct.toFixed(1)}%`} tone="warning" />
            <Metric label="Heat" value={`${snapshot.risk.heatPct.toFixed(1)}%`} tone="positive" />
          </div>
        </TerminalCard>
      </div>

      <div className="space-y-3">
        <TerminalCard
          title="BTCUSD Perpetual Chart"
          subtitle="Candles · volume profile-ready · liquidation/position overlays"
          actions={<span className="rounded bg-zinc-900 px-2 py-1 text-[10px] text-zinc-400">1m / 5m / 15m / 1h</span>}
        >
          <InstitutionalChart candles={snapshot.candles} height={520} title="BTCUSD · PERP · Execution" />
        </TerminalCard>
        <TerminalCard title="Open Positions" subtitle="Tick-by-tick PnL · liquidation · funding exposure">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px] text-xs">
              <thead className="text-left text-[10px] uppercase tracking-[0.12em] text-zinc-500">
                <tr>
                  <th className="py-2">Side</th>
                  <th>Strategy</th>
                  <th className="text-right">Entry</th>
                  <th className="text-right">Mark</th>
                  <th className="text-right">Liq</th>
                  <th className="text-right">Size</th>
                  <th className="text-right">Funding</th>
                  <th className="text-right">Live PnL</th>
                </tr>
              </thead>
              <tbody>
                {snapshot.positions.map((p) => (
                  <tr key={p.id} className="border-t border-zinc-800 font-mono">
                    <td className={p.side === "LONG" ? "py-2 text-emerald-300" : "py-2 text-rose-300"}>{p.side}</td>
                    <td className="max-w-[220px] truncate text-zinc-300">{p.strategy}</td>
                    <td className="text-right text-zinc-400">{px(p.entryPrice)}</td>
                    <td className="text-right text-zinc-100">{px(p.markPrice)}</td>
                    <td className="text-right text-amber-300">{px(p.liquidationPrice)}</td>
                    <td className="text-right text-zinc-300">{p.sizeBtc.toFixed(4)}</td>
                    <td className="text-right text-zinc-400">{pct(p.fundingRate * 100, 4)}</td>
                    <td className={`text-right font-semibold ${pnlClass(p.unrealizedPnl)}`}>{usd(p.unrealizedPnl, { signed: true })}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </TerminalCard>
      </div>

      <div className="space-y-3">
        <QuickTradePanel />
        <TerminalCard title="Alert Tape" subtitle="Execution and risk events">
          <div className="space-y-2">
            {snapshot.alerts.map((alert) => (
              <div key={alert.id} className="rounded-lg border border-zinc-800 bg-zinc-950/40 p-2">
                <div className={alert.severity === "CRITICAL" ? "text-xs font-semibold text-rose-300" : alert.severity === "WARNING" ? "text-xs font-semibold text-amber-300" : "text-xs font-semibold text-sky-300"}>
                  {alert.severity} · {alert.title}
                </div>
                <p className="mt-1 text-xs text-zinc-400">{alert.message}</p>
              </div>
            ))}
          </div>
        </TerminalCard>
      </div>
    </div>
  );
}

function DepthRow({ level, maxDepth, side }: { level: { price: number; size: number; total: number }; maxDepth: number; side: "bid" | "ask" }) {
  const width = `${Math.max(6, (level.total / maxDepth) * 100)}%`;
  return (
    <div className="relative overflow-hidden rounded px-2 py-1 font-mono text-[11px]">
      <div className={`absolute inset-y-0 right-0 ${side === "bid" ? "bg-emerald-500/10" : "bg-rose-500/10"}`} style={{ width }} />
      <div className="relative flex justify-between gap-2">
        <span className={side === "bid" ? "text-emerald-300" : "text-rose-300"}>{px(level.price)}</span>
        <span className="text-zinc-400">{level.size.toFixed(3)}</span>
        <span className="text-zinc-500">{level.total.toFixed(2)}</span>
      </div>
    </div>
  );
}

function QuickTradePanel() {
  return (
    <TerminalCard title="Quick Trade" subtitle="Hotkeys: B buy · S sell · X close">
      <div className="grid grid-cols-2 gap-2">
        <button className="rounded-lg bg-emerald-500/15 px-3 py-4 text-sm font-bold uppercase tracking-[0.14em] text-emerald-200 hover:bg-emerald-500/25">
          Buy
        </button>
        <button className="rounded-lg bg-rose-500/15 px-3 py-4 text-sm font-bold uppercase tracking-[0.14em] text-rose-200 hover:bg-rose-500/25">
          Sell
        </button>
        <button className="rounded-lg border border-zinc-700 px-3 py-3 text-xs font-semibold uppercase tracking-[0.12em] text-zinc-300">
          Close Longs
        </button>
        <button className="rounded-lg border border-zinc-700 px-3 py-3 text-xs font-semibold uppercase tracking-[0.12em] text-zinc-300">
          Close Shorts
        </button>
      </div>
      <div className="mt-3 grid grid-cols-3 gap-2 text-xs">
        <Metric label="Size" value="0.05 BTC" />
        <Metric label="Lev" value="3x" />
        <Metric label="SL/TP" value="0.35/0.70%" />
      </div>
      <button className="mt-3 w-full rounded-lg bg-amber-500/15 px-3 py-3 text-xs font-bold uppercase tracking-[0.14em] text-amber-200">
        Arm One-Click Execution
      </button>
    </TerminalCard>
  );
}
