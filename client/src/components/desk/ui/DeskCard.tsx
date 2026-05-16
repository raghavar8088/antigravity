"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

type DeskCardProps = {
  children: ReactNode;
  className?: string;
  elevation?: 0 | 1 | 2 | 3;
  variant?: "filled" | "outlined";
  padding?: "none" | "md" | "lg";
};

const elevationShadow: Record<0 | 1 | 2 | 3, string> = {
  0: "var(--desk-elevation-0)",
  1: "var(--desk-elevation-1)",
  2: "var(--desk-elevation-2)",
  3: "var(--desk-elevation-3)",
};

export function DeskCard({
  children,
  className,
  elevation = 1,
  variant = "filled",
  padding = "lg",
}: DeskCardProps) {
  const pad =
    padding === "none" ? "0" : padding === "md" ? "var(--desk-space-4)" : "var(--desk-space-5)";

  return (
    <section
      className={cn("desk-card", className)}
      style={{
        background: variant === "outlined" ? "transparent" : "var(--desk-surface)",
        border: `1px solid var(--desk-outline)`,
        borderRadius: "var(--desk-radius-card)",
        boxShadow: elevationShadow[elevation],
        padding: pad,
      }}
    >
      {children}
    </section>
  );
}
