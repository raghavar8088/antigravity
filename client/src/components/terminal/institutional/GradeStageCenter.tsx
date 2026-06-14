"use client";

import type { StrategyStatus } from "@/lib/strategyAuthority/types";
import { TradeEngineCenter } from "./TradeEngineCenter";

/** @deprecated Grade stages removed. Always renders TradeEngineCenter. */
export function GradeStageCenter({ status }: { status: StrategyStatus }) {
  void status;
  return <TradeEngineCenter />;
}
