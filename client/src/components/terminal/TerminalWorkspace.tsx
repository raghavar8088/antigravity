"use client";

import type { ReactNode } from "react";
import { cn } from "../desk/ui/cn";

type TerminalWorkspaceProps = {
  children: ReactNode;
  className?: string;
  columns?: number | string;
  gap?: number;
};

export function TerminalWorkspace({
  children,
  className,
  columns = "repeat(12, 1fr)",
  gap = 12,
}: TerminalWorkspaceProps) {
  return (
    <div
      className={cn("terminal-workspace", className)}
      style={{
        display: "grid",
        gridTemplateColumns: typeof columns === "number" ? `repeat(${columns}, 1fr)` : columns,
        gap: gap,
        width: "100%",
        alignItems: "start",
      }}
    >
      {children}
    </div>
  );
}
