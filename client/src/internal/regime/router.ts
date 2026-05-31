import type { FuturesStratDef } from "@/lib/futuresStratTypes";
import type { RegimeState, RegimeType } from "@/internal/regime";

const ACTIVE_CATEGORY_HINTS: Record<RegimeType, readonly string[]> = {
  TRENDING_BULL: ["Trend", "Breakout", "Momentum", "MTF", "Session", "Order Flow", "Smart Money"],
  TRENDING_BEAR: ["Trend", "Breakout", "Momentum", "MTF", "Session", "Order Flow", "Smart Money"],
  RANGING: ["Mean", "Range", "VWAP", "RSI", "BB", "Wyckoff"],
  VOLATILE: ["Breakout", "Momentum", "Order Flow", "Smart Money", "Liquidity"],
  LOW_VOL: ["Mean", "Range", "VWAP", "Squeeze", "Wyckoff"],
};

const DISABLED_CATEGORY_HINTS: Record<RegimeType, readonly string[]> = {
  TRENDING_BULL: ["Mean Reversion", "Range Fade"],
  TRENDING_BEAR: ["Mean Reversion", "Range Fade"],
  RANGING: ["Trend Following", "Trend", "Breakout"],
  VOLATILE: ["Slow Mean", "Position"],
  LOW_VOL: ["Breakout", "Momentum"],
};

function hasHint(strat: FuturesStratDef, hints: readonly string[]): boolean {
  const haystack = [
    strat.category,
    strat.name,
    strat.templateFamily,
    strat.btcFtTemplate,
    ...(strat.playbooks ?? []),
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
  return hints.some((hint) => haystack.includes(hint.toLowerCase()));
}

export class RegimeStrategyRouter {
  route(strategies: readonly FuturesStratDef[], regime: RegimeState): FuturesStratDef[] {
    const activeHints = ACTIVE_CATEGORY_HINTS[regime.regime];
    const disabledHints = DISABLED_CATEGORY_HINTS[regime.regime];

    return strategies.filter((strat) => {
      if (strat.researchOnly) return false;
      if (hasHint(strat, disabledHints) && !hasHint(strat, activeHints)) return false;
      if (hasHint(strat, activeHints)) return true;
      return regime.regime === "RANGING" || regime.regime === "LOW_VOL";
    });
  }
}
