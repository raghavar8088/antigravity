import type { ReactNode } from "react";
import { InstitutionalTerminalShell } from "@/components/terminal/institutional/TerminalShell";

export default function TerminalLayout({ children }: { children: ReactNode }) {
  return <InstitutionalTerminalShell>{children}</InstitutionalTerminalShell>;
}
