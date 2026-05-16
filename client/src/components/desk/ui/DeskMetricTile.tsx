"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

type DeskMetricTileProps = {
  label: string;
  value: ReactNode;
  detail?: ReactNode;
  valueClassName?: string;
  compact?: boolean;
};

export function DeskMetricTile({
  label,
  value,
  detail,
  valueClassName,
  compact = false,
}: DeskMetricTileProps) {
  return (
    <div
      className="desk-metric-tile"
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
