"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  applyPriceTickToTrade,
  buildMockTradeFromTrace,
  buildMockTradeFromResearchSignal,
  closeMockTrade,
  computeAccountState,
  computeAnalytics,
  countOpenMockTrades,
  DEFAULT_MAX_OPEN_MOCK_TRADES,
  DEFAULT_MOCK_TRADING_CONFIG,
  isStrategySignalRaised,
  isValidMockTrade,
  logForMockTradeLimitReached,
  logForMockTradeCreated,
  logForMockTradeClosed,
  maxOpenMockTradesFromConfig,
  MOCK_PERSIST_VERSION,
  normalizeMockTradingConfig,
  type MockAccountState,
  type MockTrade,
  type MockTradeAnalytics,
  type MockTradeLog,
  type MockTradingConfig,
  type MockResearchSignalInput,
  type StrategyExitOverride,
} from "@/lib/mockTradingEngine";
import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import {
  DEFAULT_MOCK_ACCOUNT_KEY,
  MOCK_RESET_CONFIRMATION,
  mergeHydratedMockTrades,
  type MockAccountSnapshotResponse,
  type MockLogsResponse,
  type MockTradingHydrateResponse,
} from "@/lib/mockTradingPersistenceTypes";
import type { StrategySignalTraceRow } from "@/lib/strategySignalTrace";

const TRACE_POLL_MS = 5_000;
const MARK_PERSIST_THROTTLE_MS = 15_000;
const ACCOUNT_SNAPSHOT_THROTTLE_MS = 15_000;
const OPEN_MARK_PERSIST_BATCH_SIZE = 100;
const LOG_RING_CAP = 200;
const TRADE_CACHE_MIN_CAP = DEFAULT_MAX_OPEN_MOCK_TRADES;
const HISTORY_PAGE_SIZE = 100;
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
    const cfg = normalizeMockTradingConfig(parsed.config);
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

function trimTradeCache(trades: MockTrade[], config: MockTradingConfig): MockTrade[] {
  const cap = Math.max(TRADE_CACHE_MIN_CAP, maxOpenMockTradesFromConfig(config));
  if (trades.length <= cap) return trades;
  const open = trades.filter((trade) => trade.status === "OPEN");
  const closed = trades.filter((trade) => trade.status === "CLOSED");
  const closedKeep = Math.max(0, cap - open.length);
  const retainedClosed = closedKeep > 0 ? closed.slice(-closedKeep) : [];
  return [...retainedClosed, ...open].sort((a, b) => a.openedAt - b.openedAt);
}

export interface UseMockTradingEngineOptions {
  /** Live BTC price (USD). 0 means not yet connected. */
  price: number;
  /** Optional account key to forward to the signal-trace API. */
  accountKey?: string | null;
  /** Disable the network poll (used by tests). */
  disablePolling?: boolean;
  /** Disable Mongo persistence/hydration (used by tests). */
  disablePersistence?: boolean;
}

export interface UseMockTradingEngineResult {
  trades: MockTrade[];
  historyTrades: MockTrade[];
  analytics: MockTradeAnalytics;
  account: MockAccountState;
  logs: MockTradeLog[];
  config: MockTradingConfig;
  setConfig: (next: MockTradingConfig) => void;
  /** Manually ingest trace rows — used by tests; production calls happen via the poll. */
  ingestTraceRows: (rows: StrategySignalTraceRow[], priceOverride?: number) => void;
  /** Ingest mock-only research-pack signals. Never routes to broker/order APIs. */
  ingestResearchSignals: (signals: MockResearchSignalInput[], priceOverride?: number) => void;
  /** Manually close an open trade. */
  closeTrade: (tradeId: string) => void;
  /** Clear every trade and log (for analysis resets). */
  reset: () => void;
  loading: boolean;
  error: string | null;
  traceAgeSeconds: number | null;
  mockLimitRejectedSignals: number;
  persistence: {
    status: "hydrating" | "mongo" | "fallback" | "error";
    loading: boolean;
    error: string | null;
    lastHydratedAt: number | null;
    lastSavedAt: number | null;
  };
  history: {
    page: number;
    limit: number;
    total: number;
    totalPages: number;
    loading: boolean;
    error: string | null;
    setPage: (page: number) => void;
  };
}

