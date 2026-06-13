"use client";

import Link from "next/link";
import type { TerminalAuthorityState } from "@/lib/terminal/terminalAuthority";
import { TERMINAL_ROUTES } from "@/lib/utils/navRoutes";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { Metric, TerminalCard } from "./TerminalCard";
import { pct, pnlClass, usd } from "./format";

export function CommandCenterHome({ snapshot }: { snapshot: TerminalAuthorityState }) {
  const equity =
    snapshot.analytics.equityCurve.length > 0
      ? snapshot.analytics.equityCurve[snapshot.analytics.equityCurve.length - 1]?.equity ?? 0
      : 0;
  const activeStrategies = snapshot.strategies.filter((s) => s.health === "ACTIVE").length;
  const openPositions = snapshot.positions.length;
  const hasAuthority = snapshot.hasAuthority ?? false;

  return (
    <div className="m3-page-stack">
      <div className="m3-kpi-strip">
        <Metric label="Portfolio Equity" value={equity > 0 ? usd(equity, { compact: true }) : "—"} tone={equity > 0 ? "positive" : "neutral"} />
        <Metric label="Gross Exposure" value={snapshot.risk.grossExposureUsd > 0 ? usd(snapshot.risk.grossExposureUsd, { compact: true }) : "—"} />
        <Metric label="Drawdown" value={snapshot.risk.drawdownPct !== 0 ? `${snapshot.risk.drawdownPct.toFixed(2)}%` : "—"} tone={snapshot.risk.drawdownPct < -3 ? "negative" : "warning"} />
        <Metric label="Portfolio PF" value={snapshot.analytics.profitFactorTrend != null ? snapshot.analytics.profitFactorTrend.toFixed(2) : "—"} />
        <Metric label="Sharpe" value={snapshot.analytics.rollingSharpe30d != null ? snapshot.analytics.rollingSharpe30d.toFixed(2) : "—"} />
        <Metric label="Active Strategies" value={activeStrategies > 0 ? String(activeStrategies) : "—"} />
        <Metric label="Open Positions" value={String(openPositions)} tone={openPositions > 0 ? "warning" : "neutral"} />
        <Metric label="Authority" value={hasAuthority ? "LIVE" : "STALE"} tone={hasAuthority ? "positive" : "negative"} />
      </div>

      <div className="grid gap-3 xl:grid-cols-3">
        <TerminalCard
          title="Strategy Intelligence"
          subtitle="Go engine · MongoDB strategy_scores"
          actions={<Link href={TERMINAL_ROUTES.strategies} className="m3-link-action">Open →</Link>}
        >
          {snapshot.strategies.length === 0 ? (
            <TerminalNoData label="NO STRATEGY DATA" />
          ) : (
            <div className="space-y-1">
              {snapshot.strategies.slice(0, 6).map((s) => (
                <div key={s.name} className="flex items-center justify-between rounded border border-zinc-800 px-2 py-1.5 text-xs">
                  <span className="truncate text-zinc-300">{s.name}</span>
                  <span className={s.health === "ACTIVE" ? "text-emerald-400" : s.health === "WATCHLIST" ? "text-amber-400" : "text-rose-400"}>{s.health}</span>
                </div>
              ))}
            </div>
          )}
        </TerminalCard>

        <TerminalCard
          title="Portfolio Analytics"
          subtitle="Equity curve · risk metrics"
          actions={<Link href={TERMINAL_ROUTES.portfolio} className="m3-link-action">Open →</Link>}
        >
          {snapshot.analytics.equityCurve.length === 0 ? (
            <TerminalNoData label="NO EQUITY CURVE" />
          ) : (
            <div className="grid grid-cols-2 gap-2">
              <Metric label="Win Rate" value={snapshot.analytics.winRatePct != null ? `${snapshot.analytics.winRatePct.toFixed(1)}%` : "—"} />
              <Metric label="Fee Drag" value={snapshot.analytics.feeDragUsd != null ? usd(snapshot.analytics.feeDragUsd) : "—"} tone="warning" />
              <Metric label="Heat" value={`${snapshot.risk.heatPct.toFixed(1)}%`} />
              <Metric label="Margin" value={`${snapshot.risk.marginUsagePct.toFixed(1)}%`} />
            </div>
          )}
        </TerminalCard>

        <TerminalCard
          title="Risk Metrics"
          subtitle="VaR · exposure · drawdown"
          actions={<Link href={TERMINAL_ROUTES.risk} className="m3-link-action">Open →</Link>}
        >
          {!hasAuthority && snapshot.risk.grossExposureUsd === 0 ? (
            <TerminalNoData />
          ) : (
            <div className="grid grid-cols-2 gap-2">
              <Metric label="VaR 95" value={snapshot.risk.var95Usd !== 0 ? usd(-snapshot.risk.var95Usd) : "—"} tone="warning" />
              <Metric label="Net Exp" value={usd(snapshot.risk.netExposureUsd, { compact: true })} />
              <Metric label="Long" value={usd(snapshot.risk.longExposureUsd, { compact: true })} />
              <Metric label="Short" value={usd(snapshot.risk.shortExposureUsd, { compact: true })} />
            </div>
          )}
        </TerminalCard>
      </div>

      <div className="grid gap-3 xl:grid-cols-2">
        <TerminalCard
          title="Recent Events"
          subtitle="Platform event stream"
          actions={<Link href={TERMINAL_ROUTES.events} className="m3-link-action">Console →</Link>}
        >
          {snapshot.alerts.length === 0 ? (
            <TerminalNoData label="NO RECENT EVENTS" />
          ) : (
            <div className="max-h-48 space-y-1 overflow-y-auto">
              {snapshot.alerts.slice(0, 8).map((a) => (
                <div key={a.id} className="rounded border border-zinc-800 px-2 py-1.5 text-xs">
                  <span className={a.severity === "CRITICAL" ? "text-rose-400" : a.severity === "WARNING" ? "text-amber-400" : "text-sky-400"}>{a.severity}</span>
                  <span className="ml-2 text-zinc-300">{a.title}</span>
                </div>
              ))}
            </div>
          )}
        </TerminalCard>

        <TerminalCard
          title="Open Positions"
          subtitle="Live marks from engine"
          actions={<Link href={TERMINAL_ROUTES.execution} className="m3-link-action">Execution →</Link>}
        >
          {snapshot.positions.length === 0 ? (
            <TerminalNoData label="NO OPEN POSITIONS" />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead className="text-left text-[10px] uppercase tracking-wider text-zinc-500">
                  <tr>
                    <th className="py-1">Strategy</th>
                    <th>Side</th>
                    <th className="text-right">PnL</th>
                  </tr>
                </thead>
                <tbody>
                  {snapshot.positions.slice(0, 6).map((p) => (
                    <tr key={p.id} className="border-t border-zinc-800 font-mono">
                      <td className="max-w-[140px] truncate py-1 text-zinc-300">{p.strategy}</td>
                      <td className={p.side === "LONG" ? "text-emerald-400" : "text-rose-400"}>{p.side}</td>
                      <td className={`text-right ${pnlClass(p.unrealizedPnl)}`}>{usd(p.unrealizedPnl, { signed: true })}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </TerminalCard>
      </div>

      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
        <QuickLink href={TERMINAL_ROUTES.execution} label="Execution Center" detail="Order book · positions · chart" />
        <QuickLink href={TERMINAL_ROUTES.observability} label="Observability" detail="API · engine · latency · feeds" />
        <QuickLink href={TERMINAL_ROUTES.health} label="System Health" detail="Mongo · OMS · reconciliation" />
        <QuickLink href={TERMINAL_ROUTES.diagnostics} label="Diagnostics" detail="Env · worker · engine probes" />
      </div>

      {snapshot.regime ? (
        <p className="text-[10px] uppercase tracking-wider text-zinc-500">
          Market regime: <span className="text-sky-300">{snapshot.regime}</span>
          {snapshot.price > 0 ? <> · Mark {usd(snapshot.price)} · 24h {pct(snapshot.priceChange24hPct)}</> : null}
        </p>
      ) : null}
    </div>
  );
}

function QuickLink({ href, label, detail }: { href: string; label: string; detail: string }) {
  return (
    <Link
      href={href}
      className="m3-quick-link"
    >
      <div className="text-xs font-semibold text-zinc-200">{label}</div>
      <div className="mt-0.5 text-[10px] text-zinc-500">{detail}</div>
    </Link>
  );
}
