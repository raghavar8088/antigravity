/**
 * usePaperOMSEngine — React hook that delegates paper-trade execution to the
 * Go engine's canonical OMS (Epic 1: execution determinism).
 *
 * The hook:
 *  1. Polls GET /api/engine/paper/state every `pollIntervalMs` ms (default 5 s).
 *  2. Exposes typed helpers to open positions, close positions, and feed ticks.
 *  3. Returns the live snapshot so components can render equity / positions / trades.
 *
 * All engine calls are routed through the Next.js proxy at /api/engine/[...path]
 * which resolves to either a direct engine URL (dev) or INTERNAL_API_URL (prod).
 */
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface OMSPosition {
  id: string;
  symbol: string;
  side: "LONG" | "SHORT";
  entryPrice: number;
  notional: number;
  contracts: number;
  marginUsed: number;
  leverage: number;
  initialSL: number;
  currentSL: number;
  tpPrice: number;
  liqPrice: number;
  holdMinutes: number;
  openedAt: string; // ISO8601
  strategyId: number;
  strategyName: string;
  templateFamily: string;
  moduleKey: string;
  breakevenMoved: boolean;
  peakReturnPct: number;
}

export interface OMSClosedTrade {
  id: string;
  symbol: string;
  side: "LONG" | "SHORT";
  entryPrice: number;
  exitPrice: number;
  notional: number;
  contracts: number;
  grossPnl: number;
  netPnl: number;
  netPct: number;
  fees: number;
  exitReason: string;
  strategyId: number;
  strategyName: string;
  templateFamily: string;
  moduleKey: string;
  openedAt: string;
  closedAt: string;
  holdMinutes: number;
}

export interface OMSStateSnapshot {
  symbol: string;
  balanceUsd: number;
  equityUsd: number;
  totalFeesUsd: number;
  openPositions: OMSPosition[];
  recentTrades: OMSClosedTrade[];
  openCount: number;
  totalTradeCount: number;
  markPrice: number;
}

export interface OpenPositionParams {
  symbol?: string;
  side: "LONG" | "SHORT";
  entryPrice: number;
  notional: number;
  leverage: number;
  slPct: number;
  tpPct: number;
  holdMinutes: number;
  strategyId?: number;
  strategyName?: string;
  templateFamily?: string;
  moduleKey?: string;
  slippageBps?: number;
}

export interface TickParams {
  symbol?: string;
  markPrice: number;
  nowUTC?: string;
}

// ── Hook ──────────────────────────────────────────────────────────────────────

const ENGINE_PAPER_BASE = "/api/engine/paper";

async function engineFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const res = await fetch(`${ENGINE_PAPER_BASE}${path}`, {
    ...init,
    cache: "no-store",
  });
  if (!res.ok) {
    let msg = `engine ${init?.method ?? "GET"} ${path} → ${res.status}`;
    try {
      const body = await res.json();
      if (body?.error) msg += `: ${body.error}`;
    } catch { /* ignore */ }
    throw new Error(msg);
  }
  return res.json() as Promise<T>;
}

export interface UsePaperOMSEngineOptions {
  /** Default symbol forwarded when callers omit it. */
  symbol?: string;
  /** State poll interval in ms. Set to 0 to disable polling. */
  pollIntervalMs?: number;
  /** Called when a poll or action fails. */
  onError?: (err: Error) => void;
}

export interface UsePaperOMSEngineResult {
  snapshot: OMSStateSnapshot | null;
  loading: boolean;
  error: Error | null;
  /** Open a new position; returns the assigned position id. */
  openPosition: (params: OpenPositionParams) => Promise<string>;
  /** Manually close an open position. */
  closePosition: (id: string) => Promise<OMSClosedTrade | null>;
  /** Feed a mark-price tick; returns any positions that were closed. */
  tick: (params: TickParams) => Promise<OMSClosedTrade[]>;
  /** Hard-reset all state and restore initial balance. */
  reset: () => Promise<void>;
  /** Trigger an immediate poll (useful after a mutation). */
  refresh: () => void;
}

export function usePaperOMSEngine({
  symbol = "BTCUSDT",
  pollIntervalMs = 5_000,
  onError,
}: UsePaperOMSEngineOptions = {}): UsePaperOMSEngineResult {
  const [snapshot, setSnapshot] = useState<OMSStateSnapshot | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const refreshRef = useRef(0);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);

  const fetchState = useCallback(async () => {
    try {
      setLoading(true);
      const snap = await engineFetch<OMSStateSnapshot>(
        `/state?symbol=${encodeURIComponent(symbol)}`,
      );
      if (mountedRef.current) {
        setSnapshot(snap);
        setError(null);
      }
    } catch (err) {
      const e = err instanceof Error ? err : new Error(String(err));
      if (mountedRef.current) setError(e);
      onError?.(e);
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, [symbol, onError]);

  // Initial fetch + periodic polling
  useEffect(() => {
    fetchState();
    if (pollIntervalMs <= 0) return;
    const id = setInterval(fetchState, pollIntervalMs);
    return () => clearInterval(id);
  }, [fetchState, pollIntervalMs, refreshRef.current]); // eslint-disable-line react-hooks/exhaustive-deps

  const refresh = useCallback(() => {
    refreshRef.current += 1;
  }, []);

  const openPosition = useCallback(
    async (params: OpenPositionParams): Promise<string> => {
      const body: Record<string, unknown> = {
        symbol: params.symbol ?? symbol,
        side: params.side,
        entryPrice: params.entryPrice,
        notional: params.notional,
        leverage: params.leverage,
        slPct: params.slPct,
        tpPct: params.tpPct,
        holdMinutes: params.holdMinutes,
      };
      if (params.strategyId !== undefined) body.strategyId = params.strategyId;
      if (params.strategyName) body.strategyName = params.strategyName;
      if (params.templateFamily) body.templateFamily = params.templateFamily;
      if (params.moduleKey) body.moduleKey = params.moduleKey;
      if (params.slippageBps !== undefined) body.slippageBps = params.slippageBps;

      const res = await engineFetch<{ ok: string; id: string }>("/open", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      refresh();
      return res.id;
    },
    [symbol, refresh],
  );

  const closePosition = useCallback(
    async (id: string): Promise<OMSClosedTrade | null> => {
      const res = await engineFetch<{ ok: boolean; trade: OMSClosedTrade }>(
        `/close/${encodeURIComponent(id)}`,
        { method: "POST" },
      );
      refresh();
      return res.trade ?? null;
    },
    [refresh],
  );

  const tick = useCallback(
    async (params: TickParams): Promise<OMSClosedTrade[]> => {
      const body: Record<string, unknown> = {
        symbol: params.symbol ?? symbol,
        markPrice: params.markPrice,
      };
      if (params.nowUTC) body.nowUTC = params.nowUTC;

      const res = await engineFetch<{ ok: boolean; closed: OMSClosedTrade[]; count: number }>(
        "/tick",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        },
      );
      if ((res.count ?? 0) > 0) refresh();
      return res.closed ?? [];
    },
    [symbol, refresh],
  );

  const reset = useCallback(async (): Promise<void> => {
    await engineFetch<{ ok: string }>("/reset", { method: "POST" });
    refresh();
  }, [refresh]);

  return { snapshot, loading, error, openPosition, closePosition, tick, reset, refresh };
}
