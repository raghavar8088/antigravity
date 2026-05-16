"use client";

import type { ReactNode } from "react";
import { DeskLinearProgress } from "./DeskLinearProgress";
import { cn } from "./cn";

type DeskShellProps = {
  children: ReactNode;
  appBar?: ReactNode;
  loading?: boolean;
  className?: string;
};

export function DeskShell({ children, appBar, loading = false, className }: DeskShellProps) {
  return (
    <div className={cn("desk-shell", className)} style={{ minHeight: "100%", background: "var(--desk-surface-dim)" }}>
      {appBar}
      <DeskLinearProgress visible={loading} />
      <div className="desk-page">{children}</div>
    </div>
  );
}
