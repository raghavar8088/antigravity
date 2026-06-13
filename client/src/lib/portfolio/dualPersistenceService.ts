/**
 * Dual Persistence Service — central coordinator.
 *
 * Every important event goes through here:
 *   1. MongoDB write (primary source of truth, caller handles this)
 *   2. Local filesystem write (secondary backup, enqueued to writeQueue)
 *   3. Audit NDJSON append (event sourcing log)
 *
 * Usage:
 *   After a successful MongoDB upsert, call the matching dps.* method.
 *   The local write is always fire-and-forget (non-blocking).
 */

import type { MockTrade, MockTradingConfig, MockAccountState } from "../trading/mockTradingEngine";
import type { MockTradeAnalytics } from "../trading/mockTradingEngine";
import type { RegimeSnapshot } from "../ai/marketRegimeClassifier";
import type { StrategyScore } from "../ai/strategyScoringEngine";
import { writeQueue } from "./writeQueue";
import {
  initLocalStorage,
  appendNdjson,
  appendCsv,
  atomicWriteJson,
  todayFilePath,
  datePath,
  totalStorageBytes,
  dirStats,
  getDataRoot,
} from "../utils/localStorageService";
import fs from "fs";
import path from "path";

// ── Event types for audit log ──────────────────────────────────────────────

export type AuditEventType =
  | "SIGNAL_GENERATED"
  | "SIGNAL_REJECTED"
  | "TRADE_CREATED"
  | "TRADE_CLOSED"
  | "POSITION_OPENED"
  | "POSITION_CLOSED"
  | "RISK_REJECTED"
  | "CONFIG_CHANGED"
  | "SYSTEM_START"
  | "SYSTEM_STOP"
  | "BACKUP_CREATED"
  | "BACKUP_FAILED"
  | "RESTORE_STARTED"
  | "RESTORE_COMPLETED"
  | "WRITE_FAILED"
  | "WRITE_QUEUED";

export interface AuditEvent {
  type: AuditEventType;
  timestamp: string;
  accountKey?: string;
  tradeId?: string;
  strategyId?: number;
  strategyName?: string;
  side?: string;
  price?: number;
  pnl?: number;
  regime?: string;
  message?: string;
  payload?: Record<string, unknown>;
}

// ── Signal record ──────────────────────────────────────────────────────────

export interface SignalRecord {
  strategyId: number;
  strategyName: string;
  strategyFamily: string;
  symbol: string;
  side: "BUY" | "SELL" | "NO_SIGNAL";
  timestamp: number;
  timeframe: string;
  entryPrice: number;
  confidenceScore: number;
  marketRegime?: string;
  indicatorSnapshot?: Record<string, unknown>;
  takeProfit?: number;
  stopLoss?: number;
  expectedProfitUsd?: number;
  expectedLossUsd?: number;
  riskReward?: number;
  entryReason: string;
}

// ── Trade CSV columns ──────────────────────────────────────────────────────

const TRADE_CSV_HEADERS = [
  "trade_id", "opened_at", "closed_at", "strategy_id", "strategy_name",
  "symbol", "side", "status", "entry_price", "close_price", "quantity",
  "realized_pnl", "fees", "funding_costs", "risk_reward_ratio",
  "confidence_score", "exit_reason", "regime_at_entry",
];

// ── DualPersistenceService class ───────────────────────────────────────────

class DualPersistenceService {
  private readonly writeFailures: { label: string; error: string; ts: string }[] = [];
  private filesWrittenToday = 0;
  private startedAt = Date.now();

  constructor() {
    initLocalStorage();
    this.emitAuditEvent({ type: "SYSTEM_START", message: "DualPersistenceService initialised", timestamp: new Date().toISOString() });
  }

  // ── Mock Trade ─────────────────────────────────────────────────────────

