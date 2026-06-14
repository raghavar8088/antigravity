"use client";

import type { StrategyWithMetrics } from "@/lib/strategyAuthority/types";

/** @deprecated Promotion pipeline removed. */
export function PromotionProgressCard({ strategy }: { strategy: StrategyWithMetrics }) {
  void strategy;
  return null;
}

/** @deprecated Promotion pipeline removed. */
export function PromotionProgressGrid({ strategies }: { strategies: StrategyWithMetrics[] }) {
  void strategies;
  return null;
}
