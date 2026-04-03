"use client";

import { formatUSD } from "@/lib/money";
import { formatShortDate, formatShortTime } from "@/lib/time";

interface RunningTrade {
  id: string;
  strategy: string;
  side: string;
  size: number;
  entry: number;
  stopLoss: number;
  takeProfit: number;
  originalSize: number;
  trailingActive: boolean;
  partialClosed: boolean;
  openTime: string;
  elapsed: string;
}

export default function RunningTrades({ currentPrice, trades }: { currentPrice: number; trades: RunningTrade[] }) {
  if (trades.length === 0) {
    return <div className="py-12 text-center text-sm" style={{ color: "var(--text-secondary)" }}>No active trades yet.</div>;
  }

  const totalUnrealized = trades.reduce((sum, t) => {
    const mark = currentPrice > 0 ? currentPrice : t.entry;
    return sum + (t.side === "LONG" ? (mark - t.entry) * t.size : (t.entry - mark) * t.size);
  }, 0);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className="h-2 w-2 rounded-full" style={{ background: "var(--green)" }} />
          <span className="text-xs font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>
            {trades.length} running positions
          </span>
        </div>
        <span
          className="rounded-full border px-3 py-1 text-xs font-medium"
          style={{
            background: totalUnrealized >= 0 ? "var(--green-dim)" : "var(--red-dim)",
            color: totalUnrealized >= 0 ? "var(--green)" : "var(--red)",
            borderColor: totalUnrealized >= 0 ? "rgba(24, 128, 56, 0.14)" : "rgba(217, 48, 37, 0.14)",
          }}
        >
          Unrealized {formatUSD(totalUnrealized, { signed: true })}
        </span>
      </div>

      <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
        <table className="w-full text-left text-sm">
          <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
            <tr className="text-[11px] uppercase tracking-[0.12em]">
              <th className="px-4 py-3 font-medium">Position</th>
              <th className="px-4 py-3 font-medium">Entry</th>
              <th className="px-4 py-3 font-medium">Size</th>
              <th className="px-4 py-3 font-medium">Opened</th>
              <th className="px-4 py-3 font-medium">PnL</th>
              <th className="px-4 py-3 font-medium text-right">Progress</th>
            </tr>
          </thead>
          <tbody>
            {trades.map((t) => {
              const markPrice = currentPrice > 0 ? currentPrice : t.entry;
              const pnl = t.side === "LONG" ? (markPrice - t.entry) * t.size : (t.entry - markPrice) * t.size;
              const totalRange = t.takeProfit - t.stopLoss;
              const progress = totalRange !== 0 ? ((markPrice - t.stopLoss) / totalRange) * 100 : 50;
              const clamped = Math.max(0, Math.min(100, progress));

              return (
                <tr key={t.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                  <td className="px-4 py-3">
                    <div className="flex flex-col gap-1">
                      <div className="flex items-center gap-2">
                        <span
                          className="rounded-full px-2 py-0.5 text-[10px] font-medium uppercase"
                          style={{
                            background: t.side === "LONG" ? "var(--green-dim)" : "var(--red-dim)",
                            color: t.side === "LONG" ? "var(--green)" : "var(--red)",
                          }}
                        >
                          {t.side}
                        </span>
                        <span className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>{t.strategy}</span>
                      </div>
                      <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                        {t.trailingActive ? "Trail active" : "Static risk"} {t.partialClosed ? "• TP1 done" : ""}
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>${t.entry.toFixed(2)}</td>
                  <td className="px-4 py-3">
                    <div className="font-mono text-xs" style={{ color: "var(--text-primary)" }}>{t.size.toFixed(4)} BTC</div>
                    {t.originalSize > t.size && <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>from {t.originalSize.toFixed(4)}</div>}
                  </td>
                  <td className="px-4 py-3 text-xs">
                    {t.openTime && t.openTime !== "-" ? (
                      <div>
                        <div className="font-mono" style={{ color: "var(--text-primary)" }}>{formatShortTime(t.openTime)}</div>
                        <div style={{ color: "var(--text-secondary)" }}>{formatShortDate(t.openTime)} • {t.elapsed}</div>
                      </div>
                    ) : (
                      <span style={{ color: "var(--text-secondary)" }}>-</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="font-mono text-sm font-semibold" style={{ color: pnl >= 0 ? "var(--green)" : "var(--red)" }}>
                      {formatUSD(pnl, { signed: true })}
                    </div>
                    <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                      SL ${t.stopLoss.toFixed(2)} • TP ${t.takeProfit.toFixed(2)}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="ml-auto w-28">
                      <div className="h-2 overflow-hidden rounded-full" style={{ background: "var(--surface-3)" }}>
                        <div
                          className="h-full rounded-full"
                          style={{
                            width: `${clamped}%`,
                            background: pnl >= 0 ? "var(--green)" : "var(--red)",
                            transition: "width 0.3s ease",
                          }}
                        />
                      </div>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
