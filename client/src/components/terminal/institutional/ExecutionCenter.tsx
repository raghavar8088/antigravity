"use client";

import Link from "next/link";
import { InstitutionalChart } from "@/components/terminal/InstitutionalChart";
import type { TerminalSnapshot, TerminalPosition } from "@/lib/terminal/terminalTypes";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { pct, pnlClass, px, usd } from "./format";
import { Metric, TerminalCard } from "./TerminalCard";
import { PageHeader } from "@/components/ui/PageHeader";
import { Badge } from "@/components/ui/StatusChip";
import { DataTable, type DataTableColumn } from "@/components/ui/DataTable";

export function ExecutionCenter({ snapshot }: { snapshot: TerminalSnapshot }) {
  const maxDepth = Math.max(
    1,
    ...snapshot.bids.map((l) => l.total),
    ...snapshot.asks.map((l) => l.total),
  );
  const hasBook = snapshot.bids.length > 0 || snapshot.asks.length > 0;
  const hasPrice = snapshot.price > 0;

  const positionColumns: DataTableColumn<TerminalPosition>[] = [
    { id: "side", header: "Side", width: "70px", cell: (p) => <Badge variant={p.side === "LONG" ? "profit" : "loss"} size="sm">{p.side}</Badge> },
    { id: "strategy", header: "Strategy", sortable: true, sortValue: (p) => p.strategy, cell: (p) => p.strategy },
    { id: "entry", header: "Entry", align: "right", cell: (p) => px(p.entryPrice) },
    { id: "mark", header: "Mark", align: "right", cell: (p) => px(p.markPrice) },
    { id: "liq", header: "Liq", align: "right", cell: (p) => <span style={{ color: "var(--amber)" }}>{px(p.liquidationPrice)}</span> },
    { id: "size", header: "Size", align: "right", sortable: true, sortValue: (p) => p.sizeBtc, cell: (p) => p.sizeBtc.toFixed(4) },
    { id: "funding", header: "Funding", align: "right", cell: (p) => pct(p.fundingRate * 100, 4) },
    {
      id: "pnl",
      header: "Live PnL",
      align: "right",
      sortable: true,
      sortValue: (p) => p.unrealizedPnl,
      cell: (p) => <span className={`font-semibold ${pnlClass(p.unrealizedPnl)}`}>{usd(p.unrealizedPnl, { signed: true })}</span>,
    },
  ];

  return (
    <div className="m3-page-stack">
      <PageHeader title="Execution" subtitle="Order book · live chart · positions · alert tape" />
      <div className="grid gap-3 xl:grid-cols-[280px_minmax(0,1fr)_320px]">
      <div className="space-y-3">
        <TerminalCard title="Order Book" subtitle="Live depth — requires WS feed">
          {!hasBook ? (
            <TerminalNoData label="Order book unavailable" />
          ) : (
            <div className="space-y-1">
              {[...snapshot.asks].reverse().slice(-10).map((level) => (
                <DepthRow key={`ask-${level.price}`} level={level} maxDepth={maxDepth} side="ask" />
              ))}
              <div className="my-2 rounded-full border border-slate-200 bg-white px-2 py-2 text-center font-mono text-sm font-semibold text-slate-900">
                {hasPrice ? `$${px(snapshot.price)}` : "$0.00"}
              </div>
              {snapshot.bids.slice(0, 10).map((level) => (
                <DepthRow key={`bid-${level.price}`} level={level} maxDepth={maxDepth} side="bid" />
              ))}
            </div>
          )}
        </TerminalCard>
        <TerminalCard title="Portfolio Summary" subtitle="MongoDB authority">
          {snapshot.risk.grossExposureUsd === 0 && snapshot.risk.drawdownPct === 0 ? (
            <TerminalNoData label="No portfolio exposure" />
          ) : (
            <div className="grid grid-cols-2 gap-2">
              <Metric label="Net Exp" value={usd(snapshot.risk.netExposureUsd, { compact: true })} />
              <Metric label="Gross Exp" value={usd(snapshot.risk.grossExposureUsd, { compact: true })} />
              <Metric label="Margin" value={`${snapshot.risk.marginUsagePct.toFixed(1)}%`} tone="warning" />
              <Metric label="Heat" value={`${snapshot.risk.heatPct.toFixed(1)}%`} tone="positive" />
            </div>
          )}
        </TerminalCard>
      </div>

      <div className="space-y-3">
        <TerminalCard
          title="BTCUSD Perpetual Chart"
          subtitle={hasPrice ? "Mark price from /api/btc/price" : "Awaiting market data"}
        >
          {snapshot.candles.length > 0 ? (
            <InstitutionalChart candles={snapshot.candles} height={520} title="BTCUSD · PERP · Execution" />
          ) : (
            <TerminalNoData label={hasPrice ? `Mark ${px(snapshot.price)} - candle feed unavailable` : "No market data"} />
          )}
        </TerminalCard>
        <TerminalCard title="Open Positions" subtitle="Trade Engine positions">
          {snapshot.positions.length === 0 ? (
            <TerminalNoData label="No open positions" />
          ) : (
            <DataTable columns={positionColumns} rows={snapshot.positions} getRowKey={(p) => p.id} density="compact" />
          )}
        </TerminalCard>
      </div>

      <div className="space-y-3">
        <TerminalCard title="Execution Authority" subtitle="Trade Engine is the sole execution surface">
          <p className="text-xs text-slate-500">
            All positions and fills originate from the engine OMS.
          </p>
          <Link href="/terminal/events" className="mt-3 inline-block text-xs font-semibold text-blue-600 hover:text-blue-700">
            Open Events
          </Link>
        </TerminalCard>
        <TerminalCard title="Alert Tape" subtitle="Risk and health events">
          {snapshot.alerts.length === 0 ? (
            <TerminalNoData label="No alerts" />
          ) : (
            <div className="space-y-2">
              {snapshot.alerts.map((alert) => (
                <div key={alert.id} className="rounded-lg border border-zinc-800 bg-zinc-950/40 p-2">
                  <div className="flex items-center gap-2">
                    <Badge variant={alert.severity === "CRITICAL" ? "loss" : alert.severity === "WARNING" ? "warning" : "info"} size="sm">{alert.severity}</Badge>
                    <span className="text-xs font-semibold text-zinc-200">{alert.title}</span>
                  </div>
                  <p className="mt-1 text-xs text-zinc-400">{alert.message}</p>
                </div>
              ))}
            </div>
          )}
        </TerminalCard>
      </div>
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
