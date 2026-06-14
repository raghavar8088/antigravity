import type { StrategyStatus } from "@/lib/strategyAuthority/types";

export interface StageSummary {
  totalStrategies: number;
  promotionCandidates: number;
  demotionCandidates: number;
  avgProfitFactor: number;
  avgSharpe: number;
  avgExpectancy: number;
  avgDrawdown: number;
}

export interface FamilyIntelligenceRow {
  family: string;
  total: number;
  avgProfitFactor: number;
  avgWinRate: number;
  avgExpectancy: number;
  avgDrawdown: number;
  avgSharpe: number;
  mainEngineCount: number;
  retiredCount: number;
  promotionCandidates: number;
  demotionRisk: number;
  byStatus: Partial<Record<StrategyStatus, number>>;
}
