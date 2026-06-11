"use client";

import { ResearchCenter } from "@/components/terminal/institutional/ResearchCenter";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { TerminalAuthorityGuard } from "@/components/terminal/TerminalAuthorityGuard";

export default function ResearchPage() {
  const snapshot = useTerminalSnapshot();
  return (
    <TerminalAuthorityGuard snapshot={snapshot}>
      <ResearchCenter snapshot={snapshot} />
    </TerminalAuthorityGuard>
  );
}
