"use client";

import { cn } from "./cn";

export type StatusDotVariant = "live" | "warning" | "error" | "inactive";

export interface StatusDotProps {
  variant?: StatusDotVariant;
  size?: number;
  className?: string;
  "aria-label"?: string;
}

const dotColor: Record<StatusDotVariant, string> = {
  live:     "bg-[var(--color-profit)]",
  warning:  "bg-[var(--color-warning)]",
  error:    "bg-[var(--color-loss)]",
  inactive: "bg-[var(--color-neutral)]",
};

const ringColor: Record<StatusDotVariant, string> = {
  live:     "bg-[rgba(34,197,94,0.3)]",
  warning:  "bg-[rgba(249,115,22,0.3)]",
  error:    "bg-[rgba(239,68,68,0.3)]",
  inactive: "",
};

export function StatusDot({ variant = "live", size = 8, className, "aria-label": ariaLabel }: StatusDotProps) {
  const isAnimated = variant !== "inactive";
  return (
    <span
      className={cn("relative inline-flex items-center justify-center shrink-0", className)}
      style={{ width: size + 8, height: size + 8 }}
      role="img"
      aria-label={ariaLabel ?? variant}
    >
      {isAnimated ? (
        <span
          className={cn("absolute rounded-full animate-ping opacity-75", ringColor[variant])}
          style={{ width: size + 6, height: size + 6 }}
          aria-hidden
        />
      ) : null}
      <span
        className={cn("relative rounded-full", dotColor[variant])}
        style={{ width: size, height: size }}
        aria-hidden
      />
    </span>
  );
}