export function useMockTradingEngine(
  opts: UseMockTradingEngineOptions,
): UseMockTradingEngineResult {
  const { price, accountKey, disablePolling, disablePersistence } = opts;
  const persistenceDisabled = disablePersistence === true || disablePolling === true;
  const mockAccountKey = accountKey?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;

  const initial = useRef(loadFromStorage());
  const [trades, setTrades] = useState<MockTrade[]>(initial.current?.trades ?? []);
  const [historyTrades, setHistoryTrades] = useState<MockTrade[]>(initial.current?.trades ?? []);
  const [config, setConfigState] = useState<MockTradingConfig>(
    initial.current?.config ?? DEFAULT_MOCK_TRADING_CONFIG,
  );
  const [logs, setLogs] = useState<MockTradeLog[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastFetchAt, setLastFetchAt] = useState<number | null>(null);
  const [persistence, setPersistence] = useState<UseMockTradingEngineResult["persistence"]>({
    status: persistenceDisabled ? "fallback" : "hydrating",
    loading: !persistenceDisabled,
    error: null,
    lastHydratedAt: null,
    lastSavedAt: null,
  });
  const [historyPage, setHistoryPage] = useState(1);
  const [historyMeta, setHistoryMeta] = useState({
    total: initial.current?.trades.length ?? 0,
    totalPages: 1,
    loading: false,
    error: null as string | null,
  });
  const [mockLimitRejectedSignals, setMockLimitRejectedSignals] = useState(0);
  const [, setTickRefresh] = useState(0);

  const seenTraceIdsRef = useRef<Set<string>>(
    new Set((initial.current?.trades ?? []).map((t) => t.traceId)),
  );
  const configRef = useRef(config);
  const priceRef = useRef(price);
  const tradesRef = useRef(trades);
  const lastMarkPersistAtRef = useRef(0);
  const lastAccountSnapshotAtRef = useRef(0);
  const lastPersistedConfigRef = useRef<string | null>(null);
  const openMarkPersistCursorRef = useRef(0);

  useEffect(() => { configRef.current = config; }, [config]);
  useEffect(() => { priceRef.current = price; }, [price]);
  useEffect(() => { tradesRef.current = trades; }, [trades]);

  // ── Persist trades + config ────────────────────────────────────────────────
  useEffect(() => {
    saveToStorage({ trades, config });
  }, [trades, config]);

  const markPersistenceOk = useCallback(() => {
    setPersistence((prev) => ({
      ...prev,
      status: "mongo",
      loading: false,
      error: null,
      lastSavedAt: Date.now(),
    }));
  }, []);

  const markPersistenceError = useCallback((message: string, fallback = false) => {
    setPersistence((prev) => ({
      ...prev,
      status: fallback ? "fallback" : "error",
      loading: false,
      error: message,
    }));
  }, []);

  const persistTrade = useCallback(
    async (
      trade: MockTrade,
      mode: "create" | "update" | "close",
    ) => {
      if (persistenceDisabled) return;
      const method = mode === "create" ? "POST" : mode === "update" ? "PATCH" : "POST";
      const url = mode === "create"
        ? "/api/mock-trading/trades"
        : mode === "update"
          ? `/api/mock-trading/trades/${encodeURIComponent(trade.id)}`
          : `/api/mock-trading/trades/${encodeURIComponent(trade.id)}/close`;
      try {
        const res = await fetch(url, {
          method,
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ accountKey: mockAccountKey, trade, config: configRef.current }),
        });
        if (!res.ok) {
          const json = await res.json().catch(() => null) as { error?: string; code?: string } | null;
          markPersistenceError(json?.error ?? `HTTP ${res.status}`, res.status === 503);
          return;
        }
        markPersistenceOk();
      } catch (err) {
        markPersistenceError((err as Error).message);
      }
    },
    [markPersistenceError, markPersistenceOk, mockAccountKey, persistenceDisabled],
  );

  const persistAccountSnapshot = useCallback(
    async (account: MockAccountState) => {
      if (persistenceDisabled) return;
      try {
        const res = await fetch("/api/mock-trading/account", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ accountKey: mockAccountKey, account, config: configRef.current }),
        });
        if (!res.ok) {
          const json = await res.json().catch(() => null) as { error?: string; code?: string } | null;
          markPersistenceError(json?.error ?? `HTTP ${res.status}`, res.status === 503);
          return;
        }
        markPersistenceOk();
      } catch (err) {
        markPersistenceError((err as Error).message);
      }
    },
    [markPersistenceError, markPersistenceOk, mockAccountKey, persistenceDisabled],
  );

  const loadHistoryPage = useCallback(
    async (page: number) => {
      if (persistenceDisabled) return;
      setHistoryMeta((prev) => ({ ...prev, loading: true, error: null }));
      try {
        const params = new URLSearchParams({
          account_key: mockAccountKey,
          page: String(page),
          limit: String(HISTORY_PAGE_SIZE),
          sort: "newest",
        });
        const res = await fetch(`/api/mock-trading/trades?${params.toString()}`);
        const json = await res.json() as MockTradingHydrateResponse | { ok: false; error?: string; code?: string };
        if (!res.ok || !json.ok) {
          const message = "error" in json && json.error ? json.error : `HTTP ${res.status}`;
          setHistoryMeta((prev) => ({ ...prev, loading: false, error: message }));
          markPersistenceError(message, res.status === 503);
          return;
        }
        setHistoryTrades(json.trades);
        setHistoryMeta({
          total: json.total,
          totalPages: json.totalPages,
          loading: false,
          error: null,
        });
        setHistoryPage(json.page);
        setPersistence((prev) => ({ ...prev, status: "mongo", loading: false, error: null }));
      } catch (err) {
        const message = (err as Error).message;
        setHistoryMeta((prev) => ({ ...prev, loading: false, error: message }));
        markPersistenceError(message);
      }
    },
    [markPersistenceError, mockAccountKey, persistenceDisabled],
  );

  // Hydrate durable state from Mongo. localStorage remains a fast cache/fallback.
  useEffect(() => {
    if (persistenceDisabled) return;
    let cancelled = false;
    const hydrate = async () => {
      setPersistence((prev) => ({ ...prev, status: "hydrating", loading: true, error: null }));
      try {
        const openParams = new URLSearchParams({
          account_key: mockAccountKey,
          page: "1",
          limit: String(DEFAULT_MAX_OPEN_MOCK_TRADES),
          status: "OPEN",
          sort: "oldest",
        });
        const historyParams = new URLSearchParams({
          account_key: mockAccountKey,
          page: "1",
          limit: String(HISTORY_PAGE_SIZE),
          sort: "newest",
        });
        const [openTradesRes, historyRes, accountRes, logsRes] = await Promise.all([
          fetch(`/api/mock-trading/trades?${openParams.toString()}`),
          fetch(`/api/mock-trading/trades?${historyParams.toString()}`),
          fetch(`/api/mock-trading/account/latest?account_key=${encodeURIComponent(mockAccountKey)}`),
          fetch(`/api/mock-trading/logs?account_key=${encodeURIComponent(mockAccountKey)}&limit=${LOG_RING_CAP}`),
        ]);
        if (cancelled) return;
        if (openTradesRes.status === 503 || historyRes.status === 503 || accountRes.status === 503 || logsRes.status === 503) {
          markPersistenceError("MongoDB not configured", true);
          return;
        }
        const openTradesJson = await openTradesRes.json() as MockTradingHydrateResponse | { ok: false; error?: string };
        const historyJson = await historyRes.json() as MockTradingHydrateResponse | { ok: false; error?: string };
        const accountJson = await accountRes.json() as MockAccountSnapshotResponse | { ok: false; error?: string };
        const logsJson = await logsRes.json() as MockLogsResponse | { ok: false; error?: string };
        if (!openTradesRes.ok || !openTradesJson.ok) throw new Error("error" in openTradesJson ? openTradesJson.error ?? "Open trade hydrate failed" : "Open trade hydrate failed");
        if (!historyRes.ok || !historyJson.ok) throw new Error("error" in historyJson ? historyJson.error ?? "Trade history hydrate failed" : "Trade history hydrate failed");
        if (!accountRes.ok || !accountJson.ok) throw new Error("error" in accountJson ? accountJson.error ?? "Account hydrate failed" : "Account hydrate failed");
        if (!logsRes.ok || !logsJson.ok) throw new Error("error" in logsJson ? logsJson.error ?? "Log hydrate failed" : "Log hydrate failed");

        const merged = mergeHydratedMockTrades(tradesRef.current, openTradesJson.trades);
        setTrades(merged);
        setHistoryTrades(historyJson.trades);
        setHistoryMeta({
          total: historyJson.total,
          totalPages: Math.max(1, Math.ceil(historyJson.total / HISTORY_PAGE_SIZE)),
          loading: false,
          error: null,
        });
        if (accountJson.config) setConfigState(normalizeMockTradingConfig(accountJson.config));
        setLogs(logsJson.logs.slice(0, LOG_RING_CAP));
        seenTraceIdsRef.current = new Set(merged.map((t) => t.traceId));
        setPersistence({
          status: "mongo",
          loading: false,
          error: null,
          lastHydratedAt: Date.now(),
          lastSavedAt: null,
        });
      } catch (err) {
        if (!cancelled) markPersistenceError((err as Error).message);
      }
    };
    void hydrate();
    return () => {
      cancelled = true;
    };
  }, [markPersistenceError, mockAccountKey, persistenceDisabled]);

  useEffect(() => {
    void loadHistoryPage(historyPage);
  }, [historyPage, loadHistoryPage]);

  // ── Ingest trace rows → create mock trades ────────────────────────────────
  const ingestTraceRows = useCallback(
    (rows: StrategySignalTraceRow[], priceOverride?: number) => {
      const livePrice = priceOverride ?? priceRef.current;
      if (!Number.isFinite(livePrice) || livePrice <= 0) return;

      const newTrades: MockTrade[] = [];
      const newLogs: MockTradeLog[] = [];
      const now = Date.now();
      const limit = maxOpenMockTradesFromConfig(configRef.current);
      const currentOpenCount = countOpenMockTrades(tradesRef.current);
      let limitRejected = 0;
      // Equity for percent-of-equity sizing is based on current state.
      const equity = computeAccountState(tradesRef.current, configRef.current).equity;
      for (const row of rows) {
        if (!isStrategySignalRaised(row)) continue;
        if (seenTraceIdsRef.current.has(row.traceId)) continue;
        if (currentOpenCount + newTrades.length >= limit) {
          seenTraceIdsRef.current.add(row.traceId);
          limitRejected++;
          const side = row.side === "SHORT" ? "SELL" : "BUY";
          const ignoredBlockers = row.status === "OPENED" || row.status === "CANDIDATE" ? [] : [row.gate];
          newLogs.push(logForMockTradeLimitReached({
            ts: now,
            strategyId: row.strategyId,
            strategyName: row.strategyName,
            side,
            price: livePrice,
            openCount: currentOpenCount + newTrades.length,
            maxOpenMockTrades: limit,
            ignoredBlockers,
          }));
          console.info(
            "[MOCK_TRADE_LIMIT_REACHED]",
            `strategy=${row.strategyName}#${row.strategyId}`,
            `side=${side}`,
            `open=${currentOpenCount + newTrades.length}`,
            `limit=${limit}`,
            `ignoredBlocker=${row.gate}`,
          );
          continue;
        }
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
          `id=${trade.id}`,
          `strategy=${trade.strategyName}#${trade.strategyId}`,
          `side=${trade.side}`,
          `price=${trade.entryPrice.toFixed(2)}`,
          `tp=${trade.takeProfitPrice.toFixed(2)}:est+$${trade.takeProfitUsd.toFixed(2)}`,
          `sl=${trade.stopLossPrice.toFixed(2)}:est-$${trade.stopLossUsd.toFixed(2)}`,
          `notional=$${trade.notional.toFixed(0)}`,
          `margin=$${trade.marginUsed.toFixed(0)}`,
          `ignoredBlockers=[${trade.blockers.map((b) => b.gate).join(",")}]`,
        );
      }
      if (limitRejected > 0) setMockLimitRejectedSignals((count) => count + limitRejected);
      if (newLogs.length > 0) {
        setLogs((prev) => {
          const combined = [...newLogs, ...prev];
          return combined.length > LOG_RING_CAP ? combined.slice(0, LOG_RING_CAP) : combined;
        });
      }
      if (newTrades.length === 0) return;
      setTrades((prev) => {
        const combined = [...prev, ...newTrades];
        return trimTradeCache(combined, configRef.current);
      });
      if (historyPage === 1) {
        setHistoryTrades((prev) => [...newTrades, ...prev].slice(0, HISTORY_PAGE_SIZE));
        setHistoryMeta((prev) => ({
          ...prev,
          total: prev.total + newTrades.length,
          totalPages: Math.max(1, Math.ceil((prev.total + newTrades.length) / HISTORY_PAGE_SIZE)),
        }));
      }
      for (const trade of newTrades) void persistTrade(trade, "create");
    },
    [historyPage, persistTrade],
  );

  // ── Ingest 500-strategy research signals → create mock trades only ────────
  const ingestResearchSignals = useCallback(
    (signals: MockResearchSignalInput[], priceOverride?: number) => {
      const livePrice = priceOverride ?? priceRef.current;
      if (!Number.isFinite(livePrice) || livePrice <= 0) return;

      const newTrades: MockTrade[] = [];
      const newLogs: MockTradeLog[] = [];
      const now = Date.now();
      const limit = maxOpenMockTradesFromConfig(configRef.current);
      const currentOpenCount = countOpenMockTrades(tradesRef.current);
      let limitRejected = 0;
      const equity = computeAccountState(tradesRef.current, configRef.current).equity;

      for (const signal of signals) {
        const dedupeKey = `mock-research-${signal.strategyId}-${signal.evaluatedAt}-${signal.side}`;
        if (seenTraceIdsRef.current.has(dedupeKey)) continue;
        const duplicateOpen = [...tradesRef.current, ...newTrades].some(
          (trade) =>
            trade.status === "OPEN" &&
            trade.strategyId === signal.strategyId &&
            trade.side === signal.side,
        );
        if (duplicateOpen) {
          seenTraceIdsRef.current.add(dedupeKey);
          continue;
        }
        if (currentOpenCount + newTrades.length >= limit) {
          seenTraceIdsRef.current.add(dedupeKey);
          limitRejected++;
          newLogs.push(logForMockTradeLimitReached({
            ts: now,
            strategyId: signal.strategyId,
            strategyName: signal.strategyName,
            side: signal.side,
            price: livePrice,
            openCount: currentOpenCount + newTrades.length,
            maxOpenMockTrades: limit,
          }));
          console.info(
            "[MOCK_TRADE_LIMIT_REACHED]",
            `strategy=${signal.strategyName}#${signal.strategyId}`,
            `side=${signal.side}`,
            `open=${currentOpenCount + newTrades.length}`,
            `limit=${limit}`,
            "source=research",
          );
          continue;
        }
        const trade = buildMockTradeFromResearchSignal({
          signal,
          currentPrice: livePrice,
          config: configRef.current,
          now,
          equity,
        });
        if (!trade) continue;
        seenTraceIdsRef.current.add(trade.traceId);
        newTrades.push(trade);
        newLogs.push(logForMockTradeCreated(trade));
        console.info(
          "[MOCK_RESEARCH_TRADE_CREATED]",
          `strategy=${trade.strategyName}#${trade.strategyId}`,
          `family=${trade.strategyFamily ?? "UNKNOWN"}`,
          `side=${trade.side}`,
          `confidence=${trade.confidenceScore?.toFixed(1) ?? "n/a"}`,
          `price=${trade.entryPrice.toFixed(2)}`,
        );
      }

      if (limitRejected > 0) setMockLimitRejectedSignals((count) => count + limitRejected);
      if (newLogs.length > 0) {
        setLogs((prev) => {
          const combined = [...newLogs, ...prev];
          return combined.length > LOG_RING_CAP ? combined.slice(0, LOG_RING_CAP) : combined;
        });
      }
      if (newTrades.length === 0) return;
      setTrades((prev) => {
        const combined = [...prev, ...newTrades];
        return trimTradeCache(combined, configRef.current);
      });
      if (historyPage === 1) {
        setHistoryTrades((prev) => [...newTrades, ...prev].slice(0, HISTORY_PAGE_SIZE));
        setHistoryMeta((prev) => ({
          ...prev,
          total: prev.total + newTrades.length,
          totalPages: Math.max(1, Math.ceil((prev.total + newTrades.length) / HISTORY_PAGE_SIZE)),
        }));
      }
      for (const trade of newTrades) void persistTrade(trade, "create");
    },
    [historyPage, persistTrade],
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
    const closedTrades: MockTrade[] = [];
    const openUpdates: MockTrade[] = [];
    const shouldPersistOpenMarks = now - lastMarkPersistAtRef.current >= MARK_PERSIST_THROTTLE_MS;
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
        closedTrades.push(updated);
        closedLogs.push(logForMockTradeClosed(updated));
        const event = updated.exitReason === "TAKE_PROFIT"
          ? "MOCK_TRADE_TP_HIT"
          : updated.exitReason === "STOP_LOSS"
            ? "MOCK_TRADE_SL_HIT"
            : "MOCK_TRADE_CLOSED";
        console.info(
          `[${event}]`,
          `id=${updated.id}`,
          `strategy=${updated.strategyName}#${updated.strategyId}`,
          `side=${updated.side}`,
          `entry=${updated.entryPrice.toFixed(2)}`,
          `exit=${updated.exitPrice?.toFixed(2)}`,
          `reason=${updated.exitReason}`,
          `pnl=${updated.realizedPnl.toFixed(2)}`,
          `fees=${updated.fees.toFixed(2)}`,
        );
      } else if (updated !== t && updated.status === "OPEN" && shouldPersistOpenMarks) {
        openUpdates.push(updated);
      }
      return updated;
    });
    if (mutated) setTrades(next);
    if (mutated && historyPage === 1) {
      const byId = new Map(next.map((trade) => [trade.id, trade]));
      setHistoryTrades((prev) => prev.map((trade) => byId.get(trade.id) ?? trade));
    }
    if (closedLogs.length > 0) {
      setLogs((prev) => {
        const combined = [...closedLogs, ...prev];
        return combined.length > LOG_RING_CAP ? combined.slice(0, LOG_RING_CAP) : combined;
      });
    }
    for (const trade of closedTrades) void persistTrade(trade, "close");
    if (openUpdates.length > 0) {
      lastMarkPersistAtRef.current = now;
      const start = openMarkPersistCursorRef.current % openUpdates.length;
      const rotated = [...openUpdates.slice(start), ...openUpdates.slice(0, start)];
      const batch = rotated.slice(0, OPEN_MARK_PERSIST_BATCH_SIZE);
      openMarkPersistCursorRef.current = (start + batch.length) % openUpdates.length;
      for (const trade of batch) void persistTrade(trade, "update");
    }
  }, [historyPage, persistTrade, price]);

  // ── Tick the trace-age counter once per second ────────────────────────────
  useEffect(() => {
    const id = setInterval(() => setTickRefresh((n) => (n + 1) % 1_000), 1_000);
    return () => clearInterval(id);
  }, []);

  // ── Manual controls ───────────────────────────────────────────────────────
  const setConfig = useCallback((next: MockTradingConfig) => setConfigState(normalizeMockTradingConfig(next)), []);

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
      setHistoryTrades((prev) => prev.map((trade) => trade.id === closed?.id ? closed : trade));
      void persistTrade(closed, "close");
    }
  }, [persistTrade]);

  const reset = useCallback(() => {
    const confirmed = typeof window === "undefined"
      ? true
      : window.confirm("Reset all persisted Mock Trading trades, logs, analytics, and account snapshots?");
    if (!confirmed) return;
    setTrades([]);
    setHistoryTrades([]);
    setLogs([]);
    setMockLimitRejectedSignals(0);
    setHistoryMeta({ total: 0, totalPages: 1, loading: false, error: null });
    seenTraceIdsRef.current = new Set();
    if (!persistenceDisabled) {
      void fetch("/api/mock-trading/reset", {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ accountKey: mockAccountKey, confirmation: MOCK_RESET_CONFIRMATION }),
      }).then((res) => {
        if (res.ok) markPersistenceOk();
        else markPersistenceError(`Reset failed: HTTP ${res.status}`, res.status === 503);
      }).catch((err: Error) => markPersistenceError(err.message));
    }
  }, [markPersistenceError, markPersistenceOk, mockAccountKey, persistenceDisabled]);

  const analytics = useMemo(() => computeAnalytics(trades), [trades]);
  const account = useMemo(() => computeAccountState(trades, config), [trades, config]);
  const traceAgeSeconds = lastFetchAt != null ? Math.floor((Date.now() - lastFetchAt) / 1000) : null;

  useEffect(() => {
    if (persistenceDisabled) return;
    const now = Date.now();
    const configKey = JSON.stringify(config);
    const configChanged = lastPersistedConfigRef.current !== configKey;
    if (!configChanged && now - lastAccountSnapshotAtRef.current < ACCOUNT_SNAPSHOT_THROTTLE_MS) return;
    lastAccountSnapshotAtRef.current = now;
    lastPersistedConfigRef.current = configKey;
    void persistAccountSnapshot(account);
  }, [account, config, persistAccountSnapshot, persistenceDisabled]);

  return {
    trades,
    historyTrades,
    analytics,
    account,
    logs,
    config,
    setConfig,
    ingestTraceRows,
    ingestResearchSignals,
    closeTrade,
    reset,
    loading,
    error,
    traceAgeSeconds,
    mockLimitRejectedSignals,
    persistence,
    history: {
      page: historyPage,
      limit: HISTORY_PAGE_SIZE,
      total: historyMeta.total,
      totalPages: historyMeta.totalPages,
      loading: historyMeta.loading,
      error: historyMeta.error,
      setPage: setHistoryPage,
    },
  };
}
