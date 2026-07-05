/**
 * Consolidated into @/components/ui/Card. This file is kept as a re-export
 * shim so the ~24 existing import sites across the terminal don't need
 * mechanical churn; TerminalCard's old auto-tone-border behavior is
 * preserved via a thin wrapper that passes autoTone={true}.
 */
import type { ComponentProps } from "react";
import { Card, Metric } from "@/components/ui/Card";

export function TerminalCard(props: ComponentProps<typeof Card>) {
  return <Card autoTone {...props} />;
}

export { Metric };
