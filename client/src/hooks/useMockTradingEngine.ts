"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  applyPriceTickToTrade,
  buildMockTradeFromTrace,
  closeMockTrade,
  computeAccountState,
  computeAnalytics,
  DEFAULT_MOCK_TRADING_CONFIG,
  isStrategySignalRaised,
  isValidMockConfig,
  isValidMockTrade,
  logForMockTradeCreated,
  logForMockTradeClosed,
  MOCK_PERSIST_VERSION,
  type MockAccountState,
  type MockTrade,
  type MockTradeAnalytics,
  type MockTradeLog,
  type MockTradingConfig,
  type StrategyExitOverride,
} from "@/lib/mockTradingEngine";
import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import type { StrategySignalTraceRow } from "@/lib/strategySignalTrace";

const TRACE_POLL_MS = 5_000;
const LOG_RING_CAP = 200;
const TRADE_RING_CAP = 1_000;
const STORAGE_KEY = "mock_trading_v2";

const STRATEGY_EXIT_OVERRIDES = new Map<number, StrategyExitOverride>(
  FUTURES_STRAT_DEFS.map((d) => [
    d.id,
    {
      strategyId: d.id,
      takeProfitPct: d.tpPct,
      stopLossPct: d.slPct,
      maxHoldMinutes: d.holdMinutes,
    },
  ]),
);

interface PersistShape {
  version: number;
  trades: MockTrade[];
  config: MockTradingConfig;
}

function loadFromStorage(): { trades: MockTrade[]; config: MockTradingConfig } | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<PersistShape>;
    if (parsed.version !== MOCK_PERSIST_VERSION) return null;
    const cfg = isValidMockConfig(parsed.config) ? parsed.config : DEFAULT_MOCK_TRADING_CONFIG;
    const rawTrades = Array.isArray(parsed.trades) ? parsed.trades : [];
    const trades: MockTrade[] = [];
    for (const t of rawTrades) {
      if (isValidMockTrade(t)) trades.push(t);
    }
    return { trades, config: cfg };
  } catch {
    return null;
  }
}

function saveToStorage(state: { trades: MockTrade[]; config: MockTradingConfig }): void {
  if (typeof window === "undefined") return;
  try {
    const payload: PersistShape = { version: MOCK_PERSIST_VERSION, ...state };
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
  } catch {
    // quota / serialization errors are non-fatal for analysis tooling
  }
}

export interface UseMockTradingEngineOptions {
  /** Live BTC price (USD). 0 means not yet connected. */
  price: number;
  /** Optional account key to forward to the signal-trace API. */
  accountKey?: string | null;
  /** Disable the network poll (used by tests). */
  disablePolling?: boolean;
}

export interface UseMockTradingEngineResult {
  trades: MockTrade[];
  analytics: MockTradeAnalytics;
  account: MockAccountState;
  logs: MockTradeLog[];
  config: MockTradingConfig;
  setConfig: (next: MockTradingConfig) => void;
  /** Manually ingest trace rows — used by tests; production calls happen via the poll. */
  ingestTraceRows: (rows: StrategySignalTraceRow[], priceOverride?: number) => void;
  /** Manually close an open trade. */
  closeTrade: (tradeId: string) => void;
  /** Clear every trade and log (for analysis resets). */
  reset: () => void;
  loading: boolean;
  error: string | null;
  traceAgeSeconds: number | null;
}

