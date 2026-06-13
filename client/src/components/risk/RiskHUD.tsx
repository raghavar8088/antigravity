"use client";

import { useEffect, useState } from "react";
import { GaugeBar } from "@/components/ui/GaugeBar";
import { PnlValue } from "@/components/ui/PnlValue";
import { cn } from "@/components/ui/cn";

interface RiskSummary {
  dailyPnl: number;
  dailyPnlLimit: number;
  dailyPnlUsedPct: number;
  drawdownPct: number;
  drawdownLimitPct: number;
  drawdownUsedPct: number;
  exposurePct: number;
  error?: string;
}

/** Compact risk gauges embedded in the header — appears when utilization > 70% */
export function RiskHUD() {
  const [risk, setRisk] = useState<RiskSummary | null>(null);

  useEffect(() => {
    const fetch_ = async () => {
      try {
        const res = await fetch("/api/risk/summary", { cache: "no-store" });
        const data = await res.json();
        if (!data.error) setRisk(data);
      } catch {
        // Silently fail — HUD is supplementary, not critical
      }
    };
    fetch_();
    const id = setInterval(fetch_, 10_000);
    return () => clearInterval(id);
  }, []);

  if (!risk) return null;

  // Only show HUD when any metric exceeds 70%
  const visible =
    risk.dailyPnlUsedPct > 70 ||
    risk.drawdownUsedPct > 70 ||
    risk.exposurePct > 70;

  if (!visible) return null;

  return (
    <div
      className="flex items-center gap-3 px-2 border-l border-[var(--color-border)] ml-1"
      aria-label="Risk utilization HUD"
    >
      {risk.dailyPnlUsedPct > 70 ? (
        <div className="flex flex-col gap-0.5 min-w-[80px]">
          <span className="text-[10px] text-[var(--color-text-muted)]">Daily P&L</span>
          <GaugeBar value={risk.dailyPnlUsedPct} height={4} />
        </div>
      ) : null}

      {risk.drawdownUsedPct > 70 ? (
        <div className="flex flex-col gap-0.5 min-w-[80px]">
          <span className="text-[10px] text-[var(--color-text-muted)]">Drawdown</span>
          <GaugeBar value={risk.drawdownUsedPct} height={4} />
        </div>
      ) : null}

      {risk.exposurePct > 70 ? (
        <div className="flex flex-col gap-0.5 min-w-[80px]">
          <span className="text-[10px] text-[var(--color-text-muted)]">Exposure</span>
          <GaugeBar value={risk.exposurePct} height={4} />
        </div>
      ) : null}

      <PnlValue value={risk.dailyPnl} showSign size="sm" />
    </div>
  );
}
