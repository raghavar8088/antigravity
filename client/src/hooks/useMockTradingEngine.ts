"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  applyPriceTickToTrade,
  markMockTradeAtPrice,
  buildMockTradeFromTrace,
  buildMockTradeFromResearchSignal,
  closeMockTrade,
  computeAccountState,
  computeAnalytics,
  DEFAULT_MAX_OPEN_MOCK_TRADES,
  DEFAULT_MOCK_TRADING_CONFIG,
  emptyMockDiagnosticFunnel,
  emptyMockRejectionCounts,
  emptyMockTradingDiagnostics,
  evaluateMockTradeOpenRisk,
  isExecutableTraceRow,
  isValidMockTrade,
  logForMockTradeCreated,
  logForMockTradeClosed,
  logForMockTradeRejected,
  maxOpenMockTradesFromConfig,
  maxSignalsPerBatchFromConfig,
  normalizeMockTradingConfig,
  rejectionCategoryFromRiskCode,
  rejectionCategoryFromTraceGate,
  scoreMockResearchSignal,
  scoreMockTraceRow,
  type MockDiagnosticFunnel,
  type MockRejectionCategory,
  type MockRejectionCounts,
  type MockRejectionEvent,
  type MockAccountState,
  type MockTrade,
  type MockTradeAnalytics,
  type MockTradeLog,
  type MockTradingConfig,
  type MockResearchSignalInput,
  type MockTradingDiagnostics,
  type StrategyExitOverride,
} from "@/lib/trading/mockTradingEngine";
import { FUTURES_STRAT_DEFS } from "@/lib/trading/futuresStrategies";
import type { MockExecutorHealth } from "@/lib/mockTradingExecutor/types";
import type { ExecutorDiagnosis } from "@/lib/mockTradingExecutor/diagnoseExecutor";
import {
  DEFAULT_MOCK_ACCOUNT_KEY,
  MOCK_RESET_CONFIRMATION,
  MOCK_CLEAR_CLOSED_CONFIRMATION,
  mergeHydratedMockTrades,
  mergePortfolioTrades,
  type MockAccountSnapshotResponse,
  type MockLogsResponse,
  type MockTradingHydrateResponse,
} from "@/lib/trading/mockTradingPersistenceTypes";
import type { StrategyStatus } from "@/lib/strategyAuthority/types";
import { getMockConfigForPipelineStage } from "@/lib/strategyAuthority/gradeStageMockConfig";
import type { StrategySignalTraceRow } from "@/lib/ai/strategySignalTrace";

const TRACE_POLL_MS = 5_000;
const DEFAULT_EXECUTOR_STATUS_POLL_MS = 2_000;
const MARK_PERSIST_THROTTLE_MS = 15_000;
const ACCOUNT_SNAPSHOT_THROTTLE_MS = 15_000;
const OPEN_MARK_PERSIST_BATCH_SIZE = 100;
const LOG_RING_CAP = 200;
const TRADE_CACHE_MIN_CAP = 500;
const HISTORY_PAGE_SIZE = 100;
const SUMMARY_FETCH_CAP = 50_000;

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

function trimTradeCache(trades: MockTrade[], config: MockTradingConfig): MockTrade[] {
  const cap = Math.max(TRADE_CACHE_MIN_CAP, maxOpenMockTradesFromConfig(config));
  if (trades.length <= cap) return trades;
  const open = trades.filter((trade) => trade.status === "OPEN");
  const closed = trades.filter((trade) => trade.status === "CLOSED");
  const closedKeep = Math.max(0, cap - open.length);
  const retainedClosed = closedKeep > 0 ? closed.slice(-closedKeep) : [];
  return [...retainedClosed, ...open].sort((a, b) => a.openedAt - b.openedAt);
}

function diagnosticsDelta(): {
  funnel: MockDiagnosticFunnel;
  rejectionCounts: MockRejectionCounts;
  recentRejections: MockRejectionEvent[];
} {
  return {
    funnel: emptyMockDiagnosticFunnel(),
    rejectionCounts: emptyMockRejectionCounts(),
    recentRejections: [],
  };
}

function addDiagnosticsRejection(
  delta: ReturnType<typeof diagnosticsDelta>,
  event: MockRejectionEvent,
): void {
  delta.rejectionCounts[event.category] += 1;
  delta.recentRejections.unshift(event);
}

