"use client";

import type { ReactNode } from "react";

type DeskEmptyStateProps = {
  icon?: ReactNode;
  title: string;
  subtitle?: string;
  action?: ReactNode;
};

export function DeskEmptyState({ icon, title, subtitle, action }: DeskEmptyStateProps) {
  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: "var(--desk-space-3)",
        padding: "var(--desk-space-6) var(--desk-space-4)",
        textAlign: "center",
        borderRadius: "var(--desk-radius-card)",
        border: "1px dashed var(--desk-outline)",
        background: "var(--desk-surface-container)",
        minHeight: 120,
      }}
    >
      {icon ? (
        <div style={{ color: "var(--desk-on-surface-variant)", opacity: 0.7 }} aria-hidden>
          {icon}
        </div>
      ) : null}
      <p className="desk-title-md">{title}</p>
      {subtitle ? (
        <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", maxWidth: 360 }}>
          {subtitle}
        </p>
      ) : null}
      {action}
    </div>
  );
}
