"use client";

import type React from "react";
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

  const sideColor = signal.side === "BUY"
    ? "var(--green)"
    : signal.side === "SELL"
      ? "var(--red)"
      : "var(--text-secondary)";

  const confidenceStyle: React.CSSProperties = signal.confidence >= 75
    ? { background: "var(--green-dim)", color: "var(--green)", borderColor: "rgba(24,128,56,0.2)" }
    : signal.confidence >= 60
      ? { background: "var(--amber-dim)", color: "var(--amber)", borderColor: "rgba(176,96,0,0.2)" }
      : { background: "var(--surface-3)", color: "var(--text-secondary)", borderColor: "var(--border)" };

  return (
    <div className="glass-panel p-5">
      <div className="flex flex-wrap items-center gap-3">
        <div className="text-xs font-medium uppercase tracking-[0.16em]" style={{ color: "var(--text-secondary)" }}>
          Live signal insight
        </div>
        <div className="text-lg font-medium" style={{ color: sideColor }}>
          {signal.side === "BUY" ? "BUY" : signal.side === "SELL" ? "SELL" : "NEUTRAL"} {signal.tag}
        </div>
        <div className="rounded-full border px-3 py-1 text-xs font-medium" style={confidenceStyle}>
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
              className="rounded-full border px-3 py-1 text-xs font-medium"
              style={{ background: "var(--accent-dim)", color: "var(--accent)", borderColor: "rgba(26,115,232,0.2)" }}
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
              className="rounded-full border px-3 py-1 text-xs"
              style={{ background: "var(--surface-3)", color: "var(--text-secondary)", borderColor: "var(--border)" }}
            >
              {reason}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
