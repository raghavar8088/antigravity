"use client";

import { usePathname } from "next/navigation";
import RiskRibbon from "@/components/RiskRibbon";
import { isTerminalRoute } from "@/lib/navRoutes";

/** Renders global risk ribbon except on /terminal where the M3 shell embeds its own copy. */
export function GlobalRiskRibbon() {
  const pathname = usePathname();
  if (isTerminalRoute(pathname)) return null;
  return <RiskRibbon />;
}
