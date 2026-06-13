"use client";

import { cn } from "./cn";

export interface GaugeBarProps {
  /** Value from 0 to 100 */
  value: number;
  /** Optional label above bar */
  label?: string;
  /** Optional value label (defaults to `${value}%`) */
  valueLabel?: string;
  height?: number;
  className?: string;
  "aria-label"?: string;
}

function segmentColor(pct: number): string {
  if (pct < 60) return "var(--color-profit)";
  if (pct < 80) return "var(--color-caution)";
  if (pct < 90) return "var(--color-warning)";
  return "var(--color-loss)";
}

export function GaugeBar({ value, label, valueLabel, height = 6, className, "aria-label": ariaLabel }: GaugeBarProps) {
  const clamped = Math.max(0, Math.min(100, value));
  const color = segmentColor(clamped);
  const displayLabel = valueLabel ?? `${clamped.toFixed(1)}%`;

  return (
    <div className={cn("flex flex-col gap-[4px]", className)}>
      {(label || valueLabel !== undefined) ? (
        <div className="flex items-center justify-between">
          {label ? <span className="text-[11px] text-[var(--color-text-secondary)]">{label}</span> : <span />}
          <span className="text-[11px] font-mono tnum" style={{ color }}>{displayLabel}</span>
        </div>
      ) : null}
      <div
        className="relative w-full rounded-full overflow-hidden"
        style={{ height, background: "var(--color-bg-elevated)" }}
        role="meter"
        aria-valuenow={clamped}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label={ariaLabel ?? label}
      >
        {/* Threshold markers */}
        {[60, 80, 90].map((t) => (
          <span
            key={t}
            className="absolute top-0 bottom-0 w-px"
            style={{ left: `${t}%`, background: "var(--color-bg-overlay)", opacity: 0.5 }}
            aria-hidden
          />
        ))}
        <div
          className="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
          style={{ width: `${clamped}%`, background: color }}
          aria-hidden
        />
      </div>
    </div>
  );
}
