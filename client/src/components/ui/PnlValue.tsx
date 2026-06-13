"use client";

import { cn } from "./cn";

export interface PnlValueProps {
  value: number;
  /** Always show + prefix on positive values */
  showSign?: boolean;
  /** Show ▲/▼ arrow prefix */
  showArrow?: boolean;
  size?: "xs" | "sm" | "md" | "lg" | "xl";
  /** Custom formatter — defaults to 2 decimal places */
  format?: (v: number) => string;
  className?: string;
}

const sizeClass: Record<NonNullable<PnlValueProps["size"]>, string> = {
  xs: "text-[11px]",
  sm: "text-[12px]",
  md: "text-[13px]",
  lg: "text-[16px]",
  xl: "text-[20px] font-semibold",
};

export function PnlValue({ value, showSign = false, showArrow = false, size = "md", format, className }: PnlValueProps) {
  const isPositive = value > 0;
  const isNegative = value < 0;

  const formatted = format ? format(value) : Math.abs(value).toLocaleString("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

  let prefix = "";
  if (showArrow) {
    prefix = isPositive ? "▲ " : isNegative ? "▼ " : "";
  } else if (showSign && isPositive) {
    prefix = "+";
  }

  const sign = isNegative && !showArrow ? "-" : "";

  return (
    <span
      className={cn(
        "font-mono tnum",
        isPositive ? "text-[var(--color-profit)]" : isNegative ? "text-[var(--color-loss)]" : "text-[var(--color-text-muted)]",
        sizeClass[size],
        className,
      )}
    >
      {prefix}{sign}{formatted}
    </span>
  );
}
