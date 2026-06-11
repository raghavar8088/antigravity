"use client";

import { ResearchCenter } from "@/components/terminal/institutional/ResearchCenter";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";

export default function ResearchPage() {
  const snapshot = useTerminalSnapshot();
  return <ResearchCenter snapshot={snapshot} />;
}
