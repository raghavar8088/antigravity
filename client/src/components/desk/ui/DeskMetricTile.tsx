"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

type DeskMetricTileProps = {
  label: string;
  value: ReactNode;
  detail?: ReactNode;
  valueClassName?: string;
  compact?: boolean;
  highlight?: boolean;
  title?: string;
};

export function DeskMetricTile({
  label,
  value,
  detail,
  valueClassName,
  compact = false,
  highlight = false,
  title,
}: DeskMetricTileProps) {
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
      {detail ? (
        <span className="desk-label-md" style={{ fontWeight: 400, minHeight: 16 }}>
          {detail}
        </span>
      ) : (
        <span aria-hidden style={{ minHeight: 16 }} />
      )}
    </div>
  );
}
