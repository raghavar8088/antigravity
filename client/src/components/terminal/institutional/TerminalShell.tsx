"use client";

import type { ReactNode } from "react";
import { M3AppShell } from "@/components/ui/M3AppShell";
import { StatusChip } from "@/components/ui/StatusChip";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { terminalAuthorityLabel } from "@/lib/terminal/terminalAuthority";

export function InstitutionalTerminalShell({ children }: { children: ReactNode }) {
  const snapshot = useTerminalSnapshot();
  const criticalAlerts = snapshot.alerts.filter((a) => a.severity === "CRITICAL").length;
  const authorityLabel = terminalAuthorityLabel(snapshot);

  return (
    <M3AppShell
      price={snapshot.hasAuthority ? snapshot.price : undefined}
      priceChange24hPct={snapshot.priceChange24hPct}
      statusChips={
        <>
          <StatusChip
            label={authorityLabel}
            tone={snapshot.hasAuthority ? (snapshot.authoritySource === "ws" ? "success" : "info") : "error"}
          />
          {criticalAlerts > 0 ? <StatusChip label={`${criticalAlerts} critical`} tone="error" /> : null}
        </>
      }
    >
      {children}
    </M3AppShell>
  );
}
