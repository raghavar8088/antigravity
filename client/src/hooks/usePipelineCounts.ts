"use client";

import type { StrategyStatus } from "@/lib/strategyAuthority/types";

export type PipelineCountsState = {
  loading: boolean;
  hasAuthority: boolean;
  byStatus: Partial<Record<StrategyStatus, number>>;
};

const EMPTY: PipelineCountsState = {
  loading: false,
  hasAuthority: true,
  byStatus: {},
};

export function usePipelineCounts(): PipelineCountsState {
  return EMPTY;
}

export function formatNavCount(
  counts: PipelineCountsState,
  status: StrategyStatus | undefined
): string | undefined {
  if (!status) return undefined;
  if (counts.loading || !counts.hasAuthority) return undefined;
  const n = counts.byStatus[status];
  return n != null && n > 0 ? String(n) : undefined;
}
