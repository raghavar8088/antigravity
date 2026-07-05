"use client";

import { useEffect, useState } from "react";
import type { TerminalSnapshot } from "@/lib/terminal/terminalTypes";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { DailyPnLTable } from "@/components/DailyPnLTable";
import { pct, pnlClass, usd } from "./format";
import { Metric, TerminalCard } from "./TerminalCard";
import { PageHeader } from "@/components/ui/PageHeader";
import { LineChart } from "@/components/ui/charts/LineChart";

export function AnalyticsCenter({ snapshot }: { snapshot: TerminalSnapshot }) {
  const curve = snapshot.analytics.equityCurve;
  const hasCurve = curve.length > 1;
  const maxR = Math.max(1, ...snapshot.analytics.rMultipleBuckets.map((b) => b.count));

  return (
    <div className="m3-page-stack">
      <PageHeader title="Analytics" subtitle="Equity curve · R-multiple distribution · rolling performance" />
      <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_360px]">
      <div className="space-y-3">
        <TerminalCard title="Equity Curve" subtitle="MongoDB authority · equity curve">
          {!hasCurve ? (
            <TerminalNoData label="No equity curve data" />
          ) : (
            <LineChart
              points={curve.map((p) => ({ ts: new Date(p.time).getTime(), value: p.equity }))}
              color="var(--green)"
              height={240}
              formatValue={(v) => usd(v, { compact: true })}
            />
          )}
        </TerminalCard>
        <TerminalCard title="R-Multiple Distribution">
          {snapshot.analytics.rMultipleBuckets.length === 0 ? (
            <TerminalNoData />
          ) : (
            <div className="grid grid-cols-5 items-end gap-2">
              {snapshot.analytics.rMultipleBuckets.map((bucket) => (
                <div key={bucket.bucket} className="text-center">
                  <div
                    className="mx-auto w-full rounded-t"
                    style={{
                      height: `${Math.max(18, (bucket.count / maxR) * 150)}px`,
                      background: bucket.bucket.trim().startsWith("-") ? "var(--red)" : "var(--green)",
                    }}
                  />
                  <div className="mt-2 text-[10px]" style={{ color: "var(--text-muted)" }}>{bucket.bucket}</div>
                  <div className="font-mono text-xs" style={{ color: "var(--text-primary)" }}>{bucket.count}</div>
                </div>
              ))}
            </div>
          )}
        </TerminalCard>
        <DailyPnLTable />
      </div>
      <div className="space-y-3">
        <TerminalCard title="Rolling Performance" subtitle="Portfolio accounting view">
          <div className="grid grid-cols-2 gap-2">
            <Metric label="Portfolio Sharpe" value={(snapshot.analytics.rollingSharpe30d ?? 0).toFixed(2)} />
            <Metric label="Portfolio PF" value={(snapshot.analytics.profitFactorTrend ?? 0).toFixed(2)} tone="positive" />
            <Metric label="Win Rate" value={snapshot.analytics.winRatePct != null ? pct(snapshot.analytics.winRatePct, 1) : "0.0%"} />
          </div>
        </TerminalCard>
        <TerminalCard title="Fee Drag">
          <Metric label="Fee Drag" value={usd(-(snapshot.analytics.feeDragUsd ?? 0))} tone="negative" />
        </TerminalCard>
        <RegimeBreakdown />
      </div>
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
              <span className={`font-mono ${pnlClass(r.net_pnl)}`}>{usd(r.net_pnl, { signed: true })} · {pct(r.win_rate * 100, 0)} · {r.trades}t</span>
            </div>
          ))}
        </div>
      )}
    </TerminalCard>
  );
}
