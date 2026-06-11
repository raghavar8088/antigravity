import type { ReactNode } from "react";
import { TerminalLayoutClient } from "@/components/terminal/institutional/TerminalLayoutClient";

export default function TerminalLayout({ children }: { children: ReactNode }) {
  return <TerminalLayoutClient>{children}</TerminalLayoutClient>;
}
