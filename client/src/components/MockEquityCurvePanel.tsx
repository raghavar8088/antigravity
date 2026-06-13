"use client";

import { useMemo } from "react";
import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import {
  buildEquityCurve,
  computeDailyPnl,
  computeMonthlyPnl,
  computeExtendedMetrics,
} from "@/lib/analytics/mockExtendedMetrics";
import {
  DeskCard,
  DeskMetricTile,
  DeskSectionHeader,
  DeskChip,
} from "@/components/desk/ui";

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtUsd(v: number): string {
  if (!Number.isFinite(v)) return "—";
  const abs = Math.abs(v);
  if (abs >= 1_000_000) return `${v >= 0 ? "+" : ""}$${(v / 1_000_000).toFixed(2)}M`;
  if (abs >= 1_000) return `${v >= 0 ? "+" : ""}$${(v / 1_000).toFixed(1)}K`;
  return `${v >= 0 ? "+" : ""}$${v.toFixed(0)}`;
}

function fmtPct(v: number): string {
  if (!Number.isFinite(v)) return "—";
  return `${v >= 0 ? "+" : ""}${(v * 100).toFixed(2)}%`;
}

function fmtDate(ts: number): string {
  return new Date(ts).toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function fmtMonth(monthStr: string): string {
  const [y, m] = monthStr.split("-");
  const date = new Date(Number(y), Number(m) - 1, 1);
  return date.toLocaleDateString("en-US", { month: "short", year: "2-digit" });
}

// ── Mini sparkline (SVG) ──────────────────────────────────────────────────────

function Sparkline({
  values,
  color,
  height = 48,
  width = "100%",
}: {
  values: number[];
  color: string;
  height?: number;
  width?: string | number;
}) {
  if (values.length < 2) return null;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const pts = values
    .map((v, i) => {
      const x = (i / (values.length - 1)) * 100;
      const y = height - ((v - min) / range) * (height - 4) - 2;
      return `${x},${y}`;
    })
    .join(" ");

  return (
    <svg
      viewBox={`0 0 100 ${height}`}
      preserveAspectRatio="none"
      style={{ width, height, display: "block" }}
    >
      <polyline
        points={pts}
        fill="none"
        stroke={color}
        strokeWidth="1.5"
        strokeLinejoin="round"
        strokeLinecap="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}

// ── Daily PnL bar chart ───────────────────────────────────────────────────────

function DailyPnlBars({ dailyPnl }: { dailyPnl: ReturnType<typeof computeDailyPnl> }) {
  const last30 = dailyPnl.slice(-30);
  if (last30.length === 0) return <p className="text-xs text-[var(--desk-muted)] py-2">No closed trades yet.</p>;
  const maxAbs = Math.max(...last30.map((d) => Math.abs(d.pnl)), 1);

  return (
    <div className="flex items-end gap-[2px] h-12 w-full">
      {last30.map((day, i) => {
        const frac = Math.abs(day.pnl) / maxAbs;
        const isPos = day.pnl >= 0;
        return (
          <div
            key={i}
            title={`${fmtDate(day.timestamp)}: ${fmtUsd(day.pnl)}`}
            className="flex-1 rounded-sm"
            style={{
              height: `${Math.max(4, frac * 100)}%`,
              backgroundColor: isPos ? "var(--desk-profit)" : "var(--desk-loss)",
              opacity: 0.85,
              minWidth: 4,
            }}
          />
        );
      })}
    </div>
  );
}

// ── Monthly PnL grid ──────────────────────────────────────────────────────────

function MonthlyGrid({ monthlyPnl }: { monthlyPnl: ReturnType<typeof computeMonthlyPnl> }) {
  if (monthlyPnl.length === 0) return <p className="text-xs text-[var(--desk-muted)]">No monthly data yet.</p>;
  return (
    <div className="flex flex-wrap gap-2 mt-1">
      {monthlyPnl.map((m) => (
        <div key={m.monthStr} className="flex flex-col items-center px-2 py-1 rounded-md bg-[var(--desk-surface-2)] min-w-[60px]">
          <span className="text-[10px] text-[var(--desk-muted)]">{fmtMonth(m.monthStr)}</span>
          <span className={`text-xs font-semibold ${m.pnl >= 0 ? "text-[var(--desk-profit)]" : "text-[var(--desk-loss)]"}`}>
            {fmtUsd(m.pnl)}
          </span>
          <span className="text-[10px] text-[var(--desk-muted)]">{m.trades}T</span>
        </div>
      ))}
    </div>
  );
}

// ── Main panel ────────────────────────────────────────────────────────────────

interface MockEquityCurvePanelProps {
  trades: readonly MockTrade[];
  startingEquityUsd: number;
}

export function MockEquityCurvePanel({ trades, startingEquityUsd }: MockEquityCurvePanelProps) {
  const equityCurve = useMemo(
    () => buildEquityCurve(trades, startingEquityUsd),
    [trades, startingEquityUsd],
  );
  const dailyPnl = useMemo(() => computeDailyPnl(trades), [trades]);
  const monthlyPnl = useMemo(() => computeMonthlyPnl(trades), [trades]);
  const metrics = useMemo(
    () => computeExtendedMetrics(trades, startingEquityUsd),
    [trades, startingEquityUsd],
  );

  const equityValues = equityCurve.map((p) => p.equity);
  const currentEquity = equityCurve.at(-1)?.equity ?? startingEquityUsd;
  const totalReturn = (currentEquity - startingEquityUsd) / startingEquityUsd;
  const currentDd = equityCurve.at(-1)?.drawdownPct ?? 0;
  const equityColor = currentEquity >= startingEquityUsd ? "var(--desk-profit)" : "var(--desk-loss)";

  return (
    <div className="flex flex-col gap-4">
      {/* Header KPIs */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <DeskMetricTile
          label="Equity"
          value={`$${(currentEquity / 1000).toFixed(1)}K`}
          sub={fmtPct(totalReturn)}
          subColor={totalReturn >= 0 ? "profit" : "loss"}
        />
        <DeskMetricTile
          label="Max Drawdown"
          value={fmtPct(metrics.maxDrawdownPct)}
          sub={`${fmtUsd(metrics.maxDrawdownUsd)} peak-to-trough`}
          subColor={metrics.maxDrawdownPct > 0.15 ? "loss" : "neutral"}
        />
        <DeskMetricTile
          label="Recovery Factor"
          value={metrics.recoveryFactor != null ? metrics.recoveryFactor.toFixed(2) : "—"}
          sub="Net PnL / Max DD"
          subColor="neutral"
        />
        <DeskMetricTile
          label="Current DD"
          value={fmtPct(currentDd)}
          sub="From equity peak"
          subColor={currentDd > 0.05 ? "loss" : "neutral"}
        />
      </div>

      {/* Equity sparkline */}
      <DeskCard>
        <DeskSectionHeader title="Equity Curve" />
        {equityValues.length >= 2 ? (
          <div className="mt-2 px-1">
            <Sparkline values={equityValues} color={equityColor} height={80} />
            <div className="flex justify-between mt-1 text-[10px] text-[var(--desk-muted)]">
              <span>{fmtDate(equityCurve[0]?.timestamp ?? 0)}</span>
              <span className="font-medium" style={{ color: equityColor }}>
                {fmtUsd(currentEquity - startingEquityUsd)}
              </span>
              <span>{fmtDate(equityCurve.at(-1)?.timestamp ?? 0)}</span>
            </div>
          </div>
        ) : (
          <p className="text-xs text-[var(--desk-muted)] py-4 text-center">
            At least 2 closed trades required to draw equity curve.
          </p>
        )}
      </DeskCard>

      {/* Daily PnL */}
      <DeskCard>
        <DeskSectionHeader title="Daily PnL (last 30 days)" />
        <div className="mt-2 px-1">
          <DailyPnlBars dailyPnl={dailyPnl} />
          <div className="flex flex-wrap gap-2 mt-2">
            {dailyPnl.slice(-5).reverse().map((d) => (
              <span
                key={d.dateStr}
                className={`text-[10px] font-medium ${d.pnl >= 0 ? "text-[var(--desk-profit)]" : "text-[var(--desk-loss)]"}`}
              >
                {fmtDate(d.timestamp)}: {fmtUsd(d.pnl)}
              </span>
            ))}
          </div>
        </div>
      </DeskCard>

      {/* Monthly PnL */}
      <DeskCard>
        <DeskSectionHeader title="Monthly PnL" />
        <div className="mt-2">
          <MonthlyGrid monthlyPnl={monthlyPnl} />
          {monthlyPnl.length > 0 && (
            <div className="flex gap-4 mt-3 text-xs text-[var(--desk-muted)]">
              <span>
                Best:{" "}
                <strong className="text-[var(--desk-profit)]">
                  {fmtUsd(Math.max(...monthlyPnl.map((m) => m.pnl)))}
                </strong>
              </span>
              <span>
                Worst:{" "}
                <strong className="text-[var(--desk-loss)]">
                  {fmtUsd(Math.min(...monthlyPnl.map((m) => m.pnl)))}
                </strong>
              </span>
              <span>
                Profitable months:{" "}
                <strong>{monthlyPnl.filter((m) => m.pnl > 0).length}/{monthlyPnl.length}</strong>
              </span>
            </div>
          )}
        </div>
      </DeskCard>

      {/* Extended metrics summary */}
      <DeskCard>
        <DeskSectionHeader title="Performance Summary" />
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 mt-2">
          <DeskMetricTile label="Sharpe Ratio" value={metrics.sharpeRatio != null ? metrics.sharpeRatio.toFixed(2) : "—"} sub="Trade-level" />
          <DeskMetricTile label="Sortino Ratio" value={metrics.sortinoRatio != null ? metrics.sortinoRatio.toFixed(2) : "—"} sub="Downside-adjusted" />
          <DeskMetricTile label="Calmar Ratio" value={metrics.calmarRatio != null ? metrics.calmarRatio.toFixed(2) : "—"} sub="Annlzd / Max DD" />
          <DeskMetricTile label="Expectancy" value={fmtUsd(metrics.expectancy)} sub="Per closed trade" subColor={metrics.expectancy >= 0 ? "profit" : "loss"} />
          <DeskMetricTile label="Profit Factor" value={metrics.profitFactor != null ? metrics.profitFactor.toFixed(2) : "—"} sub="Gross wins / losses" />
          <DeskMetricTile label="Avg Hold Time" value={`${Math.round(metrics.averageHoldingTimeMinutes)}m`} sub={`Median ${Math.round(metrics.medianHoldingTimeMinutes)}m`} />
        </div>
      </DeskCard>
    </div>
  );
}
