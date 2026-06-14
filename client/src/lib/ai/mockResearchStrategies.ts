/**
 * Mock research strategy registry.
 *
 * All mock research strategies have been removed from the application. The
 * exported types and empty collections remain for runners and UI filters.
 */

import type { OHLCVCandle, ResearchSignal } from "@/lib/ai/mockResearchIndicators";

export type ResearchFamily = string;

export interface ResearchStrategy {
  id: number;
  name: string;
  family: ResearchFamily;
  enabled: true;
  description: string;
  params: Record<string, number | string>;
  timeframe: "1m" | "5m" | "15m";
  minCandles: number;
  signal: (candles: OHLCVCandle[]) => ResearchSignal;
  entryRules: string[];
  exitRules: string[];
  stopLossRules: string[];
  takeProfitRules: string[];
  requiredIndicators: string[];
  researchConfidenceScore: number;
  sourceDocument: string;
  side: "LONG" | "SHORT" | "BOTH";
}

export const ALL_RESEARCH_FAMILIES: ResearchFamily[] = [];
export const RESEARCH_FAMILY_LABELS: Record<ResearchFamily, string> = {};
export const RESEARCH_STRATEGIES: ResearchStrategy[] = [];

export const RESEARCH_STRATEGY_BY_ID: ReadonlyMap<number, ResearchStrategy> = new Map();
export const RESEARCH_FAMILIES_WITH_STRATEGIES: ResearchFamily[] = [];
