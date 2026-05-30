"use client";

import { useEffect, useRef, useState } from "react";
import { classifyMarketRegime, type MarketRegime, type RegimeSnapshot } from "@/lib/marketRegimeClassifier";
import type { OHLCVCandle } from "@/lib/mockResearchIndicators";

export function useMarketRegime(deps: {
  candles: OHLCVCandle[];
  newCandleReady: boolean;
}): { snapshot: RegimeSnapshot | null; regime: MarketRegime | null } {
  const [snapshot, setSnapshot] = useState<RegimeSnapshot | null>(null);
  const ref = useRef(deps);
  ref.current = deps;

  useEffect(() => {
    if (!deps.newCandleReady) return;
    const id = setTimeout(() => {
      if (ref.current.candles.length < 60) return;
      setSnapshot(classifyMarketRegime(ref.current.candles));
    }, 0);
    return () => clearTimeout(id);
  }, [deps.newCandleReady]);

  return { snapshot, regime: snapshot?.regime ?? null };
}
