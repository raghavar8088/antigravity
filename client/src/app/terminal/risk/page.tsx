"use client";

import { RiskModule } from "@/components/terminal/institutional/RiskModule";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";

export default function RiskPage() {
  const snapshot = useTerminalSnapshot();
  return <RiskModule snapshot={snapshot} />;
}
