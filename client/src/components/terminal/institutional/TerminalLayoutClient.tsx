"use client";

import type { ReactNode } from "react";
import { TerminalAuthorityGuard } from "@/components/terminal/TerminalAuthorityGuard";
import { TerminalSnapshotProvider, useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { InstitutionalTerminalShell } from "./TerminalShell";

function TerminalLayoutInner({ children }: { children: ReactNode }) {
  const snapshot = useTerminalSnapshot();
  return (
    <InstitutionalTerminalShell>
      <TerminalAuthorityGuard snapshot={snapshot}>{children}</TerminalAuthorityGuard>
    </InstitutionalTerminalShell>
  );
}

export function TerminalLayoutClient({ children }: { children: ReactNode }) {
  return (
    <TerminalSnapshotProvider>
      <TerminalLayoutInner>{children}</TerminalLayoutInner>
    </TerminalSnapshotProvider>
  );
}
