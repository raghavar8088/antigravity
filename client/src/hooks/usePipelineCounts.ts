"use client";

import { useEffect, useState } from "react";
import type { StrategyStatus } from "@/lib/strategyAuthority/types";

export type PipelineCountsState = {
  loading: boolean;
  hasAuthority: boolean;
  byStatus: Partial<Record<StrategyStatus, number>>;
};

const EMPTY: PipelineCountsState = {
  loading: true,
  hasAuthority: false,
  byStatus: {},
};

export function usePipelineCounts(): PipelineCountsState {
  const [state, setState] = useState<PipelineCountsState>(EMPTY);

  useEffect(() => {
    let cancelled = false;

    fetch("/api/strategy-authority/counts")
      .then((r) => r.json())
      .then((d) => {
        if (cancelled) return;
        if (d.ok && d.counts?.byStatus) {
          setState({
            loading: false,
            hasAuthority: true,
            byStatus: d.counts.byStatus,
          });
        } else {
          setState({ loading: false, hasAuthority: false, byStatus: {} });
        }
      })
      .catch(() => {
        if (!cancelled) setState({ loading: false, hasAuthority: false, byStatus: {} });
      });

    return () => {
      cancelled = true;
    };
  }, []);

  return state;
}

export function formatNavCount(
  counts: PipelineCountsState,
  status: StrategyStatus | undefined
): string | undefined {
  if (!status) return undefined;
  if (counts.loading) return undefined;
  if (!counts.hasAuthority) return "—";
  const n = counts.byStatus[status];
  return n != null ? String(n) : "0";
}
