"use client";

import { AnalyticsCenter } from "@/components/terminal/institutional/AnalyticsCenter";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { TerminalAuthorityGuard } from "@/components/terminal/TerminalAuthorityGuard";

export default function AnalyticsPage() {
  const snapshot = useTerminalSnapshot();
  return (
    <TerminalAuthorityGuard snapshot={snapshot}>
      <AnalyticsCenter snapshot={snapshot} />
    </TerminalAuthorityGuard>
  );
}
