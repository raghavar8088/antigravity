"use client";

import { useEffect, useState } from "react";
import { PnlValue } from "@/components/ui/PnlValue";
import { Badge } from "@/components/ui/Badge";
import { Sparkline } from "@/components/ui/Sparkline";
import { cn } from "@/components/ui/cn";
import type { StrategyDetail } from "@/types/strategy";

function StatRow({ label, value, className }: { label: string; value: string | number; className?: string }) {
  return (
    <div className="flex items-center justify-between py-1 border-b border-[var(--color-border)]">
      <span className="text-[11px] text-[var(--color-text-muted)]">{label}</span>
      <span className={cn("text-[11px] font-mono tnum text-[var(--color-text-primary)]", className)}>{value}</span>
    </div>
  );
}

function formatDuration(ms: number): string {
  const h = Math.floor(ms / 3_600_000);
  const m = Math.floor((ms % 3_600_000) / 60_000);
  return h > 0 ? `${h}h ${m}m` : `${m}m`;
}

export function StrategyDetailDrawer({ strategyId, onClose }: { strategyId: string | null; onClose: () => void }) {
  const [detail, setDetail] = useState<StrategyDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [equityTab, setEquityTab] = useState<"7d" | "30d">("7d");

  useEffect(() => {
    if (!strategyId) return;
    setLoading(true);
    setDetail(null);
    fetch(`/api/strategies/${strategyId}`)
      .then((r) => r.json())
      .then((d) => { if (!d.error) setDetail(d); })
      .catch(() => { /* ignore */ })
      .finally(() => setLoading(false));
  }, [strategyId]);

  if (!strategyId) return null;

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 z-40 bg-black/40 backdrop-blur-[2px]"
        onClick={onClose}
        aria-label="Close drawer"
      />

      {/* Drawer */}
      <aside
        className="fixed right-0 top-0 bottom-0 z-50 w-[480px] bg-[var(--color-bg-surface)] border-l border-[var(--color-border)] shadow-[var(--shadow-elevated)] overflow-y-auto"
        aria-label="Strategy detail"
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)] sticky top-0 bg-[var(--color-bg-surface)] z-10">
          <h2 className="text-[14px] font-semibold text-[var(--color-text-primary)]">
            {loading ? "Loading…" : (detail?.name ?? strategyId)}
          </h2>
          <button type="button" onClick={onClose} className="text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] text-lg" aria-label="Close">
            ✕
          </button>
        </div>

        {loading ? (
          <div className="p-4 space-y-3">
            {Array.from({ length: 6 }).map((_, i) => <div key={i} className="skeleton h-8 rounded" />)}
          </div>
        ) : !detail ? (
          <div className="p-4 text-[12px] text-[var(--color-warning)]">
            Strategy data unavailable — engine may be offline.
          </div>
        ) : (
          <div className="p-4 space-y-5">
            {/* Status chips */}
            <div className="flex items-center gap-2 flex-wrap">
              <Badge variant={detail.status === "ACTIVE" ? "profit" : "caution"} size="md">{detail.status}</Badge>
              <Badge variant={detail.alphaTier === "A" ? "profit" : "neutral"} size="md">Tier {detail.alphaTier}</Badge>
              <Badge variant={detail.exchange === "NSE" || detail.exchange === "BSE" ? "caution" : "info"} size="md">
                {detail.exchange}
              </Badge>
              <Badge variant={detail.sessionType === "SESSION_GATED" ? "caution" : "info"} size="md">
                {detail.sessionType === "SESSION_GATED" ? "9:15–15:30 IST" : "24/7"}
              </Badge>
            </div>

            {/* Equity Curve */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <span className="text-[11px] font-medium text-[var(--color-text-secondary)]">Equity Curve</span>
                <div className="flex gap-1">
                  {(["7d","30d"] as const).map((t) => (
                    <button
                      key={t}
                      type="button"
                      onClick={() => setEquityTab(t)}
                      className={cn(
                        "text-[10px] px-2 py-0.5 rounded",
                        equityTab === t ? "bg-[var(--color-info)] text-white" : "text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]",
                      )}
                    >
                      {t}
                    </button>
                  ))}
                </div>
              </div>
              <Sparkline
                data={equityTab === "7d" ? detail.equityCurve7d : detail.equityCurve30d}
                width={430}
                height={80}
                strokeWidth={2}
              />
            </div>

            {/* Performance metrics */}
            <div>
              <div className="text-[11px] font-medium text-[var(--color-text-secondary)] mb-2 uppercase tracking-wide">Performance</div>
              <StatRow label="Win Rate" value={`${detail.winRatePct.toFixed(1)}%`} />
              <StatRow label="Sharpe Ratio" value={detail.sharpeRatio.toFixed(2)} />
              <StatRow label="Sortino Ratio" value={detail.sortinoRatio.toFixed(2)} />
              <StatRow label="Calmar Ratio" value={detail.calmarRatio.toFixed(2)} />
              <StatRow label="Max Drawdown" value={`${detail.maxDrawdownPct.toFixed(1)}%`} />
              <StatRow label="Avg Trade Duration" value={formatDuration(detail.avgTradeDurationMs)} />
              <StatRow label="Best Trade" value={`+${detail.bestTradePnl.toFixed(2)}`} className="text-[var(--color-profit)]" />
              <StatRow label="Worst Trade" value={`${detail.worstTradePnl.toFixed(2)}`} className="text-[var(--color-loss)]" />
            </div>

            {/* Risk contribution */}
            <div>
              <div className="text-[11px] font-medium text-[var(--color-text-secondary)] mb-2 uppercase tracking-wide">Risk Contribution</div>
              <StatRow label="Beta to BTC" value={detail.betaToBTC.toFixed(2)} />
              <StatRow label="Beta to Nifty" value={detail.betaToNifty.toFixed(2)} />
              <StatRow label="Correlation Bucket" value={detail.correlationBucket} />
              <StatRow label="Capital Allocated" value={`${detail.capitalCurrency === "INR" ? "₹" : "$"}${detail.capitalAllocated.toLocaleString()}`} />
            </div>

            {/* Recent trades */}
            <div>
              <div className="text-[11px] font-medium text-[var(--color-text-secondary)] mb-2 uppercase tracking-wide">Last 50 Trades</div>
              <div className="overflow-x-auto">
                <table className="w-full" role="table">
                  <thead>
                    <tr className="text-[10px] text-[var(--color-text-muted)]">
                      {["Entry", "Exit", "Dir", "P&L", "Return", "Exit Reason"].map((h) => (
                        <th key={h} className="px-1 py-1 text-left font-medium whitespace-nowrap">{h}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {detail.recentTrades.slice(0, 50).map((t) => (
                      <tr key={t.tradeId} className="border-b border-[var(--color-border)] text-[10px]">
                        <td className="px-1 py-0.5 font-mono text-[var(--color-text-muted)]">
                          {new Date(t.entryAt).toLocaleTimeString("en-IN", { hour: "2-digit", minute: "2-digit" })}
                        </td>
                        <td className="px-1 py-0.5 font-mono text-[var(--color-text-muted)]">
                          {new Date(t.exitAt).toLocaleTimeString("en-IN", { hour: "2-digit", minute: "2-digit" })}
                        </td>
                        <td className="px-1 py-0.5">
                          <Badge variant={t.direction === "LONG" ? "profit" : "loss"} size="sm">{t.direction}</Badge>
                        </td>
                        <td className="px-1 py-0.5">
                          <PnlValue value={t.pnl} showSign size="xs" />
                        </td>
                        <td className="px-1 py-0.5">
                          <PnlValue value={t.returnPct} showSign size="xs" format={(v) => `${Math.abs(v).toFixed(2)}%`} />
                        </td>
                        <td className="px-1 py-0.5 text-[var(--color-text-muted)]">{t.exitReason}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

            {/* Recent signals */}
            <div>
              <div className="text-[11px] font-medium text-[var(--color-text-secondary)] mb-2 uppercase tracking-wide">Last 10 Signals</div>
              {detail.recentSignals.slice(0, 10).map((s) => (
                <div key={s.signalId} className="py-1.5 border-b border-[var(--color-border)]">
                  <div className="flex items-center gap-2 mb-0.5">
                    <span className="font-mono text-[10px] text-[var(--color-text-muted)]">
                      {new Date(s.generatedAt).toLocaleTimeString("en-IN", { hour12: false })}
                    </span>
                    <Badge variant={s.action.includes("LONG") ? "profit" : s.action.includes("SHORT") ? "loss" : "neutral"} size="sm">
                      {s.action}
                    </Badge>
                    <span className="text-[11px] font-mono text-[var(--color-text-primary)]">{s.symbol}</span>
                  </div>
                  <p className="text-[10px] text-[var(--color-text-muted)]">{s.explanation}</p>
                </div>
              ))}
            </div>
          </div>
        )}
      </aside>
    </>
  );
}