  persistMockTrade(trade: MockTrade, accountKey: string): void {
    this.enqueue("mock-trade:" + trade.id, async () => {
      // NDJSON stream (append-only)
      await appendNdjson(
        todayFilePath("mockTrades", "trades.ndjson"),
        { ...flattenTrade(trade), account_key: accountKey, persisted_at: new Date().toISOString() },
      );
      // CSV (analytics-friendly)
      await appendCsv(
        todayFilePath("mockTrades", "trades.csv"),
        TRADE_CSV_HEADERS,
        tradeToRow(trade),
      );
    });

    const eventType: AuditEventType = trade.status === "CLOSED" ? "TRADE_CLOSED" : "TRADE_CREATED";
    this.emitAuditEvent({
      type: eventType,
      timestamp: new Date().toISOString(),
      accountKey,
      tradeId: trade.id,
      strategyId: trade.strategyId,
      strategyName: trade.strategyName,
      side: trade.side,
      price: trade.status === "CLOSED" ? (trade.exitPrice ?? undefined) : trade.entryPrice,
      pnl: trade.status === "CLOSED" ? trade.realizedPnl : undefined,
      regime: trade.regimeAtEntry,
    });
  }

  // ── Signal ─────────────────────────────────────────────────────────────

  persistSignal(signal: SignalRecord, accountKey: string): void {
    this.enqueue("signal:" + signal.strategyId + "-" + signal.timestamp, async () => {
      await appendNdjson(
        todayFilePath("signals", "signals.ndjson"),
        { ...signal, account_key: accountKey, persisted_at: new Date().toISOString() },
      );
      await appendCsv(
        todayFilePath("signals", "signals.csv"),
        ["strategy_id", "strategy_name", "strategy_family", "side", "timestamp", "entry_price", "confidence_score", "market_regime", "risk_reward", "entry_reason"],
        [signal.strategyId, signal.strategyName, signal.strategyFamily, signal.side, new Date(signal.timestamp).toISOString(), signal.entryPrice, signal.confidenceScore, signal.marketRegime ?? "", signal.riskReward ?? "", signal.entryReason],
      );
    });
    this.emitAuditEvent({
      type: "SIGNAL_GENERATED",
      timestamp: new Date().toISOString(),
      accountKey,
      strategyId: signal.strategyId,
      strategyName: signal.strategyName,
      side: signal.side,
      price: signal.entryPrice,
      regime: signal.marketRegime,
    });
  }

  persistSignalRejection(signal: Partial<SignalRecord>, reason: string, accountKey: string): void {
    this.enqueue("signal-reject:" + (signal.strategyId ?? "unknown"), async () => {
      await appendNdjson(
        todayFilePath("risk", "rejections.ndjson"),
        { ...signal, rejection_reason: reason, account_key: accountKey, persisted_at: new Date().toISOString() },
      );
    });
    this.emitAuditEvent({
      type: "SIGNAL_REJECTED",
      timestamp: new Date().toISOString(),
      accountKey,
      strategyId: signal.strategyId,
      message: reason,
      regime: signal.marketRegime,
    });
  }

  // ── Regime Snapshot ────────────────────────────────────────────────────

  persistRegimeSnapshot(snapshot: RegimeSnapshot, accountKey: string): void {
    this.enqueue("regime:" + snapshot.timestamp, async () => {
      await appendNdjson(
        todayFilePath("equity", "regimes.ndjson"),
        { ...snapshot, account_key: accountKey, persisted_at: new Date().toISOString() },
      );
    });
  }

  // ── Strategy Scores ────────────────────────────────────────────────────

  persistStrategyScores(scores: StrategyScore[], accountKey: string): void {
    this.enqueue("strategy-scores:" + Date.now(), async () => {
      const timestamp = new Date().toISOString();
      const dayKey = timestamp.slice(0, 10);
      await atomicWriteJson(
        datePath("metrics", `${dayKey}-strategy-scores.json`),
        { timestamp, account_key: accountKey, scores },
      );
      await appendNdjson(
        todayFilePath("metrics", "score-history.ndjson"),
        { timestamp, account_key: accountKey, topScore: scores[0] ?? null, count: scores.length },
      );
    });
  }

  // ── Equity Curve ───────────────────────────────────────────────────────

