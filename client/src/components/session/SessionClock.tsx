"use client";

import { useSession } from "@/context/SessionContext";

function pad(n: number) {
  return String(n).padStart(2, "0");
}

function formatTime(d: Date): string {
  return `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`;
}

export function SessionClock() {
  const { nowUtc, sessionLabel, nextEventLabel } = useSession();

  return (
    <div
      className="flex items-center gap-3 text-[11px]"
      aria-label={`Session clock: ${sessionLabel}`}
    >
      <span className="font-mono tnum text-[var(--color-text-secondary)]">
        {formatTime(nowUtc)} UTC
      </span>

      <span className="inline-flex items-center gap-[4px] font-medium text-[var(--color-profit)]">
        <span className="w-[5px] h-[5px] rounded-full bg-[var(--color-profit)] animate-pulse" aria-hidden />
        BTC {sessionLabel}
      </span>

      <span className="text-[var(--color-text-muted)] hidden xl:inline">{nextEventLabel}</span>
    </div>
  );
}
