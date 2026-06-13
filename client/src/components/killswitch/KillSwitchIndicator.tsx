"use client";

import { useKillSwitch } from "@/hooks/useKillSwitch";
import { cn } from "@/components/ui/cn";

/** Compact header indicator — always visible. Shows ARMED or TRIGGERED state. */
export function KillSwitchIndicator() {
  const { status } = useKillSwitch();

  const isActive = status.active;
  const isOffline = status.dataState !== "ok";

  return (
    <div
      className={cn(
        "flex items-center gap-1.5 text-[11px] font-semibold px-2 py-0.5 rounded-[4px]",
        isActive
          ? "text-[var(--color-loss)] bg-[rgba(239,68,68,0.12)] animate-pulse"
          : isOffline
          ? "text-[var(--color-warning)] bg-[rgba(249,115,22,0.08)]"
          : "text-[var(--color-profit)] bg-[rgba(34,197,94,0.08)]",
      )}
      role="status"
      aria-label={isActive ? "Kill switch triggered" : "Kill switch armed"}
    >
      <span
        className={cn(
          "w-[6px] h-[6px] rounded-full",
          isActive ? "bg-[var(--color-loss)] animate-pulse" : isOffline ? "bg-[var(--color-warning)]" : "bg-[var(--color-profit)] animate-pulse",
        )}
        aria-hidden
      />
      {isActive ? "TRIGGERED" : isOffline ? "OFFLINE" : "ARMED"}
    </div>
  );
}
