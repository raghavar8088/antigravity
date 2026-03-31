"use client";

import type { MarketSignal } from "@/lib/marketSignal";

export default function SignalInsightCard({ signal }: { signal: MarketSignal | null }) {
  if (!signal) {
    return (
      <div className="rounded-2xl border border-zinc-800/80 bg-zinc-950/70 p-5">
        <div className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
          Live Signal Insight
        </div>
        <div className="mt-4 text-sm text-zinc-500">
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
    ? "bg-emerald-500/15 text-emerald-300 border-emerald-500/30"
    : signal.confidence >= 60
      ? "bg-amber-500/15 text-amber-300 border-amber-500/30"
      : "bg-zinc-500/15 text-zinc-300 border-zinc-500/30";

  return (
    <div className="rounded-2xl border border-zinc-800/80 bg-zinc-950/70 p-5">
      <div className="flex flex-wrap items-center gap-3">
        <div className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
          Live Signal Insight
        </div>
        <div className={`text-lg font-semibold ${sideClasses}`}>
          {signal.side === "BUY" ? "BUY" : signal.side === "SELL" ? "SELL" : "NEUTRAL"} {signal.tag}
        </div>
        <div className={`rounded-full border px-3 py-1 text-xs font-semibold ${confidenceClasses}`}>
          {signal.confidence}% confidence
        </div>
        <div className="text-xs font-mono text-zinc-500">
          B:{signal.scoreBuy} S:{signal.scoreSell}
        </div>
      </div>

      {signal.strategies.length > 0 && (
        <div className="mt-4 flex flex-wrap gap-2">
          {signal.strategies.map((strategy) => (
            <span
              key={`${signal.tag}-${strategy}`}
              className="rounded-full border border-sky-500/20 bg-sky-500/10 px-3 py-1 text-xs font-medium text-sky-300"
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
              className="rounded-full border border-zinc-800 bg-zinc-900/80 px-3 py-1 text-xs text-zinc-300"
            >
              {reason}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
