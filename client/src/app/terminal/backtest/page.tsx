"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { PnlValue } from "@/components/ui/PnlValue";
import { GaugeBar } from "@/components/ui/GaugeBar";
import { Badge } from "@/components/ui/Badge";
import { Sparkline } from "@/components/ui/Sparkline";
import { cn } from "@/components/ui/cn";

interface BacktestConfig {
  strategyId: string;
  from: string;
  to: string;
  capitalUsdt: number;
  slippageModel: "none" | "fixed_bps" | "realistic";
  slippageBps: number;
  commission: "none" | "fixed" | "percentage";
  commissionValue: number;
  exchange: string;
}

interface BacktestResult {
  totalReturnPct: number;
  annualizedReturnPct: number;
  sharpeRatio: number;
  sortinoRatio: number;
  maxDrawdownPct: number;
  maxDrawdownDurationDays: number;
  winRatePct: number;
  profitFactor: number;
  avgTradeDurationMs: number;
  totalTrades: number;
  equityCurve: Array<{ date: string; value: number }>;
  monthlyReturns: Array<{ year: number; month: number; returnPct: number }>;
  trades: Array<{
    tradeId: string; entryAt: string; exitAt: string; symbol: string;
    direction: "LONG" | "SHORT"; entryPrice: number; exitPrice: number;
    quantity: number; pnl: number; returnPct: number; exitReason: string;
  }>;
}

interface JobStatus {
  jobId: string;
  status: "queued" | "running" | "complete" | "failed";
  progressPct: number;
  barsProcessed: number;
  totalBars: number;
  error: string | null;
  result?: BacktestResult;
}

const PRESETS = [
  { label: "1M",  from: () => { const d = new Date(); d.setMonth(d.getMonth()-1); return d.toISOString().split("T")[0]; } },
  { label: "3M",  from: () => { const d = new Date(); d.setMonth(d.getMonth()-3); return d.toISOString().split("T")[0]; } },
  { label: "6M",  from: () => { const d = new Date(); d.setMonth(d.getMonth()-6); return d.toISOString().split("T")[0]; } },
  { label: "1Y",  from: () => { const d = new Date(); d.setFullYear(d.getFullYear()-1); return d.toISOString().split("T")[0]; } },
];

const today = new Date().toISOString().split("T")[0];

function MetricCard({ label, value, positive }: { label: string; value: string; positive?: boolean }) {
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-3">
      <div className="text-[10px] text-[var(--color-text-muted)] uppercase tracking-wide mb-1">{label}</div>
      <div className={cn("text-[18px] font-mono tnum font-semibold",
        positive === true ? "text-[var(--color-profit)]" : positive === false ? "text-[var(--color-loss)]" : "text-[var(--color-text-primary)]",
      )}>{value}</div>
    </div>
  );
}

function monthReturnColor(pct: number): string {
  if (pct > 5)  return "bg-[rgba(34,197,94,0.6)]";
  if (pct > 0)  return "bg-[rgba(34,197,94,0.25)]";
  if (pct === 0) return "bg-[var(--color-bg-elevated)]";
  if (pct > -5) return "bg-[rgba(239,68,68,0.25)]";
  return "bg-[rgba(239,68,68,0.6)]";
}

