"use client";

import { useEffect, useState } from "react";
import type { TerminalSnapshot } from "@/lib/terminal/terminalTypes";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { pct, usd } from "./format";
import { Metric, TerminalCard } from "./TerminalCard";
import { PageHeader } from "@/components/ui/PageHeader";
import { Badge } from "@/components/ui/StatusChip";

type CorrelationMatrix = {
  labels: string[];
  matrix: number[][];
};

export function RiskModule({ snapshot }: { snapshot: TerminalSnapshot }) {
  const [correlation, setCorrelation] = useState<CorrelationMatrix | null>(null);
  const [reconStatus, setReconStatus] = useState<string>("Updating");

  useEffect(() => {
    let active = true;
    Promise.all([
      fetch("/api/paper-trades/correlation-matrix").then((r) => (r.ok ? r.json() : null)),
      fetch("/api/engine/reconciliation").then((r) => (r.ok ? r.json() : null)),
    ]).then(([corr, recon]) => {
      if (!active) return;
      if (corr?.ok) setCorrelation({ labels: corr.labels, matrix: corr.matrix });
      if (recon?.ok) setReconStatus(recon.overall ?? "UNKNOWN");
    }).catch(() => {});
    return () => { active = false; };
  }, []);

  const hasRisk = snapshot.risk.drawdownPct !== 0 || snapshot.risk.grossExposureUsd > 0;
  const heatColor =
    snapshot.risk.heatPct > 70 ? "var(--red)" : snapshot.risk.heatPct > 40 ? "var(--amber)" : "var(--green)";

  return (
    <div className="m3-page-stack">
      <PageHeader title="Risk" subtitle="VaR · exposure · portfolio heat · reconciliation" />
      <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_420px]">
      <div className="space-y-3">
        <div className="grid gap-3 md:grid-cols-4">
          <Metric label="VaR 95" value={usd(-snapshot.risk.var95Usd)} tone="warning" />
          <Metric label="VaR 99" value={usd(-snapshot.risk.var99Usd)} tone="negative" />
          <Metric label="CVaR 95" value={usd(-snapshot.risk.cvar95Usd)} tone="negative" />
          <Metric label="Drawdown" value={`${snapshot.risk.drawdownPct.toFixed(2)}%`} tone={snapshot.risk.drawdownPct < -3 ? "negative" : "neutral"} />
        </div>
        <TerminalCard title="Portfolio Heat" subtitle="0-4 normal · 4-6 reduce · 6+ block">
          {!hasRisk ? (
            <TerminalNoData />
          ) : (
            <>
              <div className="h-5 overflow-hidden rounded-full" style={{ background: "var(--canvas-soft, var(--surface-2))" }}>
                <div
                  className="h-full rounded-full"
                  style={{ width: `${Math.min(100, snapshot.risk.heatPct * 10)}%`, background: heatColor, transition: "width 0.35s ease, background 0.2s ease" }}
                />
              </div>
              <div className="mt-3 grid gap-2 md:grid-cols-4">
                <Metric label="Current Heat" value={`${snapshot.risk.heatPct.toFixed(1)}%`} />
                <Metric label="Margin Usage" value={`${snapshot.risk.marginUsagePct.toFixed(1)}%`} />
                <Metric label="Long Exp" value={usd(snapshot.risk.longExposureUsd, { compact: true })} />
                <Metric label="Short Exp" value={usd(snapshot.risk.shortExposureUsd, { compact: true })} />
              </div>
            </>
          )}
        </TerminalCard>
        <TerminalCard title="Strategy Correlation" subtitle="Pearson · daily PnL · /api/paper-trades/correlation-matrix">
          {correlation && correlation.labels.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[400px] text-center font-mono text-[10px]">
                <thead>
                  <tr>
                    <th className="text-left text-zinc-500" />
                    {correlation.labels.map((l) => (
                      <th key={l} className="truncate px-1 text-zinc-500" title={l}>{l.slice(0, 8)}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {correlation.matrix.map((row, i) => (
                    <tr key={correlation.labels[i] ?? i}>
                      <td className="truncate text-left text-zinc-500">{correlation.labels[i]?.slice(0, 8)}</td>
                      {row.map((v, j) => {
                        const tone = v > 0.7 ? "red" : v > 0.45 ? "amber" : "green";
                        return (
                          <td
                            key={`${i}-${j}`}
                            className="rounded px-1 py-2"
                            style={{
                              background: `var(--${tone}-dim)`,
                              color: `var(--${tone})`,
                            }}
                          >
                            {v.toFixed(2)}
                          </td>
                        );
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <TerminalNoData label="No correlation data" />
          )}
        </TerminalCard>
        <ReconciliationPanel />
      </div>
      <div className="space-y-3">
        <TerminalCard title="Reconciliation Status">
          <Metric label="Overall" value={reconStatus} tone={reconStatus === "GREEN" ? "positive" : reconStatus === "RED" ? "negative" : "warning"} />
        </TerminalCard>
        <TerminalCard title="Open Position Risk">
          {snapshot.positions.length === 0 ? (
            <TerminalNoData label="No open positions" />
          ) : (
            <div className="mt-3 space-y-2">
              {snapshot.positions.map((position) => (
                <div key={position.id} className="flex justify-between rounded-lg bg-zinc-950/40 px-3 py-2 text-xs">
                  <span className="text-zinc-300">{position.strategy}</span>
                  <span className="font-mono text-amber-300">{pct(position.fundingRate * 100, 4)} · {usd(position.fundingPnl, { signed: true })}</span>
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

function ReconciliationPanel() {
  const [events, setEvents] = useState<Array<{ ts: string; drift_amount: number; action: string; kill_switch_triggered: boolean }>>([]);

  useEffect(() => {
    fetch("/api/engine/reconciliation")
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (data?.ok && Array.isArray(data.events)) setEvents(data.events.slice(0, 10));
      })
      .catch(() => {});
  }, []);

  return (
    <TerminalCard title="Reconciliation History" subtitle="Last 24h · /api/engine/reconciliation">
      {events.length === 0 ? (
        <TerminalNoData label="No reconciliation events" />
      ) : (
        <div className="space-y-2 text-xs">
          {events.map((e, i) => (
            <div key={`${e.ts}-${i}`} className="flex items-center justify-between rounded-lg border border-zinc-800 bg-zinc-950/40 px-3 py-2">
              <span className="text-zinc-400">{safeDateTime(e.ts)}</span>
              <span className="font-mono text-zinc-200">{e.action}</span>
              <span className="flex items-center gap-2 font-mono">
                drift {e.drift_amount.toFixed(4)}
                {e.kill_switch_triggered ? <Badge variant="loss" size="sm">KS</Badge> : null}
              </span>
            </div>
          ))}
        </div>
      )}
    </TerminalCard>
  );
}

function safeDateTime(value: string) {
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? new Date(parsed).toLocaleString() : "Timestamp unavailable";
}
