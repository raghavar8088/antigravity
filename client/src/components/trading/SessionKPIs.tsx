"use client";

import { useEffect, useState } from "react";
import { PnlValue } from "@/components/ui/PnlValue";
import { GaugeBar } from "@/components/ui/GaugeBar";
import { useSession } from "@/context/SessionContext";
import type { RiskSummary } from "@/types/trading";

export function SessionKPIs() {
  const [risk, setRisk] = useState<RiskSummary | null>(null);
  const session = useSession();

  useEffect(() => {
    const load = async () => {
      try {
        const res = await fetch("/api/risk/summary", { cache: "no-store" });
        const d = await res.json();
        if (!d.error) setRisk(d);
      } catch { /* ignore */ }
    };
    load();
    const id = setInterval(load, 10_000);
    return () => clearInterval(id);
  }, []);

  return (
    <div className="flex flex-col gap-3 p-3 h-full overflow-y-auto">
      <div className="text-[11px] font-semibold text-[var(--color-text-secondary)] uppercase tracking-wide">
        Session KPIs
      </div>

      {/* Session state */}
      <div className="flex flex-col gap-1">
        <span className="text-[10px] text-[var(--color-text-muted)]">NSE Session</span>
        <span className={`text-[13px] font-semibold ${session.nseOpen ? "text-[var(--color-profit)]" : "text-[var(--color-neutral)]"}`}>
          {session.sessionLabel}
        </span>
        {session.nextEventLabel !== "—" && !session.isHoliday ? (
          <span className="text-[10px] text-[var(--color-caution)] font-mono tnum">{session.nextEventLabel}</span>
        ) : null}
      </div>

      {risk ? (
        <>
          <div>
            <span className="text-[10px] text-[var(--color-text-muted)]">Session P&L</span>
            <PnlValue value={risk.dailyPnl} showSign size="lg" className="block mt-0.5" />
          </div>

          <GaugeBar value={risk.dailyPnlUsedPct} label="Daily limit used" />
          <GaugeBar value={risk.drawdownUsedPct} label="Drawdown" />
          <GaugeBar value={risk.exposurePct} label="Exposure" />

          <div className="grid grid-cols-2 gap-2 mt-1">
            <div>
              <div className="text-[10px] text-[var(--color-text-muted)]">Strategies</div>
              <div className="text-[16px] font-mono tnum font-semibold text-[var(--color-text-primary)]">
                {risk.activeStrategies}
              </div>
            </div>
            <div>
              <div className="text-[10px] text-[var(--color-text-muted)]">Open Orders</div>
              <div className="text-[16px] font-mono tnum font-semibold text-[var(--color-text-primary)]">
                {risk.openOrders}
              </div>
            </div>
            <div>
              <div className="text-[10px] text-[var(--color-text-muted)]">Positions</div>
              <div className="text-[16px] font-mono tnum font-semibold text-[var(--color-text-primary)]">
                {risk.openPositions}
              </div>
            </div>
            <div>
              <div className="text-[10px] text-[var(--color-text-muted)]">Exposure</div>
              <div className="text-[13px] font-mono tnum font-semibold text-[var(--color-text-primary)]">
                ${(risk.exposureUsdt / 1000).toFixed(1)}K
              </div>
            </div>
          </div>
        </>
      ) : (
        <div className="space-y-2 mt-2">
          {Array.from({ length: 5 }).map((_, i) => <div key={i} className="skeleton h-6 rounded" />)}
        </div>
      )}
    </div>
  );
}
