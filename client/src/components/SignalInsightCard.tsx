"use client";

import type { MarketSignal } from "@/lib/marketSignal";

export default function SignalInsightCard({ signal }: { signal: MarketSignal | null }) {
  if (!signal) {
    return (
      <div className="glass-panel p-5">
        <div className="text-xs font-medium uppercase tracking-[0.16em]" style={{ color: "var(--text-secondary)" }}>
          Live signal insight
        </div>
        <div className="mt-4 text-sm" style={{ color: "var(--text-secondary)" }}>
          Waiting for enough live candle context to score the market.
        </div>
      </div>
    );
  }

  const sideClasses = signal.side === "BUY"
    ? "text-emerald-300"
    : signal.side === "SELL"
      ? "text-rose-300"
      : "text-zinc-300";

  const confidenceClasses = signal.confidence >= 75
    ? "bg-emerald-50 text-emerald-700 border-emerald-200"
    : signal.confidence >= 60
      ? "bg-amber-50 text-amber-700 border-amber-200"
      : "bg-slate-100 text-slate-700 border-slate-200";

  return (
    <div className="glass-panel p-5">
      <div className="flex flex-wrap items-center gap-3">
        <div className="text-xs font-medium uppercase tracking-[0.16em]" style={{ color: "var(--text-secondary)" }}>
          Live signal insight
        </div>
        <div className={`text-lg font-medium ${sideClasses}`}>
          {signal.side === "BUY" ? "BUY" : signal.side === "SELL" ? "SELL" : "NEUTRAL"} {signal.tag}
        </div>
        <div className={`rounded-full border px-3 py-1 text-xs font-medium ${confidenceClasses}`}>
          {signal.confidence}% confidence
        </div>
        <div className="text-xs font-mono" style={{ color: "var(--text-secondary)" }}>
          B:{signal.scoreBuy} S:{signal.scoreSell}
        </div>
      </div>

      {signal.strategies.length > 0 && (
        <div className="mt-4 flex flex-wrap gap-2">
          {signal.strategies.map((strategy) => (
            <span
              key={`${signal.tag}-${strategy}`}
              className="rounded-full border border-blue-200 bg-blue-50 px-3 py-1 text-xs font-medium text-blue-700"
            >
              {strategy}
            </span>
          ))}
        </div>
      )}

      {signal.reasons.length > 0 && (
        <div className="mt-4 flex flex-wrap gap-2">
          {signal.reasons.slice(0, 8).map((reason) => (
            <span
              key={`${signal.tag}-${reason}`}
              className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs text-slate-700"
            >
              {reason}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
