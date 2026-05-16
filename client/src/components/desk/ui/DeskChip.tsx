"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

type DeskChipTone = "default" | "primary" | "success" | "error" | "warning";

type DeskChipProps = {
  children: ReactNode;
  tone?: DeskChipTone;
  className?: string;
  title?: string;
};

const toneStyles: Record<DeskChipTone, { bg: string; color: string; border: string }> = {
  default: {
    bg: "var(--desk-surface-container)",
    color: "var(--desk-on-surface-variant)",
    border: "var(--desk-outline)",
  },
  primary: {
    bg: "var(--desk-primary-container)",
    color: "var(--desk-on-primary-container)",
    border: "transparent",
  },
  success: {
    bg: "var(--desk-success-container)",
    color: "var(--desk-success)",
    border: "transparent",
  },
  error: {
    bg: "var(--desk-error-container)",
    color: "var(--desk-error)",
    border: "transparent",
  },
  warning: {
    bg: "var(--desk-warning-container)",
    color: "var(--desk-warning)",
    border: "transparent",
  },
};

export function DeskChip({ children, tone = "default", className, title }: DeskChipProps) {
  const t = toneStyles[tone];
  return (
    <span
      title={title}
      className={cn("desk-chip", className)}
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 6,
        minHeight: 28,
        padding: "0 10px",
        borderRadius: "var(--desk-radius-chip)",
        border: `1px solid ${t.border}`,
        background: t.bg,
        color: t.color,
        fontSize: "0.75rem",
        fontWeight: 500,
        lineHeight: 1.2,
      }}
    >
      {children}
    </span>
  );
}
