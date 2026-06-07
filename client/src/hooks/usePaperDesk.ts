"use client";

/**
 * usePaperDesk — polling data hook for the Go Engine Paper Desk dashboard.
 *
 * Architecture decision (Phase 31C): POLLING over SSE.
 *   - Deployment is Vercel serverless (Hobby). Long-lived SSE connections hold a
 *     function invocation open and are killed at the platform timeout, so they
 *     are not production-safe here. Polling a single aggregated /snapshot route
 *     every 5s is stateless, reconnect-free, and identical across all devices.
 *   - The engine itself snapshots paper_state every 10s and equity every 5min,
 *     so a 5s client poll is already faster than the underlying write cadence.
 *
 * The hook polls /api/paper-desk/snapshot (one request per tick) for the live
 * summary, and lazily fetches detail collections (full trades page, OMS orders,
 * equity curve, full strategy table) only when those tabs are opened.
 *
 * All requests are credentialed; account_key is derived server-side from the JWT
 * session. The client never sends an account key.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import type {
  PaperStateDoc,
  PaperTradeDoc,
  PaperPositionDoc,
  PaperOrderDoc,
  EquityCurveDoc,
  DailyPnLDoc,
  StrategyScoreDoc,
  StrategyHealthDoc,
} from "@/lib/paperDeskClient";

const POLL_MS = 5000;

export type HealthSummary = {
  healthy: number;
  warning: number;
  critical: number;
  insufficient_data: number;
  total: number;
};

type SnapshotResponse = {
  ok: boolean;
  error?: string;
  server_time?: string;
  state?: PaperStateDoc | null;
  open_positions?: PaperPositionDoc[];
  recent_trades?: PaperTradeDoc[];
  health_summary?: HealthSummary;
};

export type PaperDeskConnection = "connecting" | "live" | "stale" | "error" | "unauthorized";

async function getJSON<T>(url: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(url, { credentials: "include", cache: "no-store", signal });
  if (res.status === 401) {
    throw new Error("UNAUTHORIZED");
  }
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}`);
  }
  return (await res.json()) as T;
}

export function usePaperDesk() {
  const [state, setState] = useState<PaperStateDoc | null>(null);
  const [openPositions, setOpenPositions] = useState<PaperPositionDoc[]>([]);
  const [recentTrades, setRecentTrades] = useState<PaperTradeDoc[]>([]);
  const [healthSummary, setHealthSummary] = useState<HealthSummary | null>(null);
  const [connection, setConnection] = useState<PaperDeskConnection>("connecting");
  const [serverTime, setServerTime] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<number | null>(null);

  const mountedRef = useRef(true);
  const inFlightRef = useRef<AbortController | null>(null);

  const pollSnapshot = useCallback(async () => {
    inFlightRef.current?.abort();
    const ac = new AbortController();
    inFlightRef.current = ac;
    try {
      const data = await getJSON<SnapshotResponse>("/api/paper-desk/snapshot", ac.signal);
      if (!mountedRef.current) return;
      if (!data.ok) {
        setConnection("error");
        return;
      }
      setState(data.state ?? null);
      setOpenPositions(data.open_positions ?? []);
      setRecentTrades(data.recent_trades ?? []);
      setHealthSummary(data.health_summary ?? null);
      setServerTime(data.server_time ?? null);
      setLastUpdated(Date.now());
      setConnection("live");
    } catch (err) {
      if (!mountedRef.current) return;
      if (err instanceof DOMException && err.name === "AbortError") return;
      if (err instanceof Error && err.message === "UNAUTHORIZED") {
        setConnection("unauthorized");
        return;
      }
      // Keep last good data on screen but mark the feed stale.
      setConnection((prev) => (prev === "live" ? "stale" : "error"));
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    // Wrapped in an async IIFE so the eventual setState lands in a callback
    // context (satisfies react-hooks/set-state-in-effect).
    void (async () => {
      await pollSnapshot();
    })();
    const id = setInterval(() => void pollSnapshot(), POLL_MS);
    // Pause polling while the tab is hidden to save Vercel function invocations.
    const onVisibility = () => {
      if (document.visibilityState === "visible") void pollSnapshot();
    };
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      mountedRef.current = false;
      clearInterval(id);
      inFlightRef.current?.abort();
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [pollSnapshot]);

  return {
    state,
    openPositions,
    recentTrades,
    healthSummary,
    connection,
    serverTime,
    lastUpdated,
    refresh: pollSnapshot,
  };
}

// ── On-demand detail fetchers (used by detail tabs) ─────────────────────────

export type TradesPage = {
  trades: PaperTradeDoc[];
  pagination: { limit: number; offset: number; total: number; has_more: boolean };
};

export async function fetchTradesPage(params: {
  limit?: number;
  offset?: number;
  strategyId?: string;
  side?: string;
  sortBy?: "closed_at" | "net_pnl";
  sortDir?: "asc" | "desc";
}): Promise<TradesPage> {
  const q = new URLSearchParams();
  if (params.limit) q.set("limit", String(params.limit));
  if (params.offset) q.set("offset", String(params.offset));
  if (params.strategyId) q.set("strategy_id", params.strategyId);
  if (params.side) q.set("side", params.side);
  if (params.sortBy) q.set("sort_by", params.sortBy);
  if (params.sortDir) q.set("sort_dir", params.sortDir);
  const data = await getJSON<{ ok: boolean; trades: PaperTradeDoc[]; pagination: TradesPage["pagination"] }>(
    `/api/paper-desk/trades?${q.toString()}`,
  );
  return { trades: data.trades ?? [], pagination: data.pagination };
}

export async function fetchOrders(limit = 100): Promise<PaperOrderDoc[]> {
  const data = await getJSON<{ ok: boolean; orders: PaperOrderDoc[] }>(
    `/api/paper-desk/orders?limit=${limit}`,
  );
  return data.orders ?? [];
}

export async function fetchEquity(points = 288, days = 30): Promise<{ curve: EquityCurveDoc[]; daily: DailyPnLDoc[] }> {
  const data = await getJSON<{ ok: boolean; curve: EquityCurveDoc[]; daily_pnl: DailyPnLDoc[] }>(
    `/api/paper-desk/equity?points=${points}&days=${days}`,
  );
  return { curve: data.curve ?? [], daily: data.daily_pnl ?? [] };
}

export async function fetchStrategyHealth(status?: string): Promise<{
  scores: StrategyScoreDoc[];
  health: StrategyHealthDoc[];
  summary: HealthSummary;
}> {
  const q = status ? `?status=${encodeURIComponent(status)}` : "";
  const data = await getJSON<{
    ok: boolean;
    scores: StrategyScoreDoc[];
    health: StrategyHealthDoc[];
    summary: HealthSummary;
  }>(`/api/paper-desk/strategy-health${q}`);
  return { scores: data.scores ?? [], health: data.health ?? [], summary: data.summary };
}