export function useMockTradingEngine(
  opts: UseMockTradingEngineOptions,
): UseMockTradingEngineResult {
  const { price, accountKey, disablePolling } = opts;

  const initial = useRef(loadFromStorage());
  const [trades, setTrades] = useState<MockTrade[]>(initial.current?.trades ?? []);
  const [config, setConfigState] = useState<MockTradingConfig>(
    initial.current?.config ?? DEFAULT_MOCK_TRADING_CONFIG,
  );
  const [logs, setLogs] = useState<MockTradeLog[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastFetchAt, setLastFetchAt] = useState<number | null>(null);
  const [, setTickRefresh] = useState(0);

  const seenTraceIdsRef = useRef<Set<string>>(
    new Set((initial.current?.trades ?? []).map((t) => t.traceId)),
  );
  const configRef = useRef(config);
  const priceRef = useRef(price);
  const tradesRef = useRef(trades);

  useEffect(() => { configRef.current = config; }, [config]);
  useEffect(() => { priceRef.current = price; }, [price]);
  useEffect(() => { tradesRef.current = trades; }, [trades]);

  // ── Persist trades + config ────────────────────────────────────────────────
  useEffect(() => {
    saveToStorage({ trades, config });
  }, [trades, config]);

  // ── Ingest trace rows → create mock trades ────────────────────────────────
  const ingestTraceRows = useCallback(
    (rows: StrategySignalTraceRow[], priceOverride?: number) => {
      const livePrice = priceOverride ?? priceRef.current;
      if (!Number.isFinite(livePrice) || livePrice <= 0) return;

      const newTrades: MockTrade[] = [];
      const newLogs: MockTradeLog[] = [];
      const now = Date.now();
      // Equity for percent-of-equity sizing is based on current state.
      const equity = computeAccountState(tradesRef.current, configRef.current).equity;
      for (const row of rows) {
        if (!isStrategySignalRaised(row)) continue;
        if (seenTraceIdsRef.current.has(row.traceId)) continue;
        const override = STRATEGY_EXIT_OVERRIDES.get(row.strategyId);
        const trade = buildMockTradeFromTrace({
          row,
          currentPrice: livePrice,
          config: configRef.current,
          now,
          equity,
          override,
        });
        if (!trade) continue;
        seenTraceIdsRef.current.add(row.traceId);
        newTrades.push(trade);
        newLogs.push(logForMockTradeCreated(trade));
        console.info(
          "[MOCK_TRADE_CREATED]",
          `strategy=${trade.strategyName}#${trade.strategyId}`,
          `side=${trade.side}`,
          `price=${trade.entryPrice.toFixed(2)}`,
          `notional=$${trade.notional.toFixed(0)}`,
          `margin=$${trade.marginUsed.toFixed(0)}`,
          `ignoredBlockers=[${trade.blockers.map((b) => b.gate).join(",")}]`,
        );
      }
      if (newTrades.length === 0) return;
      setTrades((prev) => {
        const combined = [...prev, ...newTrades];
        return combined.length > TRADE_RING_CAP
          ? combined.slice(combined.length - TRADE_RING_CAP)
          : combined;
      });
      setLogs((prev) => {
        const combined = [...newLogs, ...prev];
        return combined.length > LOG_RING_CAP ? combined.slice(0, LOG_RING_CAP) : combined;
      });
    },
    [],
  );

  // ── Poll the signal-trace API ─────────────────────────────────────────────
  useEffect(() => {
    if (disablePolling) return;
    let cancelled = false;
    const fetchOnce = async () => {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams();
        if (accountKey) params.set("account_key", accountKey);
        params.set("limit", "500");
        const res = await fetch(`/api/strategy-signal-trace?${params.toString()}`);
        const json = (await res.json()) as { rows?: StrategySignalTraceRow[]; error?: string };
        if (cancelled) return;
        if (!res.ok || !json.rows) {
          setError(json.error ?? `HTTP ${res.status}`);
          return;
        }
        ingestTraceRows(json.rows);
        setLastFetchAt(Date.now());
      } catch (err) {
        if (cancelled) return;
        setError((err as Error).message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void fetchOnce();
    const id = setInterval(() => void fetchOnce(), TRACE_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [accountKey, disablePolling, ingestTraceRows]);

  // ── Apply price ticks → unrealized PnL and TP/SL ──────────────────────────
  useEffect(() => {
    if (!Number.isFinite(price) || price <= 0) return;
    const now = Date.now();
    let mutated = false;
    const closedLogs: MockTradeLog[] = [];
    const next = tradesRef.current.map((t) => {
      const before = t.status;
      const override = STRATEGY_EXIT_OVERRIDES.get(t.strategyId);
      const updated = applyPriceTickToTrade({
        trade: t,
        price,
        config: configRef.current,
        override,
        now,
      });
      if (updated !== t) mutated = true;
      if (before === "OPEN" && updated.status === "CLOSED") {
        closedLogs.push(logForMockTradeClosed(updated));
        console.info(
          "[MOCK_TRADE_CLOSED]",
          `strategy=${updated.strategyName}#${updated.strategyId}`,
          `side=${updated.side}`,
          `exit=${updated.exitPrice?.toFixed(2)}`,
          `reason=${updated.exitReason}`,
          `pnl=${updated.realizedPnl.toFixed(2)}`,
          `fees=${updated.fees.toFixed(2)}`,
        );
      }
      return updated;
    });
    if (mutated) setTrades(next);
    if (closedLogs.length > 0) {
      setLogs((prev) => {
        const combined = [...closedLogs, ...prev];
        return combined.length > LOG_RING_CAP ? combined.slice(0, LOG_RING_CAP) : combined;
      });
    }
  }, [price]);

  // ── Tick the trace-age counter once per second ────────────────────────────
  useEffect(() => {
    const id = setInterval(() => setTickRefresh((n) => (n + 1) % 1_000), 1_000);
    return () => clearInterval(id);
  }, []);

  // ── Manual controls ───────────────────────────────────────────────────────
  const setConfig = useCallback((next: MockTradingConfig) => setConfigState(next), []);

  const closeTrade = useCallback((tradeId: string) => {
    const now = Date.now();
    const current = priceRef.current;
    if (!Number.isFinite(current) || current <= 0) return;
    let closed: MockTrade | null = null;
    setTrades((prev) =>
      prev.map((t) => {
        if (t.id !== tradeId || t.status !== "OPEN") return t;
        const next = closeMockTrade(t, current, now, configRef.current);
        closed = next;
        return next;
      }),
    );
    if (closed) {
      const log = logForMockTradeClosed(closed);
      setLogs((prev) => {
        const combined = [log, ...prev];
        return combined.length > LOG_RING_CAP ? combined.slice(0, LOG_RING_CAP) : combined;
      });
    }
  }, []);

  const reset = useCallback(() => {
    setTrades([]);
    setLogs([]);
    seenTraceIdsRef.current = new Set();
  }, []);

  const analytics = useMemo(() => computeAnalytics(trades), [trades]);
  const account = useMemo(() => computeAccountState(trades, config), [trades, config]);
  const traceAgeSeconds = lastFetchAt != null ? Math.floor((Date.now() - lastFetchAt) / 1000) : null;

  return {
    trades,
    analytics,
    account,
    logs,
    config,
    setConfig,
    ingestTraceRows,
    closeTrade,
    reset,
    loading,
    error,
    traceAgeSeconds,
  };
}
