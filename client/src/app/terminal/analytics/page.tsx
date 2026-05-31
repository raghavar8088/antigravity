"use client";

import { AnalyticsCenter } from "@/components/terminal/institutional/AnalyticsCenter";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";

export default function AnalyticsPage() {
  const snapshot = useTerminalSnapshot();
  return <AnalyticsCenter snapshot={snapshot} />;
}
