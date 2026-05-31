"use client";

import { TradeJournalPro } from "@/components/terminal/institutional/TradeJournalPro";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";

export default function JournalPage() {
  const snapshot = useTerminalSnapshot();
  return <TradeJournalPro snapshot={snapshot} />;
}
