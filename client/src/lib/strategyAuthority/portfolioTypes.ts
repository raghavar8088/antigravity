/**
 * Portfolio-level allocation types used by the Strategy Authority system.
 */

export interface PortfolioStrategyEntry {
  strategy_name: string;
  strategy_id: number;
  allocation_weight: number;
  family?: string;
}

export interface PortfolioAllocationSummary {
  strategies: PortfolioStrategyEntry[];
  total_allocated_weight: number;
  computed_at?: string;
}