  persistEquityCurvePoint(point: {
    timestamp: number;
    equity: number;
    realizedPnl: number;
    unrealizedPnl: number;
    drawdownPct: number;
    regime?: string;
  }, accountKey: string): void {
    this.enqueue("equity:" + point.timestamp, async () => {
      await appendNdjson(
        todayFilePath("equity", "curve.ndjson"),
        { ...point, account_key: accountKey, persisted_at: new Date().toISOString() },
      );
      await appendCsv(
        todayFilePath("equity", "curve.csv"),
        ["timestamp", "equity", "realized_pnl", "unrealized_pnl", "drawdown_pct", "regime", "account_key"],
        [new Date(point.timestamp).toISOString(), point.equity, point.realizedPnl, point.unrealizedPnl, point.drawdownPct, point.regime ?? "", accountKey],
      );
    });
  }

  // ── Account Snapshot ───────────────────────────────────────────────────

  persistAccountSnapshot(state: MockAccountState, config: MockTradingConfig, accountKey: string): void {
    this.enqueue("account-snapshot:" + Date.now(), async () => {
      const timestamp = new Date().toISOString();
      await appendNdjson(
        todayFilePath("equity", "account-snapshots.ndjson"),
        { ...state, config, account_key: accountKey, persisted_at: timestamp },
      );
    });
  }

  // ── Analytics ──────────────────────────────────────────────────────────

  persistStrategyAnalytics(analytics: MockTradeAnalytics, accountKey: string): void {
    this.enqueue("analytics:" + Date.now(), async () => {
      const dayKey = new Date().toISOString().slice(0, 10);
      await atomicWriteJson(
        datePath("metrics", `${dayKey}-strategy-analytics.json`),
        { generated_at: new Date().toISOString(), account_key: accountKey, analytics },
      );
    });
  }

  // ── Config Snapshot ────────────────────────────────────────────────────

  persistConfigSnapshot(config: MockTradingConfig, accountKey: string): void {
    this.enqueue("config-snapshot:" + Date.now(), async () => {
      const timestamp = new Date().toISOString();
      await appendNdjson(
        datePath("configs", "config-history.ndjson"),
        { config, account_key: accountKey, changed_at: timestamp },
      );
      await atomicWriteJson(
        datePath("configs", `latest-config-${accountKey}.json`),
        { config, account_key: accountKey, changed_at: timestamp },
      );
    });
    this.emitAuditEvent({ type: "CONFIG_CHANGED", timestamp: new Date().toISOString(), accountKey, payload: config as unknown as Record<string, unknown> });
  }

  // ── Risk rejection ─────────────────────────────────────────────────────

  persistRiskRejection(details: { reason: string; strategyId?: number; side?: string; price?: number; regime?: string }, accountKey: string): void {
    this.enqueue("risk-reject:" + Date.now(), async () => {
      await appendNdjson(
        todayFilePath("risk", "risk-decisions.ndjson"),
        { ...details, account_key: accountKey, persisted_at: new Date().toISOString() },
      );
    });
    this.emitAuditEvent({
      type: "RISK_REJECTED",
      timestamp: new Date().toISOString(),
      accountKey,
      strategyId: details.strategyId,
      side: details.side,
      message: details.reason,
      regime: details.regime,
    });
  }

  // ── Audit event ────────────────────────────────────────────────────────

  emitAuditEvent(event: AuditEvent): void {
    this.enqueue("audit:" + event.type, async () => {
      await appendNdjson(
        todayFilePath("audit", "events.ndjson"),
        event,
      );
    });
  }

  // ── Storage health ─────────────────────────────────────────────────────

  async getStorageHealth(): Promise<LocalStorageHealth> {
    const root = getDataRoot();
    const auditDir = path.join(root, "audit");
    const backupDir = path.join(root, "backups");

    const [totalBytes, auditStats] = await Promise.all([
      totalStorageBytes(),
      dirStats(auditDir),
    ]);

    // Count backup files across periods
    let backupCount = 0;
    let lastBackupAt: string | null = null;
    for (const period of ["daily", "weekly", "monthly"] as const) {
      const dir = path.join(backupDir, period);
      try {
        const entries = (await fs.promises.readdir(dir)).filter((f) => f.endsWith(".json.gz"));
        backupCount += entries.length;
        if (entries.length > 0) {
          const newest = entries.sort().reverse()[0];
          const match = newest.match(/(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2})/);
          if (match) {
            const ts = match[1].replace(/T(\d{2})-(\d{2})-(\d{2})/, "T$1:$2:$3");
            if (!lastBackupAt || ts > lastBackupAt) lastBackupAt = ts;
          }
        }
      } catch {
        /* ignore */
      }
    }