function applyDiagnosticsDelta(
  current: MockTradingDiagnostics,
  delta: ReturnType<typeof diagnosticsDelta>,
  now: number,
): MockTradingDiagnostics {
  const next: MockTradingDiagnostics = {
    funnel: { ...current.funnel },
    rejectionCounts: { ...current.rejectionCounts },
    recentRejections: [...delta.recentRejections, ...current.recentRejections].slice(0, 75),
    lastUpdatedAt: now,
  };
  for (const key of Object.keys(delta.funnel) as Array<keyof MockDiagnosticFunnel>) {
    next.funnel[key] += delta.funnel[key];
  }
  for (const key of Object.keys(delta.rejectionCounts) as Array<keyof MockRejectionCounts>) {
    next.rejectionCounts[key] += delta.rejectionCounts[key];
  }
  return next;
}

function incrementFunnelForDecision(
  funnel: MockDiagnosticFunnel,
  category: MockRejectionCategory | null,
  created: boolean,
): void {
  if (category !== "LOW_SIGNAL_SCORE") funnel.scorePassed++;
  if (category !== "LOW_SIGNAL_SCORE" && category !== "RR_TOO_LOW") funnel.riskRewardPassed++;
  if (
    category !== "LOW_SIGNAL_SCORE" &&
    category !== "RR_TOO_LOW" &&
    category !== "DAILY_LOSS_LIMIT" &&
    category !== "WEEKLY_LOSS_LIMIT" &&
    category !== "MAX_DRAWDOWN" &&
    category !== "MARGIN_CHECK"
  ) {
    funnel.riskPassed++;
  }
  if (category !== "COOLDOWN" && category !== "LOW_SIGNAL_SCORE" && category !== "RR_TOO_LOW") funnel.cooldownPassed++;
  if (category !== "FAMILY_CONFLICT" && category !== "COOLDOWN" && category !== "LOW_SIGNAL_SCORE" && category !== "RR_TOO_LOW") {
    funnel.familyConflictPassed++;
  }
  if (
    category !== "EXPOSURE_LIMIT" &&
    category !== "MARGIN_CHECK" &&
    category !== "FAMILY_CONFLICT" &&
    category !== "COOLDOWN" &&
    category !== "LOW_SIGNAL_SCORE" &&
    category !== "RR_TOO_LOW"
  ) {
    funnel.exposurePassed++;
  }
  if (created) funnel.tradesCreated++;
}

export interface UseMockTradingEngineOptions {
  /** Live BTC price (USD). 0 means not yet connected. */
  price: number;
  /** Optional account key to forward to the signal-trace API. */
  accountKey?: string | null;
  /** Legacy pipeline stage label retained for persisted mock trade compatibility. */
  pipelineStage?: StrategyStatus | null;
  /** Optional initial config override (merged via normalizeMockTradingConfig). */
  initialConfig?: Partial<MockTradingConfig>;
  /** Disable the network poll (used by tests). */
  disablePolling?: boolean;
  /** Disable Mongo persistence/hydration (used by tests). */
  disablePersistence?: boolean;
  /** Poll backend executor health (read-only Trade Engine mode). */
  refetchExecutorStatus?: boolean;
  /** Executor status poll interval when refetchExecutorStatus is true. */
  executorStatusPollMs?: number;
}

export interface UseMockTradingEngineResult {
  /** Live engine cache — open trades and recently closed rows. */
  trades: MockTrade[];
  /** Paginated Mongo history for the trade table. */
  historyTrades: MockTrade[];
  /** Live cache merged with persisted history — use for portfolio metrics. */
  portfolioTrades: MockTrade[];
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
  /** Remove all closed trades; open positions and balance are kept. */
  clearClosedTrades: () => boolean;
  /** Remove one closed trade by id. */
  deleteClosedTrade: (tradeId: string) => boolean;
  loading: boolean;
  error: string | null;
  traceAgeSeconds: number | null;
  mockLimitRejectedSignals: number;
  diagnostics: MockTradingDiagnostics;
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
  executor: {
    health: MockExecutorHealth | null;
    /** @deprecated use diagnosis.reason */
    noTradeReason: ExecutorDiagnosis["reason"] | null;
    diagnosis: ExecutorDiagnosis | null;
    lastFetchedAt: number | null;
  };
}

