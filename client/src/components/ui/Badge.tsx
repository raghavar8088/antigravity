"use client";

import { cn } from "./cn";

export type BadgeVariant = "profit" | "loss" | "warning" | "info" | "caution" | "neutral";
export type BadgeSize = "sm" | "md";

export interface BadgeProps {
  variant?: BadgeVariant;
  size?: BadgeSize;
  /** Show an animated live-indicator dot before the label */
  dot?: boolean;
  children: React.ReactNode;
  className?: string;
}

const variantStyles: Record<BadgeVariant, string> = {
  profit:  "bg-[rgba(34,197,94,0.12)]  text-[var(--color-profit)]  border-[rgba(34,197,94,0.25)]",
  loss:    "bg-[rgba(239,68,68,0.12)]  text-[var(--color-loss)]    border-[rgba(239,68,68,0.25)]",
  warning: "bg-[rgba(249,115,22,0.12)] text-[var(--color-warning)] border-[rgba(249,115,22,0.25)]",
  info:    "bg-[rgba(59,130,246,0.12)] text-[var(--color-info)]    border-[rgba(59,130,246,0.25)]",
  caution: "bg-[rgba(234,179,8,0.12)]  text-[var(--color-caution)] border-[rgba(234,179,8,0.25)]",
  neutral: "bg-[rgba(100,116,139,0.12)] text-[var(--color-neutral)] border-[rgba(100,116,139,0.25)]",
};

const dotColors: Record<BadgeVariant, string> = {
  profit:  "bg-[var(--color-profit)]",
  loss:    "bg-[var(--color-loss)]",
  warning: "bg-[var(--color-warning)]",
  info:    "bg-[var(--color-info)]",
  caution: "bg-[var(--color-caution)]",
  neutral: "bg-[var(--color-neutral)]",
};

const sizeStyles: Record<BadgeSize, string> = {
  sm: "text-[11px] px-[6px] py-[2px] gap-[4px]",
  md: "text-[12px] px-[8px] py-[3px] gap-[5px]",
};

export function Badge({ variant = "neutral", size = "md", dot = false, children, className }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-[4px] border font-medium leading-none whitespace-nowrap",
        variantStyles[variant],
        sizeStyles[size],
        className,
      )}
    >
      {dot ? (
        <span
          className={cn("rounded-full shrink-0", dotColors[variant], size === "sm" ? "w-[5px] h-[5px]" : "w-[6px] h-[6px]")}
          aria-hidden
        />
      ) : null}
      {children}
    </span>
  );
}
