"use client";

import { buildDailyPnlRows, type ClosedTradeLike } from "@/lib/portfolio/dailyPnl";

type CurrencyFormatOptions = {
  signed?: boolean;
  decimals?: number;
};

type CurrencyFormatter = (value: number, opts?: CurrencyFormatOptions) => string;

type DailyPnlLedgerProps<T extends ClosedTradeLike> = {
  trades: T[];
  initialEquity: number;
  title: string;
  description: string;
  emptyMessage: string;
  formatCurrency: CurrencyFormatter;
  className?: string;
};

function formatSignedPercent(value: number) {
  return `${value >= 0 ? "+" : "-"}${Math.abs(value).toFixed(2)}%`;
}

export default function DailyPnlLedger<T extends ClosedTradeLike>({
  trades,
  initialEquity,
  title,
  description,
  emptyMessage,
  formatCurrency,
  className = "",
}: DailyPnlLedgerProps<T>) {
  const rows = buildDailyPnlRows(trades, initialEquity);
  const bestDay = rows.reduce<typeof rows[number] | null>((best, row) => (!best || row.pnl > best.pnl ? row : best), null);
  const worstDay = rows.reduce<typeof rows[number] | null>((worst, row) => (!worst || row.pnl < worst.pnl ? row : worst), null);

  return (
    <div className={`glass-panel px-5 py-6 md:px-6 ${className}`.trim()}>
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
            {title}
          </h2>
          <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>
            {description}
          </div>
        </div>
        <div className="flex flex-wrap gap-2 text-[11px]">
          <span className="rounded-full border border-zinc-200 bg-white px-3 py-1 font-medium text-zinc-600">
            {rows.length} trading day{rows.length === 1 ? "" : "s"}
          </span>
          <span
            className={`rounded-full px-3 py-1 font-medium ${bestDay && bestDay.pnl >= 0 ? "text-emerald-700" : "text-zinc-600"}`}
            style={{ background: bestDay && bestDay.pnl >= 0 ? "rgba(24,128,56,0.10)" : "var(--surface-2)" }}
          >
            Best day {bestDay ? formatCurrency(bestDay.pnl, { signed: true }) : "-"}
          </span>
          <span
            className={`rounded-full px-3 py-1 font-medium ${worstDay && worstDay.pnl < 0 ? "text-rose-700" : "text-zinc-600"}`}
            style={{ background: worstDay && worstDay.pnl < 0 ? "rgba(217,48,37,0.10)" : "var(--surface-2)" }}
          >
            Worst day {worstDay ? formatCurrency(worstDay.pnl, { signed: true }) : "-"}
          </span>
        </div>
      </div>

      {rows.length === 0 ? (
        <div
          className="flex min-h-[160px] items-center justify-center rounded-[20px] border border-dashed px-6 py-12 text-center text-sm"
          style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}
        >
          {emptyMessage}
        </div>
      ) : (
        <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
          <table className="w-full text-left text-sm" style={{ minWidth: 900 }}>
            <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
              <tr className="text-[11px] uppercase tracking-[0.12em]">
                <th className="px-4 py-3 font-medium">Date</th>
                <th className="px-4 py-3 font-medium">Trades</th>
                <th className="px-4 py-3 font-medium">W / L</th>
                <th className="px-4 py-3 font-medium text-right">Start Equity</th>
                <th className="px-4 py-3 font-medium text-right">Daily PnL</th>
                <th className="px-4 py-3 font-medium text-right">Daily PnL %</th>
                <th className="px-4 py-3 font-medium text-right">End Equity</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.dateKey} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                  <td className="px-4 py-3">
                    <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{row.label}</div>
                    <div className="font-mono text-[11px]" style={{ color: "var(--text-secondary)" }}>{row.dateKey}</div>
                  </td>
                  <td className="px-4 py-3 font-mono text-sm" style={{ color: "var(--text-primary)" }}>{row.trades}</td>
                  <td className="px-4 py-3 text-sm" style={{ color: "var(--text-secondary)" }}>
                    {row.wins}W / {row.losses}L
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-sm" style={{ color: "var(--text-secondary)" }}>
                    {formatCurrency(row.startEquity)}
                  </td>
                  <td className={`px-4 py-3 text-right font-mono text-sm font-bold ${row.pnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                    {formatCurrency(row.pnl, { signed: true })}
                  </td>
                  <td className={`px-4 py-3 text-right font-mono text-sm font-semibold ${row.pnlPct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                    {formatSignedPercent(row.pnlPct)}
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-sm" style={{ color: "var(--text-primary)" }}>
                    {formatCurrency(row.endEquity)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