    const queueStats = writeQueue.getStats();

    return {
      ok: true,
      dataRoot: root,
      totalBytes,
      totalMb: Math.round(totalBytes / 1024 / 1024),
      filesWrittenToday: this.filesWrittenToday,
      backupCount,
      lastBackupAt,
      auditFileCount: auditStats.fileCount,
      writeFailures: this.writeFailures.length,
      recentFailures: this.writeFailures.slice(-5),
      queueStats,
      uptimeMs: Date.now() - this.startedAt,
    };
  }

  getWriteFailures() {
    return [...this.writeFailures];
  }

  // ── Internal helpers ───────────────────────────────────────────────────

  private enqueue(label: string, fn: () => Promise<void>): void {
    this.filesWrittenToday++;
    writeQueue.enqueue(label, async () => {
      try {
        await fn();
      } catch (err) {
        const errorMsg = err instanceof Error ? err.message : String(err);
        this.writeFailures.push({ label, error: errorMsg, ts: new Date().toISOString() });
        // Keep last 100 failures in memory
        if (this.writeFailures.length > 100) this.writeFailures.shift();
        throw err; // Let writeQueue handle retry
      }
    });
  }
}

// ── Health type ────────────────────────────────────────────────────────────

export interface LocalStorageHealth {
  ok: boolean;
  dataRoot: string;
  totalBytes: number;
  totalMb: number;
  filesWrittenToday: number;
  backupCount: number;
  lastBackupAt: string | null;
  auditFileCount: number;
  writeFailures: number;
  recentFailures: { label: string; error: string; ts: string }[];
  queueStats: ReturnType<typeof writeQueue.getStats>;
  uptimeMs: number;
}

// ── Trade helpers ──────────────────────────────────────────────────────────

function flattenTrade(trade: MockTrade): Record<string, unknown> {
  return {
    trade_id: trade.id,
    strategy_id: trade.strategyId,
    strategy_name: trade.strategyName,
    strategy_family: trade.strategyFamily,
    symbol: trade.symbol ?? "BTCUSDT",
    side: trade.side,
    status: trade.status,
    opened_at: trade.openedAt,
    closed_at: trade.closedAt ?? null,
    entry_price: trade.entryPrice,
    close_price: trade.exitPrice ?? null,
    take_profit_price: trade.takeProfitPrice,
    stop_loss_price: trade.stopLossPrice,
    quantity: trade.quantity,
    notional: trade.notional,
    leverage: trade.leverage,
    margin_used: trade.marginUsed,
    realized_pnl: trade.realizedPnl,
    unrealized_pnl: trade.unrealizedPnl,
    fees: trade.fees,
    funding_costs: trade.fundingCosts,
    risk_reward_ratio: trade.riskRewardRatio,
    confidence_score: trade.confidenceScore,
    exit_reason: trade.exitReason ?? null,
    regime_at_entry: trade.regimeAtEntry ?? null,
  };
}

function tradeToRow(trade: MockTrade): (string | number | null)[] {
  return [
    trade.id,
    new Date(trade.openedAt).toISOString(),
    trade.closedAt ? new Date(trade.closedAt).toISOString() : "",
    trade.strategyId,
    trade.strategyName,
    trade.symbol ?? "BTCUSDT",
    trade.side,
    trade.status,
    trade.entryPrice,
    trade.exitPrice ?? "",
    trade.quantity,
    trade.realizedPnl,
    trade.fees,
    trade.fundingCosts ?? 0,
    trade.riskRewardRatio ?? 0,
    trade.confidenceScore ?? 0,
    trade.exitReason ?? "",
    trade.regimeAtEntry ?? "",
  ];
}

// ── Singleton export ───────────────────────────────────────────────────────

export const dps = new DualPersistenceService();
