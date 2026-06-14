/**
 * Grade-5 catalog signal fan-out.
 *
 * Takes base signal trace rows produced for the standard desk strategy roster
 * and multiplies them across each ISPAPCatalogEntry in the strategy catalog so
 * Grade-5 strategies get individual trace rows tracked in MongoDB.
 */

import type { ISPAPCatalogEntry } from "./types";
import type { StrategySignalTraceRow } from "@/lib/ai/strategySignalTrace";

export interface FanOutArgs {
  catalog: readonly ISPAPCatalogEntry[];
  baseRows: readonly StrategySignalTraceRow[];
  tickAt: number;
  symbol: string;
  regime: string;
}

/**
 * Maps each catalog entry to a trace row derived from the closest matching
 * base row (by signalKey) or the best CANDIDATE row if no match is found.
 */
export function fanOutGrade5CatalogSignals(args: FanOutArgs): StrategySignalTraceRow[] {
  const { catalog, baseRows, tickAt, symbol, regime } = args;

  if (catalog.length === 0) return [...baseRows];

  const candidates = baseRows.filter((r) => r.status === "CANDIDATE");
  const bestBase: StrategySignalTraceRow | undefined =
    candidates[0] ??
    baseRows.reduce<StrategySignalTraceRow | undefined>((best, r) => {
      if (!best) return r;
      return r.signalScore > best.signalScore ? r : best;
    }, undefined);

  const keyedBySignal = new Map<string, StrategySignalTraceRow>();
  for (const r of baseRows) {
    if (r.gate) keyedBySignal.set(r.gate, r);
  }

  return catalog.map((entry): StrategySignalTraceRow => {
    const matched = keyedBySignal.get(entry.signalKey) ?? bestBase;

    const isCandidate = matched?.status === "CANDIDATE";

    return {
      traceId: `grade5-${Math.floor(tickAt / 60_000)}-${entry.id}`,
      tickAt,
      mode: "browser",
      symbol,
      strategyId: entry.id,
      strategyName: entry.name,
      category: entry.category,
      side: matched?.side,
      status: isCandidate ? "CANDIDATE" : "REJECTED",
      gate: matched?.gate ?? "NO_BASE",
      reason: matched?.reason ?? "no base signal",
      signalScore: matched?.signalScore ?? 0,
      requiredThreshold: matched?.requiredThreshold ?? 0,
      confirmPassed: matched?.confirmPassed ?? false,
      feeHurdlePassed: matched?.feeHurdlePassed,
      openAttempted: isCandidate,
      regime,
      regimeAllowed: matched?.regimeAllowed ?? true,
      atrPct: matched?.atrPct,
      contributions: matched?.contributions,
    };
  });
}
