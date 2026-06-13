"use client";

import { useState } from "react";
import { StrategyRegistryDashboard } from "@/components/strategies/StrategyRegistryDashboard";
import { StrategyDetailDrawer } from "@/components/strategies/StrategyDetailDrawer";

export default function StrategyOpsPage() {
  const [selectedId, setSelectedId] = useState<string | null>(null);

  return (
    <>
      <div className="flex flex-col h-[calc(100vh-96px)] overflow-hidden">
        <div className="px-4 pt-4 pb-2 border-b border-[var(--color-border)]">
          <h1 className="text-[16px] font-semibold text-[var(--color-text-primary)]">Strategy Operations Console</h1>
          <p className="text-[12px] text-[var(--color-text-muted)] mt-0.5">
            600+ strategies — WINNERS_ONLY gate active · Session-gated strategies pause outside 09:15–15:30 IST
          </p>
        </div>
        <div className="flex-1 overflow-hidden flex flex-col">
          <StrategyRegistryDashboard onSelectStrategy={(id) => setSelectedId(id)} />
        </div>
      </div>

      <StrategyDetailDrawer
        strategyId={selectedId}
        onClose={() => setSelectedId(null)}
      />
    </>
  );
}
