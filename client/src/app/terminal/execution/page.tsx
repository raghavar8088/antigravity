"use client";

import { ExecutionCenter } from "@/components/terminal/institutional/ExecutionCenter";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { TerminalAuthorityGuard } from "@/components/terminal/TerminalAuthorityGuard";

export default function ExecutionPage() {
  const snapshot = useTerminalSnapshot();
  return (
    <TerminalAuthorityGuard snapshot={snapshot}>
      <ExecutionCenter snapshot={snapshot} />
    </TerminalAuthorityGuard>
  );
}
