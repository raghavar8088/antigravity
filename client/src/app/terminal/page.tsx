"use client";

import { CommandCenterHome } from "@/components/terminal/institutional/CommandCenterHome";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";

export default function CommandCenterPage() {
  const snapshot = useTerminalSnapshot();
  return <CommandCenterHome snapshot={snapshot} />;
}
