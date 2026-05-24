"use client";

import { DeskChip } from "@/components/desk/ui/DeskChip";

type LiveDeskStatusBarProps = {
  profitModeEnabled: boolean;
  isSignedIn: boolean;
  /** Optional — pass `unifiedReadinessLabel(stats.unifiedReadiness)` when available. */
  readinessLabel?: string;
};

export function LiveDeskStatusBar({ profitModeEnabled, isSignedIn, readinessLabel }: LiveDeskStatusBarProps) {
  return (
    <div
      style={{
        display: "flex",
        flexWrap: "wrap",
        gap: 6,
        padding: "6px 0 4px",
        marginBottom: 4,
      }}
      aria-label="Desk status"
    >
      <DeskChip tone={profitModeEnabled ? "success" : "default"}>
        Profit mode {profitModeEnabled ? "ON" : "OFF"}
      </DeskChip>
      {readinessLabel && <DeskChip>{readinessLabel}</DeskChip>}
      <DeskChip tone={isSignedIn ? "success" : "default"}>
        {isSignedIn ? "Signed in" : "Anonymous"}
      </DeskChip>
    </div>
  );
}
