import type { StrategyHealthRow } from "@/lib/strategyHealthEngine";
import type { StrategyScore } from "@/lib/strategyScoringEngine";

export interface StrategyAllocationRow {
  strategyId: number;
  strategyName: string;
  family: string;
  state: StrategyHealthRow["state"];
  scoreWeight: number;
  allocationPct: number;
  capitalUsd: number;
  trustScore: number;
}

export interface FamilyAllocationRow {
  family: string;
  allocationPct: number;
  capitalUsd: number;
  strategyCount: number;
}

export interface PortfolioAllocationResult {
  strategyRows: StrategyAllocationRow[];
  familyRows: FamilyAllocationRow[];
  unallocatedPct: number;
}

function clamp(value: number, min = 0, max = 1): number {
  if (!Number.isFinite(value)) return min;
  return Math.min(max, Math.max(min, value));
}

export function computePortfolioAllocation(args: {
  scores: readonly StrategyScore[];
  healthRows: readonly StrategyHealthRow[];
  equity: number;
  strategyFamilyById?: ReadonlyMap<number, string>;
  maxStrategyWeightPct?: number;
  maxTotalAllocationPct?: number;
}): PortfolioAllocationResult {
  const maxStrategyWeightPct = args.maxStrategyWeightPct ?? 0.12;
  const maxTotalAllocationPct = args.maxTotalAllocationPct ?? 1;
  const healthById = new Map(args.healthRows.map((row) => [row.strategyId, row]));
  const candidates = args.scores
    .map((score) => ({ score, health: healthById.get(score.strategyId) }))
    .filter((item): item is { score: StrategyScore; health: StrategyHealthRow } => item.health != null)
    .filter((item) => item.health.state === "ACTIVE")
    .map((item) => {
      const family = args.strategyFamilyById?.get(item.score.strategyId) ?? "Research";
      const scoreWeight =
        Math.max(0, item.score.overallScore) *
        Math.max(0, item.score.currentRegimeScore) *
        Math.max(0, item.health.trustScore) /
        1_000_000;
      return { ...item, family, scoreWeight };
    })
    .filter((item) => item.scoreWeight > 0);

  const totalRaw = candidates.reduce((sum, item) => sum + item.scoreWeight, 0);
  const strategyRows = candidates.map((item) => {
    const rawPct = totalRaw > 0 ? item.scoreWeight / totalRaw : 0;
    const allocationPct = clamp(rawPct * maxTotalAllocationPct, 0, maxStrategyWeightPct);
    return {
      strategyId: item.score.strategyId,
      strategyName: item.score.strategyName,
      family: item.family,
      state: item.health.state,
      scoreWeight: item.scoreWeight,
      allocationPct,
      capitalUsd: args.equity * allocationPct,
      trustScore: item.health.trustScore,
    };
  });

  const allocatedPct = strategyRows.reduce((sum, row) => sum + row.allocationPct, 0);
  if (allocatedPct > maxTotalAllocationPct && allocatedPct > 0) {
    const scale = maxTotalAllocationPct / allocatedPct;
    for (const row of strategyRows) {
      row.allocationPct *= scale;
      row.capitalUsd = args.equity * row.allocationPct;
    }
  }

  const familyMap = new Map<string, FamilyAllocationRow>();
  for (const row of strategyRows) {
    const family = row.family;
    const existing = familyMap.get(family) ?? { family, allocationPct: 0, capitalUsd: 0, strategyCount: 0 };
    existing.allocationPct += row.allocationPct;
    existing.capitalUsd += row.capitalUsd;
    existing.strategyCount++;
    familyMap.set(family, existing);
  }

  return {
    strategyRows: strategyRows.sort((a, b) => b.allocationPct - a.allocationPct),
    familyRows: [...familyMap.values()].sort((a, b) => b.allocationPct - a.allocationPct),
    unallocatedPct: clamp(1 - strategyRows.reduce((sum, row) => sum + row.allocationPct, 0)),
  };
}
