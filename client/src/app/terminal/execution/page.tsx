"use client";

import { ExecutionCenter } from "@/components/terminal/institutional/ExecutionCenter";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";

export default function ExecutionPage() {
  const snapshot = useTerminalSnapshot();
  return <ExecutionCenter snapshot={snapshot} />;
}
