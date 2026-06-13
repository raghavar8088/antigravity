"use client";

import { useSession } from "@/context/SessionContext";
import { cn } from "@/components/ui/cn";

function pad(n: number) {
  return String(n).padStart(2, "0");
}

function formatTime(d: Date): string {
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

export function SessionClock() {
  const { nowIST, sessionLabel, isHoliday, nseOpen, nextEventLabel } = useSession();

  const sessionColor =
    isHoliday       ? "text-[var(--color-neutral)]"  :
    nseOpen          ? "text-[var(--color-profit)]"   :
    sessionLabel === "PRE-MARKET" || sessionLabel === "POST-MARKET"
                     ? "text-[var(--color-caution)]"  :
    /* CLOSED */       "text-[var(--color-neutral)]";

  return (
    <div
      className="flex items-center gap-3 text-[11px]"
      aria-label={`Session clock: ${sessionLabel}`}
    >
      {/* IST Clock */}
      <span className="font-mono tnum text-[var(--color-text-secondary)]">
        {formatTime(nowIST)} IST
      </span>

      {/* NSE/BSE session badge */}
      <span
        className={cn(
          "inline-flex items-center gap-[4px] font-medium",
          sessionColor,
        )}
      >
        {nseOpen ? (
          <span className="w-[5px] h-[5px] rounded-full bg-[var(--color-profit)] animate-pulse" aria-hidden />
        ) : null}
        NSE {sessionLabel}
      </span>

      {/* Countdown — only show when approaching event */}
      {nextEventLabel !== "—" && !isHoliday ? (
        <span className="text-[var(--color-warning)] font-mono tnum hidden xl:inline">
          {nextEventLabel}
        </span>
      ) : null}

      {/* Crypto always open */}
      <span className="text-[var(--color-profit)] hidden lg:inline">
        <span className="w-[5px] h-[5px] rounded-full bg-[var(--color-profit)] inline-block mr-1 align-middle animate-pulse" aria-hidden />
        24/7
      </span>
    </div>
  );
}
