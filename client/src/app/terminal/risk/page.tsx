"use client";

import { RiskModule } from "@/components/terminal/institutional/RiskModule";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { TerminalAuthorityGuard } from "@/components/terminal/TerminalAuthorityGuard";

export default function RiskPage() {
  const snapshot = useTerminalSnapshot();
  return (
    <TerminalAuthorityGuard snapshot={snapshot}>
      <RiskModule snapshot={snapshot} />
    </TerminalAuthorityGuard>
  );
}
