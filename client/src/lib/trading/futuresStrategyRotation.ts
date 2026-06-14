import type { StrategyDiagnosticRow } from "./futuresStrategyDiagnostics";

export type RotationStatus =
  | "PROMOTED"
  | "ACTIVE"
  | "PROBATION"
  | "SUSPENDED"
  | "INSUFFICIENT";

export interface RotationScore {
  strategyId: number;
  strategyName: string;
  score: number;
  status: RotationStatus;
  rank: number;
  reasoning: string;
}

export interface RotationReport {
  scores: RotationScore[];
  promoted: RotationScore[];
  active: RotationScore[];
  probation: RotationScore[];
  suspended: RotationScore[];
  insufficient: RotationScore[];
  topStrategyId: number | null;
}

const MIN_TRADES = 5;

function scoreRow(row: StrategyDiagnosticRow): number {
  const feeFactor = Math.max(0, 1 - row.feePctOfAbsGross);
  const pfCapped = Math.min(row.profitFactor, 5) / 5;
  const expectancyFactor = row.avgNetPnl > 0 ? Math.min(row.avgNetPnl / 20, 1) : 0;
  const slRate = row.totalTrades > 0 ? row.slCount / row.totalTrades : 1;

  return (
    row.winRate * 30 +
    pfCapped * 25 +
    feeFactor * 20 +
    expectancyFactor * 15 +
    (1 - slRate) * 10
  );
}

function toStatus(score: number): RotationStatus {
  if (score >= 60) return "PROMOTED";
  if (score >= 40) return "ACTIVE";
  if (score >= 25) return "PROBATION";
  return "SUSPENDED";
}

export function computeStrategyRotation(rows: StrategyDiagnosticRow[]): RotationReport {
  const eligible = rows.filter((r) => !r.isProbe);

  const scored: RotationScore[] = eligible.map((row) => {
    if (row.totalTrades < MIN_TRADES) {
      return {
        strategyId: row.strategyId,
        strategyName: row.strategyName,
        score: 0,
        status: "INSUFFICIENT" as RotationStatus,
        rank: 0,
        reasoning: `Score: 0 | Status: INSUFFICIENT (${row.totalTrades} trades, need ${MIN_TRADES})`,
      };
    }

    const score = Math.max(0, Math.min(100, scoreRow(row)));
    const status = toStatus(score);

    return {
      strategyId: row.strategyId,
      strategyName: row.strategyName,
      score,
      status,
      rank: 0,
      reasoning: `Score: ${score.toFixed(1)} | Status: ${status} | WR:${(row.winRate * 100).toFixed(0)}% PF:${row.profitFactor.toFixed(2)} E:$${row.avgNetPnl.toFixed(2)}`,
    };
  });

  const rankable = scored
    .filter((s) => s.status !== "INSUFFICIENT")
    .sort((a, b) => b.score - a.score);

  rankable.forEach((s, i) => {
    s.rank = i + 1;
  });

  const all = scored;
  const topStrategyId = rankable.length > 0 ? rankable[0].strategyId : null;

  return {
    scores: all,
    promoted: all.filter((s) => s.status === "PROMOTED"),
    active: all.filter((s) => s.status === "ACTIVE"),
    probation: all.filter((s) => s.status === "PROBATION"),
    suspended: all.filter((s) => s.status === "SUSPENDED"),
    insufficient: all.filter((s) => s.status === "INSUFFICIENT"),
    topStrategyId,
  };
}

export function getSuspendedStrategyIds(report: RotationReport): Set<number> {
  return new Set(report.suspended.map((s) => s.strategyId));
}
