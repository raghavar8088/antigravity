"use client";

import { TradeJournalPro } from "@/components/terminal/institutional/TradeJournalPro";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { TerminalAuthorityGuard } from "@/components/terminal/TerminalAuthorityGuard";

export default function JournalPage() {
  const snapshot = useTerminalSnapshot();
  return (
    <TerminalAuthorityGuard snapshot={snapshot}>
      <TradeJournalPro snapshot={snapshot} />
    </TerminalAuthorityGuard>
  );
}
