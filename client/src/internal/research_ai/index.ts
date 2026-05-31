import type { RegimeType } from "@/internal/regime";

export interface AIDecisionCacheKey {
  regime: RegimeType;
  adx: number;
  rsi: number;
  vwapDistance: number;
  emaAlignment: "BULLISH" | "BEARISH" | "MIXED";
}

export interface CachedResearchDecision {
  decision: "APPROVE" | "REJECT" | "REVIEW";
  confidence: number;
  reasoning: string;
  createdAt: number;
}

export interface ResearchTradeReview {
  tradeId: string;
  strategyId: number;
  pnl: number;
  notes: string[];
}

const DEFAULT_TTL_MS = 5 * 60 * 1000;

function bucket(n: number, step: number): number {
  if (!Number.isFinite(n)) return 0;
  return Math.round(n / step) * step;
}

function keyOf(key: AIDecisionCacheKey): string {
  return [
    key.regime,
    bucket(key.adx, 2),
    bucket(key.rsi, 5),
    bucket(key.vwapDistance, 0.1),
    key.emaAlignment,
  ].join("|");
}

export class AIDecisionCache {
  private readonly rows = new Map<string, CachedResearchDecision>();

  constructor(private readonly ttlMs = DEFAULT_TTL_MS) {}

  get(key: AIDecisionCacheKey, now = Date.now()): CachedResearchDecision | null {
    const row = this.rows.get(keyOf(key));
    if (!row) return null;
    if (now - row.createdAt > this.ttlMs) {
      this.rows.delete(keyOf(key));
      return null;
    }
    return row;
  }

  set(key: AIDecisionCacheKey, decision: Omit<CachedResearchDecision, "createdAt">, now = Date.now()): CachedResearchDecision {
    const row = { ...decision, createdAt: now };
    this.rows.set(keyOf(key), row);
    return row;
  }
}

export class ResearchAIService {
  constructor(private readonly cache = new AIDecisionCache()) {}

  getDecisionCache(): AIDecisionCache {
    return this.cache;
  }

  reviewExecutedTrade(input: {
    tradeId: string;
    strategyId: number;
    pnl: number;
    maxAdverseExcursion?: number;
    maxFavorableExcursion?: number;
  }): ResearchTradeReview {
    const notes: string[] = [];
    if (input.pnl < 0) notes.push("loss requires post-trade review");
    if ((input.maxAdverseExcursion ?? 0) < -2 * Math.abs(input.pnl)) notes.push("large adverse excursion");
    if ((input.maxFavorableExcursion ?? 0) > Math.abs(input.pnl) * 2) notes.push("profit giveback observed");
    if (notes.length === 0) notes.push("trade behavior within expected bounds");
    return { tradeId: input.tradeId, strategyId: input.strategyId, pnl: input.pnl, notes };
  }
}
