"use client";

import { usePathname } from "next/navigation";
import RiskRibbon from "@/components/RiskRibbon";
import { isMockTradingRoute, isTerminalRoute } from "@/lib/navRoutes";

/** Shell routes embed their own risk ribbon — skip global duplicate. */
export function GlobalRiskRibbon() {
  const pathname = usePathname();
  if (isTerminalRoute(pathname) || isMockTradingRoute(pathname)) return null;
  return <RiskRibbon />;
}
