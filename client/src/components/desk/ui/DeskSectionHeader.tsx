"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

type DeskSectionHeaderProps = {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
  className?: string;
};

export function DeskSectionHeader({ title, subtitle, actions, className }: DeskSectionHeaderProps) {
  return (
    <div
      className={cn("desk-section-header", className)}
      style={{
        display: "flex",
        flexWrap: "wrap",
        alignItems: "flex-start",
        justifyContent: "space-between",
        gap: "var(--desk-space-3)",
        marginBottom: "var(--desk-space-4)",
      }}
    >
      <div>
        <h2 className="desk-title-md">{title}</h2>
        {subtitle ? (
          <p className="desk-label-md" style={{ marginTop: 4, fontWeight: 400 }}>
            {subtitle}
          </p>
        ) : null}
      </div>
      {actions ? <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>{actions}</div> : null}
    </div>
  );
}
