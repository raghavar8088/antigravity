import {
  createTraceRow,
  type StrategySignalTraceRow,
} from "@/lib/strategySignalTrace";
import { isExecutableTraceRow } from "@/lib/mockTradingEngine";
import type { ISPAPCatalogEntry } from "./types";

const FAMILY_BUCKETS: Record<string, readonly string[]> = {
  trend: ["trend", "mtf trend", "mtf macd", "mtf adx", "mtf break", "adaptive elite", "time-of-day"],
  mean: ["mean reversion", "mean rev elite", "statistical"],
  breakout: ["breakout", "breakout elite", "price action elite"],
  volatility: ["volatility"],
  momentum: ["momentum", "momentum elite", "multi-signal"],
  micro: ["microstructure", "order flow", "smart money"],
  structure: ["structure", "liquidity", "phase 11 structure", "phase 11 smart money", "phase 11 liquidity"],
  funding: ["funding", "phase 11 derivatives", "phase 11 order flow", "phase 11 liquidations"],
  session: ["session", "intraday", "market profile"],
};

function normalizeFamily(value: string | undefined): string {
  return (value ?? "unknown").trim().toLowerCase();
}

function familyBucket(value: string | undefined): string {
  const normalized = normalizeFamily(value);
  for (const [bucket, aliases] of Object.entries(FAMILY_BUCKETS)) {
    if (aliases.some((alias) => normalized.includes(alias) || alias.includes(normalized))) {
      return bucket;
    }
  }
  return normalized;
}

/** Stable numeric id for ISPAP catalog slugs — avoids collision with futures desk ids. */
export function catalogStrategyNumericId(catalogId: string): number {
  let h = 2_166_136_261;
  for (let i = 0; i < catalogId.length; i++) {
    h ^= catalogId.charCodeAt(i);
    h = Math.imul(h, 1_677_761_9);
  }
  return 900_000 + ((h >>> 0) % 99_000);
}

/**
 * Fan executable desk signals out to all Grade 5 catalog strategies so ISPAP
 * metrics and mock_trades align with strategy_authority_profiles.
 */
export function fanOutGrade5CatalogSignals(args: {
  catalog: readonly ISPAPCatalogEntry[];
  baseRows: readonly StrategySignalTraceRow[];
  tickAt: number;
  symbol: string;
  regime: string;
}): StrategySignalTraceRow[] {
  const executables = args.baseRows.filter(isExecutableTraceRow);
  if (executables.length === 0) return [];

  const byBucket = new Map<string, StrategySignalTraceRow>();
  for (const row of executables) {
    const bucket = familyBucket(row.category);
    const existing = byBucket.get(bucket);
    if (!existing || row.signalScore > existing.signalScore) {
      byBucket.set(bucket, row);
    }
  }

  const globalBest = executables.reduce((best, row) =>
    row.signalScore > best.signalScore ? row : best,
  );

  const rows: StrategySignalTraceRow[] = [];
  for (const entry of args.catalog) {
    const template = byBucket.get(familyBucket(entry.family)) ?? globalBest;
    if (!template.side) continue;

    rows.push(
      createTraceRow({
        traceId: `g5-${Math.floor(args.tickAt / 60_000)}-${entry.id}`,
        tickAt: args.tickAt,
        mode: "browser",
        symbol: args.symbol,
        strategyId: catalogStrategyNumericId(entry.id),
        strategyName: entry.name,
        category: entry.category,
        side: template.side,
        status: "CANDIDATE",
        gate: "OPENED",
        reason: "grade-5 discovery candidate",
        signalScore: template.signalScore,
        requiredThreshold: template.requiredThreshold,
        confirmPassed: true,
        regime: args.regime,
        regimeAllowed: true,
        feeHurdlePassed: true,
        openAttempted: true,
        ispapStrategyId: entry.id,
        pipelineStage: "GRADE_5",
      }),
    );
  }

  return rows;
}