export default function BacktestPage() {
  const [config, setConfig] = useState<BacktestConfig>({
    strategyId: "", from: PRESETS[1].from(), to: today,
    capitalUsdt: 10000, slippageModel: "realistic",
    slippageBps: 5, commission: "percentage", commissionValue: 0.05,
    exchange: "BINANCE",
  });
  const [jobId, setJobId] = useState<string | null>(null);
  const [job, setJob] = useState<JobStatus | null>(null);
  const [running, setRunning] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  function set<K extends keyof BacktestConfig>(k: K, v: BacktestConfig[K]) {
    setConfig((c) => ({ ...c, [k]: v }));
  }

  const pollStatus = useCallback(async (id: string) => {
    try {
      const res = await fetch(`/api/backtest/status/${id}`, { cache: "no-store" });
      const data: JobStatus = await res.json();
      setJob(data);
      if (data.status === "complete" || data.status === "failed") {
        setRunning(false);
        if (pollRef.current) clearInterval(pollRef.current);
      }
    } catch { /* ignore */ }
  }, []);

  async function runBacktest() {
    if (!config.strategyId.trim()) return;
    setRunning(true);
    setJob(null);
    try {
      const res = await fetch("/api/backtest/run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(config),
      });
      const data = await res.json();
      if (data.jobId) {
        setJobId(data.jobId);
        pollRef.current = setInterval(() => pollStatus(data.jobId), 2000);
      }
    } catch {
      setRunning(false);
    }
  }

  useEffect(() => () => { if (pollRef.current) clearInterval(pollRef.current); }, []);

  const result = job?.result;
  const equityValues = result?.equityCurve.map((p) => p.value) ?? [];

  const MONTHS = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"];
  const years = result ? [...new Set(result.monthlyReturns.map((r) => r.year))].sort() : [];

  return (
    <div className="flex h-[calc(100vh-96px)] gap-0">
      {/* Left sidebar — configurator */}
      <aside className="w-[300px] shrink-0 border-r border-[var(--color-border)] overflow-y-auto p-4 space-y-4">
        <h2 className="text-[13px] font-semibold text-[var(--color-text-primary)]">Backtest Configurator</h2>

        <div>
          <label className="text-[11px] text-[var(--color-text-muted)] block mb-1">Strategy ID</label>
          <input
            type="text"
            value={config.strategyId}
            onChange={(e) => set("strategyId", e.target.value)}
            placeholder="e.g. ema_cross_15"
            className="w-full text-[12px] px-3 py-1.5 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] outline-none focus:border-[var(--color-border-focus)]"
          />
        </div>

        <div>
          <div className="flex items-center justify-between mb-1">
            <label className="text-[11px] text-[var(--color-text-muted)]">Date Range</label>
            <div className="flex gap-1">
              {PRESETS.map((p) => (
                <button key={p.label} type="button" onClick={() => set("from", p.from())}
                  className="text-[10px] px-1.5 py-0.5 rounded bg-[var(--color-bg-elevated)] text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]">
                  {p.label}
                </button>
              ))}
            </div>
          </div>
          <div className="flex gap-2">
            <input type="date" value={config.from} onChange={(e) => set("from", e.target.value)}
              className="flex-1 text-[12px] px-2 py-1.5 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] outline-none" />
            <input type="date" value={config.to} onChange={(e) => set("to", e.target.value)}
              className="flex-1 text-[12px] px-2 py-1.5 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] outline-none" />
          </div>
        </div>

        <div>
          <label className="text-[11px] text-[var(--color-text-muted)] block mb-1">Capital (USDT)</label>
          <input type="number" value={config.capitalUsdt} onChange={(e) => set("capitalUsdt", Number(e.target.value))}
            className="w-full text-[12px] px-3 py-1.5 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] outline-none" />
        </div>

        <div>
          <label className="text-[11px] text-[var(--color-text-muted)] block mb-1">Slippage Model</label>
          <select value={config.slippageModel} onChange={(e) => set("slippageModel", e.target.value as BacktestConfig["slippageModel"])}
            className="w-full text-[12px] px-2 py-1.5 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-[var(--color-text-secondary)] outline-none">
            <option value="none">None</option>
            <option value="fixed_bps">Fixed BPS</option>
            <option value="realistic">Realistic (spread model)</option>
          </select>
        </div>

        <div>
          <label className="text-[11px] text-[var(--color-text-muted)] block mb-1">Exchange</label>
          <select value={config.exchange} onChange={(e) => set("exchange", e.target.value)}
            className="w-full text-[12px] px-2 py-1.5 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-[var(--color-text-secondary)] outline-none">
            {["BINANCE","DELTA","NSE","BSE"].map((ex) => <option key={ex} value={ex}>{ex}</option>)}
          </select>
        </div>

        <button
          type="button"
          onClick={runBacktest}
          disabled={running || !config.strategyId.trim()}
          className={cn(
            "w-full py-2 rounded text-[13px] font-semibold transition-colors",
            running || !config.strategyId.trim()
              ? "bg-[var(--color-bg-elevated)] text-[var(--color-text-muted)] cursor-not-allowed"
              : "bg-[var(--color-info)] text-white hover:bg-[rgba(59,130,246,0.85)]",
          )}
        >
          {running ? "Running…" : "▶ Run Backtest"}
        </button>

        {/* Progress */}
        {running && job && (
          <div>
            <GaugeBar value={job.progressPct} label={`Processing ${job.barsProcessed.toLocaleString()} / ${job.totalBars.toLocaleString()} bars`} />
          </div>
        )}
      </aside>

      {/* Right — results */}
      <main className="flex-1 overflow-y-auto p-4">
        {!result && !running && (
          <div className="flex items-center justify-center h-full text-[13px] text-[var(--color-text-muted)]">
            Configure a strategy above and click Run Backtest
          </div>
        )}

        {running && !result && (
          <div className="flex items-center justify-center h-full flex-col gap-3">
            <div className="text-[13px] text-[var(--color-text-secondary)]">Running backtest…</div>
            {job && <GaugeBar value={job.progressPct} className="w-64" />}
          </div>
        )}

        {result && (
          <div className="space-y-6">
            {/* Equity curve */}
            <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-4">
              <div className="text-[11px] font-medium text-[var(--color-text-secondary)] mb-3 uppercase tracking-wide">Equity Curve</div>
              <Sparkline data={equityValues} width={Math.min(900, equityValues.length * 3)} height={120} strokeWidth={2} />
            </div>

            {/* Performance metrics */}
            <div className="grid grid-cols-5 gap-2">
              <MetricCard label="Total Return" value={`${result.totalReturnPct >= 0 ? "+" : ""}${result.totalReturnPct.toFixed(2)}%`} positive={result.totalReturnPct >= 0} />
              <MetricCard label="Ann. Return" value={`${result.annualizedReturnPct >= 0 ? "+" : ""}${result.annualizedReturnPct.toFixed(2)}%`} positive={result.annualizedReturnPct >= 0} />
              <MetricCard label="Sharpe" value={result.sharpeRatio.toFixed(2)} positive={result.sharpeRatio > 1} />
              <MetricCard label="Max DD" value={`-${result.maxDrawdownPct.toFixed(2)}%`} positive={false} />
              <MetricCard label="Win Rate" value={`${result.winRatePct.toFixed(1)}%`} positive={result.winRatePct >= 50} />
              <MetricCard label="Sortino" value={result.sortinoRatio.toFixed(2)} positive={result.sortinoRatio > 1.5} />
              <MetricCard label="Profit Factor" value={result.profitFactor.toFixed(2)} positive={result.profitFactor > 1.5} />
              <MetricCard label="Max DD Days" value={`${result.maxDrawdownDurationDays}d`} />
              <MetricCard label="Avg Duration" value={(() => { const m = Math.floor(result.avgTradeDurationMs / 60000); return m < 60 ? `${m}m` : `${Math.floor(m/60)}h`; })()} />
              <MetricCard label="Total Trades" value={result.totalTrades.toLocaleString()} />
            </div>

            {/* Monthly returns heatmap */}
            {years.length > 0 && (
              <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-4">
                <div className="text-[11px] font-medium text-[var(--color-text-secondary)] mb-3 uppercase tracking-wide">Monthly Returns</div>
                <table role="table" className="text-[10px]">
                  <thead>
                    <tr>
                      <th className="pr-2 text-left text-[var(--color-text-muted)] font-medium">Year</th>
                      {MONTHS.map((m) => <th key={m} className="px-1 text-center text-[var(--color-text-muted)] font-medium w-10">{m}</th>)}
                    </tr>
                  </thead>
                  <tbody>
                    {years.map((yr) => (
                      <tr key={yr}>
                        <td className="pr-2 font-mono tnum text-[var(--color-text-secondary)]">{yr}</td>
                        {Array.from({ length: 12 }, (_, mo) => {
                          const entry = result.monthlyReturns.find((r) => r.year === yr && r.month === mo + 1);
                          return (
                            <td key={mo} className="px-0.5 py-0.5">
                              {entry ? (
                                <div className={cn("rounded text-center w-10 py-0.5 font-mono tnum", monthReturnColor(entry.returnPct))}>
                                  {entry.returnPct >= 0 ? "+" : ""}{entry.returnPct.toFixed(1)}%
                                </div>
                              ) : (
                                <div className="w-10 py-0.5 text-center text-[var(--color-text-muted)]">—</div>
                              )}
                            </td>
                          );
                        })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {/* Trade log */}
            <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)]">
              <div className="px-4 py-3 border-b border-[var(--color-border)]">
                <span className="text-[11px] font-medium text-[var(--color-text-secondary)] uppercase tracking-wide">
                  Trade Log ({result.trades.length.toLocaleString()} trades)
                </span>
              </div>
              <div className="overflow-auto max-h-80">
                <table className="w-full" role="table">
                  <thead className="sticky top-0 bg-[var(--color-bg-surface)]">
                    <tr className="text-[10px] text-[var(--color-text-muted)]">
                      {["Entry","Exit","Symbol","Dir","Entry Px","Exit Px","Qty","P&L","Return","Exit Reason"].map((h) => (
                        <th key={h} className="px-2 py-1.5 text-left font-medium whitespace-nowrap">{h}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {result.trades.slice(0, 500).map((t) => (
                      <tr key={t.tradeId} className="border-b border-[var(--color-border)] hover:bg-[var(--color-bg-elevated)] text-[10px]">
                        <td className="px-2 py-1 font-mono text-[var(--color-text-muted)]">
                          {new Date(t.entryAt).toLocaleDateString("en-IN", { month: "short", day: "numeric" })}
                        </td>
                        <td className="px-2 py-1 font-mono text-[var(--color-text-muted)]">
                          {new Date(t.exitAt).toLocaleDateString("en-IN", { month: "short", day: "numeric" })}
                        </td>
                        <td className="px-2 py-1 font-mono text-[var(--color-text-primary)]">{t.symbol}</td>
                        <td className="px-2 py-1">
                          <Badge variant={t.direction === "LONG" ? "profit" : "loss"} size="sm">{t.direction}</Badge>
                        </td>
                        <td className="px-2 py-1 font-mono tnum text-right text-[var(--color-text-secondary)]">
                          {t.entryPrice.toLocaleString("en-IN", { maximumFractionDigits: 2 })}
                        </td>
                        <td className="px-2 py-1 font-mono tnum text-right text-[var(--color-text-secondary)]">
                          {t.exitPrice.toLocaleString("en-IN", { maximumFractionDigits: 2 })}
                        </td>
                        <td className="px-2 py-1 font-mono tnum text-right text-[var(--color-text-secondary)]">{t.quantity}</td>
                        <td className="px-2 py-1 text-right"><PnlValue value={t.pnl} showSign size="xs" /></td>
                        <td className="px-2 py-1 text-right"><PnlValue value={t.returnPct} showSign size="xs" format={(v) => `${Math.abs(v).toFixed(2)}%`} /></td>
                        <td className="px-2 py-1 text-[var(--color-text-muted)]">{t.exitReason}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