export function useMockTradingEngine(
  opts: UseMockTradingEngineOptions,
): UseMockTradingEngineResult {
  const {
    price,
    accountKey,
    pipelineStage = null,
    initialConfig,
    disablePolling = false,
    disablePersistence = false,
    refetchExecutorStatus = false,
    executorStatusPollMs = DEFAULT_EXECUTOR_STATUS_POLL_MS,
  } = opts;
  const persistenceDisabled = disablePersistence;
  const mockAccountKey = accountKey?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const stageConfig = pipelineStage ? getMockConfigForPipelineStage(pipelineStage) : DEFAULT_MOCK_TRADING_CONFIG;
  const bootConfig = normalizeMockTradingConfig({
    ...stageConfig,
    ...(initialConfig ?? {}),
    ...(pipelineStage ? { pipelineStage } : {}),
  });

  const [trades, setTrades] = useState<MockTrade[]>([]);
  const [historyTrades, setHistoryTrades] = useState<MockTrade[]>([]);
  const [summaryTrades, setSummaryTrades] = useState<MockTrade[]>([]);
  const [hydratedAccount, setHydratedAccount] = useState<MockAccountState | null>(null);
  const [config, setConfigState] = useState<MockTradingConfig>(bootConfig);
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
    total: 0,
    totalPages: 1,
    loading: false,
    error: null as string | null,
  });
  const [mockLimitRejectedSignals, setMockLimitRejectedSignals] = useState(0);
  const [diagnostics, setDiagnostics] = useState<MockTradingDiagnostics>(emptyMockTradingDiagnostics);
  const [, setTickRefresh] = useState(0);

  const seenTraceIdsRef = useRef<Set<string>>(new Set());
  const configRef = useRef(config);
  const priceRef = useRef(price);
  const tradesRef = useRef(trades);
  const summaryTradesRef = useRef(summaryTrades);
  const lastMarkPersistAtRef = useRef(0);
  const lastAccountSnapshotAtRef = useRef(0);
  const lastPersistedConfigRef = useRef<string | null>(null);
  const openMarkPersistCursorRef = useRef(0);

  useEffect(() => { configRef.current = config; }, [config]);
  useEffect(() => { priceRef.current = price; }, [price]);
  useEffect(() => { tradesRef.current = trades; }, [trades]);
  useEffect(() => { summaryTradesRef.current = summaryTrades; }, [summaryTrades]);

  const portfolioTradesFor = useCallback(
    (liveTrades: readonly MockTrade[]) =>
      mergePortfolioTrades(liveTrades, summaryTradesRef.current.length > 0 ? summaryTradesRef.current : historyTrades),
    [historyTrades],
  );

  const syncSummaryTrades = useCallback((updates: readonly MockTrade[]) => {
    if (updates.length === 0) return;
    setSummaryTrades((prev) => mergePortfolioTrades(updates, prev));
  }, []);

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
          if (mode === "update" && json?.code === "DEPRECATED") {
            markPersistenceOk();
            return;
          }
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

  // Hydrate all state from MongoDB on mount. No localStorage fallback.
  useEffect(() => {
    if (persistenceDisabled) return;
    let cancelled = false;
    const hydrate = async () => {
      setPersistence((prev) => ({ ...prev, status: "hydrating", loading: true, error: null }));
      try {
        const hydrateOpenLimit = pipelineStage
          ? maxOpenMockTradesFromConfig(getMockConfigForPipelineStage(pipelineStage))
          : DEFAULT_MAX_OPEN_MOCK_TRADES;
        const openParams = new URLSearchParams({
          account_key: mockAccountKey,
          page: "1",
          limit: String(hydrateOpenLimit),
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
        if (openTradesRes.status === 503 && historyRes.status === 503) {
          markPersistenceError("MongoDB not configured", true);
          return;
        }
        const openTradesJson = await openTradesRes.json() as MockTradingHydrateResponse | { ok: false; error?: string };
        const historyJson = await historyRes.json() as MockTradingHydrateResponse | { ok: false; error?: string };
        const accountJson = accountRes.ok
          ? await accountRes.json() as MockAccountSnapshotResponse | { ok: false; error?: string }
          : { ok: false as const, error: `HTTP ${accountRes.status}` };
        const logsJson = logsRes.ok
          ? await logsRes.json() as MockLogsResponse | { ok: false; error?: string }
          : { ok: false as const, error: `HTTP ${logsRes.status}` };
        if (!openTradesRes.ok || !openTradesJson.ok) throw new Error("error" in openTradesJson ? openTradesJson.error ?? "Open trade hydrate failed" : "Open trade hydrate failed");
        if (!historyRes.ok || !historyJson.ok) throw new Error("error" in historyJson ? historyJson.error ?? "Trade history hydrate failed" : "Trade history hydrate failed");

        const merged = mergeHydratedMockTrades(tradesRef.current, openTradesJson.trades);
        setTrades(merged);
        setHistoryTrades(historyJson.trades);

        let summary = historyJson.trades;
        if (historyJson.total > historyJson.trades.length) {
          const fullParams = new URLSearchParams({
            account_key: mockAccountKey,
            page: "1",
            limit: String(Math.min(historyJson.total, SUMMARY_FETCH_CAP)),
            sort: "newest",
          });
          const fullRes = await fetch(`/api/mock-trading/trades?${fullParams.toString()}`);
          const fullJson = await fullRes.json() as MockTradingHydrateResponse | { ok: false; error?: string };
          if (fullRes.ok && fullJson.ok) summary = fullJson.trades;
        }
        setSummaryTrades(mergePortfolioTrades(merged, summary));
        setHydratedAccount(accountJson.ok ? accountJson.snapshot ?? null : null);

        setHistoryMeta({
          total: historyJson.total,
          totalPages: Math.max(1, Math.ceil(historyJson.total / HISTORY_PAGE_SIZE)),
          loading: false,
          error: null,
        });
        if (accountJson.ok && accountJson.config) {
          setConfigState(
            pipelineStage
              ? getMockConfigForPipelineStage(pipelineStage)
              : normalizeMockTradingConfig(accountJson.config),
          );
        }
        setLogs(logsJson.ok ? logsJson.logs.slice(0, LOG_RING_CAP) : []);
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
  }, [markPersistenceError, mockAccountKey, persistenceDisabled, pipelineStage]);

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
      let limitRejected = 0;
      // Equity for percent-of-equity sizing is based on current state.
      const equity = computeAccountState(
        portfolioTradesFor(tradesRef.current),
        configRef.current,
      ).equity;
      const delta = diagnosticsDelta();
      const raisedRows = rows.filter(
        (row) => isExecutableTraceRow(row) && !seenTraceIdsRef.current.has(row.traceId),
      );
      delta.funnel.signalsGenerated += raisedRows.length;
      delta.funnel.confidencePassed += raisedRows.length;
      const rankedRows = raisedRows
        .sort((a, b) => scoreMockTraceRow(b) - scoreMockTraceRow(a));
      const selectedRows = rankedRows.slice(0, maxSignalsPerBatchFromConfig(configRef.current));
      for (const row of rankedRows.slice(selectedRows.length)) {
        addDiagnosticsRejection(delta, {
          ts: now,
          category: "OTHER",
          strategyId: row.strategyId,
          strategyName: row.strategyName,
          side: row.side === "SHORT" ? "SELL" : "BUY",
          signalScore: row.signalScore,
          message: "signal was outside maxSignalsPerBatch top-ranked selection",
        });
      }

      for (const row of selectedRows) {
        const side = row.side === "SHORT" ? "SELL" : "BUY";
        const override = STRATEGY_EXIT_OVERRIDES.get(row.strategyId);
        const trade = buildMockTradeFromTrace({
          row,
          currentPrice: livePrice,
          config: configRef.current,
          now,
          equity,
          override,
        });
        if (!trade) {
          seenTraceIdsRef.current.add(row.traceId);
          limitRejected++;
          const code = row.status === "REJECTED" || row.status === "FIRED" ? "BLOCKER" : "SIGNAL_SCORE";
          const category = row.status === "REJECTED" || row.status === "FIRED"
            ? rejectionCategoryFromTraceGate(row.gate)
            : "LOW_SIGNAL_SCORE";
          addDiagnosticsRejection(delta, {
            ts: now,
            category,
            strategyId: row.strategyId,
            strategyName: row.strategyName,
            side,
            signalScore: row.signalScore,
            message: row.status === "REJECTED" || row.status === "FIRED"
              ? `${row.gate}: ${row.reason}`
              : "signal failed score or trade quality threshold",
          });
          newLogs.push(logForMockTradeRejected({
            ts: now,
            strategyId: row.strategyId,
            strategyName: row.strategyName,
            side,
            price: livePrice,
            code,
            category,
            signalScore: row.signalScore,
            message: row.status === "REJECTED" || row.status === "FIRED"
              ? `${row.gate}: ${row.reason}`
              : "signal failed score or trade quality threshold",
          }));
          console.info(
            "[MOCK_TRADE_REJECTED]",
            {
              ts: now,
              strategyId: row.strategyId,
              strategyName: row.strategyName,
              side,
              signalScore: row.signalScore,
              rejectionReason: category,
              gate: row.gate,
              message: row.reason,
            },
          );
          continue;
        }
        const decision = evaluateMockTradeOpenRisk({
          trade,
          existingTrades: tradesRef.current,
          pendingTrades: newTrades,
          config: configRef.current,
          now,
        });
        if (!decision.allowed) {
          seenTraceIdsRef.current.add(row.traceId);
          limitRejected++;
          const category = rejectionCategoryFromRiskCode(decision.code);
          incrementFunnelForDecision(delta.funnel, category, false);
          addDiagnosticsRejection(delta, {
            ts: now,
            category,
            strategyId: row.strategyId,
            strategyName: row.strategyName,
            side,
            signalScore: trade.signalScore,
            riskRewardRatio: trade.riskRewardRatio,
            message: decision.message ?? "risk engine rejected trade",
          });
          newLogs.push(logForMockTradeRejected({
            ts: now,
            strategyId: row.strategyId,
            strategyName: row.strategyName,
            side,
            price: livePrice,
            code: decision.code ?? "MAX_OPEN",
            category,
            signalScore: trade.signalScore,
            riskRewardRatio: trade.riskRewardRatio,
            message: decision.message ?? "risk engine rejected trade",
          }));
          console.info(
            "[MOCK_TRADE_REJECTED]",
            {
              ts: now,
              strategyId: row.strategyId,
              strategyName: row.strategyName,
              side,
              signalScore: trade.signalScore,
              riskRewardRatio: trade.riskRewardRatio,
              rejectionReason: category,
              code: decision.code ?? "UNKNOWN",
              message: decision.message ?? "",
            },
          );
          continue;
        }
        incrementFunnelForDecision(delta.funnel, null, true);
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
          `riskReward=${trade.riskRewardRatio.toFixed(2)}`,
        );
      }
      if (limitRejected > 0) setMockLimitRejectedSignals((count) => count + limitRejected);
      if (delta.funnel.signalsGenerated > 0 || delta.recentRejections.length > 0 || delta.funnel.tradesCreated > 0) {
        setDiagnostics((prev) => applyDiagnosticsDelta(prev, delta, now));
      }
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
      syncSummaryTrades(newTrades);
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
    [historyPage, persistTrade, portfolioTradesFor, syncSummaryTrades],
  );

  // ── Ingest 500-strategy research signals → create mock trades only ────────
  const ingestResearchSignals = useCallback(
    (signals: MockResearchSignalInput[], priceOverride?: number) => {
      const livePrice = priceOverride ?? priceRef.current;
      if (!Number.isFinite(livePrice) || livePrice <= 0) return;

      const newTrades: MockTrade[] = [];
      const newLogs: MockTradeLog[] = [];
      const now = Date.now();
      let limitRejected = 0;
      const equity = computeAccountState(
        portfolioTradesFor(tradesRef.current),
        configRef.current,
      ).equity;
      const delta = diagnosticsDelta();
      const freshSignals = signals.filter((signal) => !seenTraceIdsRef.current.has(`mock-research-${signal.strategyId}-${signal.evaluatedAt}-${signal.side}`));
      delta.funnel.signalsGenerated += freshSignals.length;
      delta.funnel.confidencePassed += freshSignals.length;
      const rankedSignals = freshSignals.sort((a, b) => scoreMockResearchSignal(b) - scoreMockResearchSignal(a));
      const selectedSignals = rankedSignals.slice(0, maxSignalsPerBatchFromConfig(configRef.current));
      for (const signal of rankedSignals.slice(selectedSignals.length)) {
        addDiagnosticsRejection(delta, {
          ts: now,
          category: "OTHER",
          strategyId: signal.strategyId,
          strategyName: signal.strategyName,
          side: signal.side,
          confidenceScore: signal.confidenceScore,
          message: "signal was outside maxSignalsPerBatch top-ranked selection",
        });
      }

      for (const signal of selectedSignals) {
        const dedupeKey = `mock-research-${signal.strategyId}-${signal.evaluatedAt}-${signal.side}`;
        const duplicateOpen = [...tradesRef.current, ...newTrades].some(
          (trade) =>
            trade.status === "OPEN" &&
            trade.strategyId === signal.strategyId &&
            trade.side === signal.side,
        );
        if (duplicateOpen) {
          seenTraceIdsRef.current.add(dedupeKey);
          limitRejected++;
          addDiagnosticsRejection(delta, {
            ts: now,
            category: "DUPLICATE_POSITION",
            strategyId: signal.strategyId,
            strategyName: signal.strategyName,
            side: signal.side,
            confidenceScore: signal.confidenceScore,
            message: "strategy already has an open same-side position",
          });
          newLogs.push(logForMockTradeRejected({
            ts: now,
            strategyId: signal.strategyId,
            strategyName: signal.strategyName,
            side: signal.side,
            price: livePrice,
            code: "COOLDOWN",
            category: "DUPLICATE_POSITION",
            confidenceScore: signal.confidenceScore,
            message: "strategy already has an open same-side position",
          }));
          continue;
        }
        const trade = buildMockTradeFromResearchSignal({
          signal,
          currentPrice: livePrice,
          config: configRef.current,
          now,
          equity,
        });
        if (!trade) {
          seenTraceIdsRef.current.add(dedupeKey);
          limitRejected++;
          addDiagnosticsRejection(delta, {
            ts: now,
            category: "LOW_SIGNAL_SCORE",
            strategyId: signal.strategyId,
            strategyName: signal.strategyName,
            side: signal.side,
            confidenceScore: signal.confidenceScore,
            message: "research signal failed score or trade quality threshold",
          });
          newLogs.push(logForMockTradeRejected({
            ts: now,
            strategyId: signal.strategyId,
            strategyName: signal.strategyName,
            side: signal.side,
            price: livePrice,
            code: "SIGNAL_SCORE",
            category: "LOW_SIGNAL_SCORE",
            confidenceScore: signal.confidenceScore,
            message: "research signal failed score or trade quality threshold",
          }));
          continue;
        }
        const decision = evaluateMockTradeOpenRisk({
          trade,
          existingTrades: tradesRef.current,
          pendingTrades: newTrades,
          config: configRef.current,
          now,
        });
        if (!decision.allowed) {
          seenTraceIdsRef.current.add(dedupeKey);
          limitRejected++;
          const category = rejectionCategoryFromRiskCode(decision.code);
          incrementFunnelForDecision(delta.funnel, category, false);
          addDiagnosticsRejection(delta, {
            ts: now,
            category,
            strategyId: signal.strategyId,
            strategyName: signal.strategyName,
            side: signal.side,
            signalScore: trade.signalScore,
            confidenceScore: signal.confidenceScore,
            riskRewardRatio: trade.riskRewardRatio,
            message: decision.message ?? "risk engine rejected trade",
          });
          newLogs.push(logForMockTradeRejected({
            ts: now,
            strategyId: signal.strategyId,
            strategyName: signal.strategyName,
            side: signal.side,
            price: livePrice,
            code: decision.code ?? "MAX_OPEN",
            category,
            signalScore: trade.signalScore,
            confidenceScore: signal.confidenceScore,
            riskRewardRatio: trade.riskRewardRatio,
            message: decision.message ?? "risk engine rejected trade",
          }));
          console.info(
            "[MOCK_TRADE_REJECTED]",
            {
              ts: now,
              strategyId: signal.strategyId,
              strategyName: signal.strategyName,
              side: signal.side,
              signalScore: trade.signalScore,
              confidenceScore: signal.confidenceScore,
              riskRewardRatio: trade.riskRewardRatio,
              rejectionReason: category,
              code: decision.code ?? "UNKNOWN",
              source: "research",
            },
          );
          continue;
        }
        incrementFunnelForDecision(delta.funnel, null, true);
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
      if (delta.funnel.signalsGenerated > 0 || delta.recentRejections.length > 0 || delta.funnel.tradesCreated > 0) {
        setDiagnostics((prev) => applyDiagnosticsDelta(prev, delta, now));
      }
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
      syncSummaryTrades(newTrades);
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
    [historyPage, persistTrade, portfolioTradesFor, syncSummaryTrades],
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
        params.set("account_key", mockAccountKey);
        const res = await fetch(`/api/mock-trading/signal-tick?${params.toString()}`);
        const json = (await res.json()) as {
          rows?: StrategySignalTraceRow[];
          markPrice?: number;
          error?: string;
        };
        if (cancelled) return;
        if (!res.ok || !json.rows) {
          setError(json.error ?? `HTTP ${res.status}`);
          return;
        }
        const tickPrice =
          Number.isFinite(json.markPrice) && (json.markPrice ?? 0) > 0
            ? json.markPrice
            : undefined;
        ingestTraceRows(json.rows, tickPrice);
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
  }, [mockAccountKey, disablePolling, ingestTraceRows, pipelineStage]);

  const [executorHealth, setExecutorHealth] = useState<MockExecutorHealth | null>(null);
  const [executorDiagnosis, setExecutorDiagnosis] = useState<ExecutorDiagnosis | null>(null);
  const [executorLastFetchedAt, setExecutorLastFetchedAt] = useState<number | null>(null);

  // ── Backend executor health (read-only Trade Engine) ─────────────────────
  useEffect(() => {
    if (!refetchExecutorStatus || persistenceDisabled) return;
    let cancelled = false;

    const syncOpenTrades = async () => {
      const openParams = new URLSearchParams({
        account_key: mockAccountKey,
        page: "1",
        limit: String(maxOpenMockTradesFromConfig(configRef.current)),
        status: "OPEN",
        sort: "newest",
      });
      const res = await fetch(`/api/mock-trading/trades?${openParams.toString()}`);
      if (!res.ok) return;
      const json = (await res.json()) as MockTradingHydrateResponse;
      if (!json.ok) return;
      setTrades((prev) => {
        const closed = prev.filter((t) => t.status === "CLOSED");
        const byId = new Map(closed.map((t) => [t.id, t]));
        for (const open of json.trades) byId.set(open.id, open);
        return [...byId.values()];
      });
    };

    const pollExecutor = async () => {
      try {
        const params = new URLSearchParams({ account_key: mockAccountKey });
        const res = await fetch(`/api/mock-trading/executor-status?${params.toString()}`);
        const json = (await res.json()) as {
          ok?: boolean;
          executor?: MockExecutorHealth;
          no_trade_diagnosis?: ExecutorDiagnosis;
        };
        if (cancelled || !res.ok || !json.ok || !json.executor) return;
        setExecutorHealth(json.executor);
        setExecutorDiagnosis(json.no_trade_diagnosis ?? null);
        setExecutorLastFetchedAt(Date.now());
        if (json.executor.healthy) {
          await syncOpenTrades();
        }
      } catch {
        if (!cancelled) {
          setExecutorHealth((prev) =>
            prev
              ? { ...prev, healthy: false, stale: true }
              : {
                  healthy: false,
                  stale: true,
                  ageSeconds: null,
                  lastTickAt: null,
                  lastMode: null,
                  dominantBlocker: null,
                  candidateCount: null,
                  openedLastCycle: null,
                  closedLastCycle: null,
                  errors: ["executor_status_fetch_failed"],
                },
          );
        }
      }
    };

    void pollExecutor();
    const id = setInterval(() => void pollExecutor(), executorStatusPollMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [
    executorStatusPollMs,
    mockAccountKey,
    persistenceDisabled,
    refetchExecutorStatus,
  ]);

  // ── Apply price ticks → unrealized PnL and TP/SL ──────────────────────────
  // Engine-owned mode (disablePolling): live marks are applied in portfolioTrades useMemo.
  useEffect(() => {
    if (disablePolling) return;
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
    if (closedTrades.length > 0) syncSummaryTrades(closedTrades);
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
  }, [historyPage, persistTrade, price, syncSummaryTrades]);

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
      syncSummaryTrades([closed]);
      void persistTrade(closed, "close");
    }
  }, [persistTrade, syncSummaryTrades]);

  const reset = useCallback(() => {
    const confirmed = typeof window === "undefined"
      ? true
      : window.confirm("Reset all persisted Trade Engine trades, logs, analytics, and account snapshots?");
    if (!confirmed) return;
    setTrades([]);
    setHistoryTrades([]);
    setSummaryTrades([]);
    setHydratedAccount(null);
    setLogs([]);
    setMockLimitRejectedSignals(0);
    setDiagnostics(emptyMockTradingDiagnostics());
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

  const removeClosedFromState = useCallback((tradeId?: string) => {
    const isClosedTarget = (trade: MockTrade) =>
      trade.status === "CLOSED" && (tradeId == null || trade.id === tradeId);
    setTrades((prev) => prev.filter((trade) => !isClosedTarget(trade)));
    setHistoryTrades((prev) => prev.filter((trade) => !isClosedTarget(trade)));
    setSummaryTrades((prev) => prev.filter((trade) => !isClosedTarget(trade)));
    setHistoryMeta((prev) => ({
      ...prev,
      total: tradeId == null ? 0 : Math.max(0, prev.total - 1),
    }));
  }, []);

  const clearClosedTrades = useCallback((): boolean => {
    const confirmed = typeof window === "undefined"
      ? true
      : window.confirm("Clear all closed trades from history? Open positions and balance will be kept.");
    if (!confirmed) return false;
    removeClosedFromState();
    if (!persistenceDisabled) {
      void fetch("/api/mock-trading/trades/closed", {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ accountKey: mockAccountKey, confirmation: MOCK_CLEAR_CLOSED_CONFIRMATION }),
      }).then((res) => {
        if (res.ok) markPersistenceOk();
        else markPersistenceError(`Clear closed trades failed: HTTP ${res.status}`, res.status === 503);
      }).catch((err: Error) => markPersistenceError(err.message));
    }
    return true;
  }, [markPersistenceError, markPersistenceOk, mockAccountKey, persistenceDisabled, removeClosedFromState]);

  const deleteClosedTrade = useCallback((tradeId: string): boolean => {
    const exists = [...trades, ...historyTrades, ...summaryTrades].some(
      (trade) => trade.id === tradeId && trade.status === "CLOSED",
    );
    if (!exists) return false;
    const confirmed = typeof window === "undefined"
      ? true
      : window.confirm("Remove this closed trade from history?");
    if (!confirmed) return false;
    removeClosedFromState(tradeId);
    if (!persistenceDisabled) {
      void fetch(`/api/mock-trading/trades/${encodeURIComponent(tradeId)}`, {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ accountKey: mockAccountKey }),
      }).then((res) => {
        if (res.ok) markPersistenceOk();
        else markPersistenceError(`Delete trade failed: HTTP ${res.status}`, res.status === 503);
      }).catch((err: Error) => markPersistenceError(err.message));
    }
    return true;
  }, [historyTrades, markPersistenceError, markPersistenceOk, mockAccountKey, persistenceDisabled, removeClosedFromState, summaryTrades, trades]);

  const basePortfolioTrades = useMemo(
    () => mergePortfolioTrades(trades, summaryTrades.length > 0 ? summaryTrades : historyTrades),
    [trades, summaryTrades, historyTrades],
  );
  const portfolioTrades = useMemo(() => {
    if (!disablePolling || !Number.isFinite(price) || price <= 0) return basePortfolioTrades;
    const now = Date.now();
    return basePortfolioTrades.map((t) => {
      if (t.status !== "OPEN") return t;
      const override = STRATEGY_EXIT_OVERRIDES.get(t.strategyId);
      return markMockTradeAtPrice({ trade: t, price, config, now, override });
    });
  }, [basePortfolioTrades, config, disablePolling, price]);
  const computedAccount = useMemo(
    () => computeAccountState(portfolioTrades, config),
    [portfolioTrades, config],
  );
  const account = useMemo(() => {
    if (portfolioTrades.length === 0 && hydratedAccount) return hydratedAccount;
    return computedAccount;
  }, [computedAccount, hydratedAccount, portfolioTrades.length]);
  const analytics = useMemo(() => computeAnalytics(portfolioTrades), [portfolioTrades]);
  const traceAgeSeconds = lastFetchAt != null ? Math.floor((Date.now() - lastFetchAt) / 1000) : null;

  useEffect(() => {
    // Account snapshots are written by the Go engine to paper_state.
    // Browser snapshots to mock_account_snapshots are disabled.
    if (persistenceDisabled || disablePolling) return;
    const now = Date.now();
    const configKey = JSON.stringify(config);
    const configChanged = lastPersistedConfigRef.current !== configKey;
    if (!configChanged && now - lastAccountSnapshotAtRef.current < ACCOUNT_SNAPSHOT_THROTTLE_MS) return;
    lastAccountSnapshotAtRef.current = now;
    lastPersistedConfigRef.current = configKey;
    void persistAccountSnapshot(account);
  }, [account, config, disablePolling, persistAccountSnapshot, persistenceDisabled]);

  return {
    trades,
    historyTrades,
    portfolioTrades,
    analytics,
    account,
    logs,
    config,
    setConfig,
    ingestTraceRows,
    ingestResearchSignals,
    closeTrade,
    reset,
    clearClosedTrades,
    deleteClosedTrade,
    loading,
    error,
    traceAgeSeconds,
    mockLimitRejectedSignals,
    diagnostics,
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
    executor: {
      health: executorHealth,
      noTradeReason: executorDiagnosis?.reason ?? null,
      diagnosis: executorDiagnosis,
      lastFetchedAt: executorLastFetchedAt,
    },
  };
}
