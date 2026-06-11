"use client";

import type { ReactNode } from "react";
import { TerminalAuthorityGuard } from "@/components/terminal/TerminalAuthorityGuard";
import { M3ErrorBoundary } from "@/components/ui/ErrorBoundary";
import { TerminalSnapshotProvider, useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { InstitutionalTerminalShell } from "./TerminalShell";

function TerminalLayoutInner({ children }: { children: ReactNode }) {
  const snapshot = useTerminalSnapshot();
  return (
    <InstitutionalTerminalShell>
      <M3ErrorBoundary fallbackTitle="Terminal panel failed to load">
        <TerminalAuthorityGuard snapshot={snapshot}>{children}</TerminalAuthorityGuard>
      </M3ErrorBoundary>
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
