"use client";

import Link from "next/link";
import type { TerminalAuthorityState } from "@/lib/terminal/terminalAuthority";
import { TERMINAL_ROUTES } from "@/lib/utils/navRoutes";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { Metric, TerminalCard } from "./TerminalCard";
import { pct, pnlClass, usd } from "./format";
import { Sparkline } from "@/components/ui/Sparkline";

export function CommandCenterHome({ snapshot }: { snapshot: TerminalAuthorityState }) {
  const equity =
    snapshot.analytics.equityCurve.length > 0
      ? snapshot.analytics.equityCurve[snapshot.analytics.equityCurve.length - 1]?.equity ?? 0
      : 0;
  const equityCurveValues = snapshot.analytics.equityCurve
    .map((point) => point.equity)
    .filter((value) => Number.isFinite(value));
  const activeStrategies = snapshot.strategies.filter((s) => s.health === "ACTIVE").length;
  const openPositions = snapshot.positions.length;
  const hasAuthority = snapshot.hasAuthority ?? false;
  const pastInitialLoad = !snapshot.loading;
  const freshnessLabel = formatFreshness(snapshot.updatedAt);
  const realizedPnl = snapshot.journal.reduce((sum, trade) => sum + trade.netPnl, 0);
  const riskState = snapshot.risk.heatPct > 70 ? "Reduce risk" : snapshot.risk.heatPct > 40 ? "Watch" : "Normal";

  return (
    <div className="m3-page-stack">
      <TerminalCard
        title="Pre-Live Trade Engine"
        subtitle="100 backtested-qualified strategies · real broker · no position limits"
        actions={<Link href={TERMINAL_ROUTES["pre-live-engine"]} className="m3-link-action">Open Pre-Live Engine</Link>}
      >
        <div className="m3-kpi-strip">
          <Metric label="Strategies" value="100 Qualified" tone="positive" />
          <Metric label="Position Limit" value="Unlimited" tone="positive" />
          <Metric label="Cooldown" value="None" tone="positive" />
          <Metric label="Broker" value="Real Money" tone="warning" />
          <Metric label="Risk Gate" value="SL/TP Only" tone="positive" />
        </div>
      </TerminalCard>

      <TerminalCard
        title="Trade Engine"
        subtitle="Live BTC paper trading engine"
        actions={<Link href={TERMINAL_ROUTES["trade-engine"]} className="m3-link-action">Open Trade Engine</Link>}
      >
        <div className="m3-kpi-strip">
          <Metric label="Engine Status" value={hasAuthority ? "Live" : "Updating"} tone={hasAuthority ? "positive" : "warning"} />
          <Metric label="Equity" value={usd(equity, { compact: true })} tone={equity > 0 ? "positive" : "neutral"} />
          <Metric label="BTC Mark" value={snapshot.price > 0 ? usd(snapshot.price) : "$0.00"} tone={snapshot.price > 0 ? "positive" : "neutral"} />
          <Metric label="Realized PnL" value={usd(realizedPnl, { signed: true, compact: true })} tone={realizedPnl >= 0 ? "positive" : "negative"} />
          <Metric label="Risk State" value={riskState} tone={riskState === "Normal" ? "positive" : riskState === "Watch" ? "warning" : "negative"} />
        </div>
      </TerminalCard>

      {!hasAuthority && pastInitialLoad ? (
        <div className="icc-authority-banner" role="status">
          Trade Engine data is updating
        </div>
      ) : null}

      <div className="m3-kpi-strip">
        <Metric label="Portfolio Equity" value={usd(equity, { compact: true })} tone={equity > 0 ? "positive" : "neutral"} size="lg">
          <Sparkline
            data={equityCurveValues}
            width={88}
            height={28}
            color={
              equityCurveValues.length < 2 || equityCurveValues[equityCurveValues.length - 1] >= equityCurveValues[0]
                ? "var(--green)"
                : "var(--red)"
            }
          />
        </Metric>
        <Metric label="Gross Exposure" value={usd(snapshot.risk.grossExposureUsd, { compact: true })} />
        <Metric label="Drawdown" value={`${snapshot.risk.drawdownPct.toFixed(2)}%`} tone={snapshot.risk.drawdownPct < -3 ? "negative" : "warning"} />
        <Metric label="Portfolio PF" value={snapshot.analytics.profitFactorTrend != null ? snapshot.analytics.profitFactorTrend.toFixed(2) : "—"} />
        <Metric label="Sharpe" value={snapshot.analytics.rollingSharpe30d != null ? snapshot.analytics.rollingSharpe30d.toFixed(2) : "—"} />
        <Metric label="Active Strategies" value={String(activeStrategies)} />
        <Metric label="Open Positions" value={String(openPositions)} tone={openPositions > 0 ? "warning" : "neutral"} />
        <Metric label="Authority" value={hasAuthority ? "Live" : "Stale"} tone={hasAuthority ? "positive" : "negative"} />
      </div>

      <div className="grid gap-3 xl:grid-cols-3">
        <TerminalCard
          title="Strategy Performance"
          subtitle="Live strategy health and leadership"
          actions={<Link href={TERMINAL_ROUTES["trade-engine"]} className="m3-link-action">View</Link>}
        >
          {snapshot.strategies.length === 0 ? (
            <StrategyWaitingState />
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
          <CardFreshness label={freshnessLabel} />
        </TerminalCard>

        <TerminalCard
          title="Portfolio"
          subtitle="Equity curve · risk metrics"
          actions={<Link href={TERMINAL_ROUTES.portfolio} className="m3-link-action">View</Link>}
        >
          {snapshot.analytics.equityCurve.length === 0 ? (
            <TerminalNoData label="No equity curve" />
          ) : (
            <div className="grid grid-cols-2 gap-2">
              <Metric label="Win Rate" value={snapshot.analytics.winRatePct != null ? `${snapshot.analytics.winRatePct.toFixed(1)}%` : "—"} />
              <Metric label="Fee Drag" value={snapshot.analytics.feeDragUsd != null ? usd(snapshot.analytics.feeDragUsd) : "—"} tone="warning" />
              <Metric label="Heat" value={`${snapshot.risk.heatPct.toFixed(1)}%`} />
              <Metric label="Margin" value={`${snapshot.risk.marginUsagePct.toFixed(1)}%`} />
            </div>
          )}
          <CardFreshness label={freshnessLabel} />
        </TerminalCard>

        <TerminalCard
          title="Risk"
          subtitle="VaR · exposure · drawdown"
          actions={<Link href={TERMINAL_ROUTES.risk} className="m3-link-action">View</Link>}
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
          <CardFreshness label={freshnessLabel} />
        </TerminalCard>
      </div>

      <div className="grid gap-3 xl:grid-cols-2">
        <TerminalCard
          title="Recent Events"
          subtitle="Platform event stream"
          actions={<Link href={TERMINAL_ROUTES.events} className="m3-link-action">View</Link>}
        >
          {snapshot.alerts.length === 0 ? (
            <TerminalNoData label="No recent events" />
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
          subtitle="Live marks from Trade Engine"
          actions={<Link href={TERMINAL_ROUTES.execution} className="m3-link-action">View</Link>}
        >
          {snapshot.positions.length === 0 ? (
            <TerminalNoData label="No open positions" />
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
        <QuickLink href={TERMINAL_ROUTES["pre-live-engine"]} label="Pre-Live Engine" detail="100 strategies · real money · no limits" />
        <QuickLink href={TERMINAL_ROUTES["trade-engine"]} label="Trade Engine" detail="Positions · trades · live signals" />
        <QuickLink href={TERMINAL_ROUTES.portfolio} label="Portfolio" detail="Equity · exposure · fees" />
        <QuickLink href={TERMINAL_ROUTES.risk} label="Risk" detail="VaR · heat · margin" />
        <QuickLink href={TERMINAL_ROUTES.health} label="Events & Health" detail="Alerts · system checks" />
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

function formatFreshness(updatedAt: string) {
  const parsed = Date.parse(updatedAt);
  if (!Number.isFinite(parsed)) return "Updated just now";

  const minutesAgo = Math.max(0, Math.floor((Date.now() - parsed) / 60_000));
  if (minutesAgo < 1) return "Updated just now";
  return `Updated ${minutesAgo}m ago`;
}

function StrategyWaitingState() {
  return (
    <div className="icc-waiting-state" role="status">
      <span className="icc-pulse-dot" aria-hidden />
      <span>Waiting for strategy performance data</span>
    </div>
  );
}

function CardFreshness({ label }: { label: string }) {
  return <div className="icc-card-updated">{label}</div>;
}

function QuickLink({ href, label, detail }: { href: string; label: string; detail: string }) {
  return (
    <Link
      href={href}
      className="m3-quick-link"
    >
      <div className="text-xs font-semibold text-slate-800">{label}</div>
      <div className="mt-0.5 text-[11px] text-slate-500">{detail}</div>
    </Link>
  );
}
