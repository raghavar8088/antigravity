"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

type DeskMetricTileProps = {
  label: string;
  value: ReactNode;
  detail?: ReactNode;
  sub?: ReactNode;
  subColor?: "profit" | "loss" | "neutral" | "default";
  valueClassName?: string;
  compact?: boolean;
  highlight?: boolean;
  title?: string;
};

export function DeskMetricTile({
  label,
  value,
  detail,
  sub,
  subColor = "default",
  valueClassName,
  compact = false,
  highlight = false,
  title,
}: DeskMetricTileProps) {
  const finalDetail = detail ?? sub;
  const detailColor =
    subColor === "profit"
      ? "var(--desk-success)"
      : subColor === "loss"
      ? "var(--desk-error)"
      : subColor === "neutral"
      ? "var(--desk-on-surface-variant)"
      : "var(--desk-on-surface-variant)";

  return (
    <div
      title={title}
      className={cn(
        "desk-metric-tile",
        highlight && "desk-metric-tile--highlight",
        valueClassName?.includes("desk-pnl-positive") && "desk-metric-tile--pnl-positive",
        valueClassName?.includes("desk-pnl-negative") && "desk-metric-tile--pnl-negative",
      )}
      style={{
        background: "var(--desk-surface-container)",
        borderRadius: "var(--desk-radius-chip)",
        padding: compact ? "var(--desk-space-3)" : "var(--desk-space-4)",
        minHeight: compact ? 72 : 88,
        display: "flex",
        flexDirection: "column",
        justifyContent: "space-between",
        gap: "var(--desk-space-1)",
      }}
    >
      <span className="desk-label-md">{label}</span>
      <span
        className={cn("desk-mono", compact ? "desk-title-md" : "desk-display-lg", valueClassName)}
        style={compact ? { fontSize: "1.125rem" } : undefined}
      >
        {value}
      </span>
      {finalDetail ? (
        <span className="desk-label-md" style={{ fontWeight: 400, minHeight: 16, color: detailColor }}>
          {finalDetail}
        </span>
      ) : (
        <span aria-hidden style={{ minHeight: 16 }} />
      )}
    </div>
  );
}
