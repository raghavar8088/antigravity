"use client";

import { type CSSProperties, type ReactNode, useEffect, useMemo, useState } from "react";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import { useMockTradingEngine } from "@/hooks/useMockTradingEngine";
import {
  computeAnalytics,
  type MockTrade,
} from "@/lib/trading/mockTradingEngine";
import { rankStrategies, type StrategyRankRow } from "@/lib/ai/mockStrategyRankingEngine";
import {
  getMockConfigForPipelineStage,
  mockAccountKeyForStage,
} from "@/lib/strategyAuthority/gradeStageMockConfig";
import type { StrategyStatus } from "@/lib/strategyAuthority/types";
import { Sparkline } from "@/components/ui/Sparkline";
import { SkeletonBlock } from "@/components/ui/EmptyState";
import { TerminalCard } from "./TerminalCard";

const TRADE_ENGINE_TITLE = "Trade Engine";

const CLOSED_TRADE_INITIAL_LIMIT = 50;
const CLOSED_TRADE_LIMIT_STEP = 50;
const LEADERBOARD_LIMIT = 100;
const SCALERS_REGIME_POLL_MS = 30_000;
const SCALERS_SIGNAL_POLL_MS = 15_000;
const MAX_LIVE_SIGNALS = 10;

type ScalersSignal = {
  id: string;
  strategy: string;
  side: string;
  confidence: number;
  timestamp: string | null;
  price: number | null;
};

type ScalersStrategyStat = {
  id: string;
  name: string;
  trades: number | null;
  winRatePct: number | null;
};

type ScalersStatsState = {
  regime: string;
  signals: ScalersSignal[];
  strategyStats: ScalersStrategyStat[];
};

const UNKNOWN_SCALERS_STATS: ScalersStatsState = {
  regime: "UNKNOWN",
  signals: [],
  strategyStats: [],
};

type MockStageLayoutSections = {
  summaryStrip: ReactNode;
  leftPanel: ReactNode;
  rightPanel: ReactNode;
};

type MockStageTradingSuiteProps = {
  status: StrategyStatus;
  showPipeline?: boolean;
  renderShell?: (sections: MockStageLayoutSections) => ReactNode;
};

const tableStyle: CSSProperties = {
  width: "100%",
  borderCollapse: "separate",
  borderSpacing: 0,
  fontSize: 13,
  minWidth: 820,
};

const thStyle: CSSProperties = {
  padding: "12px 12px",
  fontSize: 11,
  fontWeight: 700,
  textTransform: "uppercase",
  letterSpacing: "0.06em",
  color: "var(--text-muted)",
  borderBottom: "1px solid var(--border-subtle, var(--border))",
  background: "var(--surface-2)",
  whiteSpace: "nowrap",
};

const tdStyle: CSSProperties = {
  padding: "13px 12px",
  verticalAlign: "middle",
  color: "var(--text-secondary)",
  borderBottom: "1px solid var(--border-subtle, var(--border))",
};

const monoCellStyle: CSSProperties = {
  fontFamily: "var(--font-mono)",
  fontVariantNumeric: "tabular-nums",
  whiteSpace: "nowrap",
};

function rowBackground(index: number) {
  return index % 2 === 1 ? "#fbfdff" : "#ffffff";
}

function fmtUsd(value: number) {
  if (!Number.isFinite(value)) return "—";
  const sign = value > 0 ? "+" : value < 0 ? "-" : "";
  return `${sign}$${Math.abs(value).toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}

function fmtPct(value: number) {
  if (!Number.isFinite(value)) return "—";
  return `${value.toFixed(1)}%`;
}

function fmtPrice(value: number) {
  if (!Number.isFinite(value)) return "—";
  return `$${value.toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}

function fmtSignedPct(value: number) {
  if (!Number.isFinite(value)) return "—";
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(2)}%`;
}

function fmtSize(trade: MockTrade) {
  if (!Number.isFinite(trade.quantity)) return "—";
  const decimals = Math.abs(trade.quantity) >= 1 ? 3 : 5;
  return `${trade.quantity.toFixed(decimals)} BTC`;
}

function fmtAge(startTs: number | null, endTs: number) {
  if (startTs == null || !Number.isFinite(startTs) || !Number.isFinite(endTs)) return "—";
  const totalMinutes = Math.max(0, Math.floor((endTs - startTs) / 60_000));
  if (totalMinutes < 60) return `${totalMinutes}m`;
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return `${hours}h ${minutes}m`;
}

function pnlColor(value: number) {
  if (value > 0) return "var(--green)";
  if (value < 0) return "var(--red)";
  return "var(--text-muted)";
}

function sideColor(side: MockTrade["side"]) {
  return side === "BUY" ? "var(--green)" : "var(--red)";
}

function signalSideColor(side: string) {
  if (side === "BUY") return "var(--green)";
  if (side === "SELL") return "var(--red)";
  return "var(--text-muted)";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function numberValue(value: unknown): number | null {
  const n = typeof value === "number" ? value : typeof value === "string" ? Number(value) : Number.NaN;
  return Number.isFinite(n) ? n : null;
}

function stringValue(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value.trim() : null;
}

function pickString(record: Record<string, unknown>, keys: string[]): string | null {
  for (const key of keys) {
    const value = stringValue(record[key]);
    if (value) return value;
  }
  return null;
}

function pickNumber(record: Record<string, unknown>, keys: string[]): number | null {
  for (const key of keys) {
    const value = numberValue(record[key]);
    if (value != null) return value;
  }
  return null;
}

function normalizeRegime(value: unknown): string {
  const regime = stringValue(value)?.toUpperCase();
  if (regime === "TRENDING" || regime === "RANGING" || regime === "VOLATILE") return regime;
  return regime || "UNKNOWN";
}

function normalizeSide(value: unknown): string {
  const side = stringValue(value)?.toUpperCase();
  if (side === "LONG") return "BUY";
  if (side === "SHORT") return "SELL";
  return side || "—";
}

function normalizeConfidence(value: unknown): number {
  const raw = numberValue(value) ?? 0;
  const normalized = raw > 1 && raw <= 100 ? raw / 100 : raw;
  return Math.max(0, Math.min(1, normalized));
}

function normalizeTimestamp(value: unknown): string | null {
  if (typeof value === "number" && Number.isFinite(value)) {
    const ms = value < 1_000_000_000_000 ? value * 1000 : value;
    return new Date(ms).toISOString();
  }
  const text = stringValue(value);
  if (!text) return null;
  const parsed = Date.parse(text);
  return Number.isFinite(parsed) ? new Date(parsed).toISOString() : text;
}

function firstArray(...values: unknown[]): unknown[] {
  for (const value of values) {
    if (Array.isArray(value)) return value;
  }
  return [];
}

function firstDefined(...values: unknown[]): unknown {
  return values.find((value) => value != null);
}

function parseSignal(value: unknown, index: number): ScalersSignal | null {
  if (!isRecord(value)) return null;
  const strategy = pickString(value, ["strategyName", "strategy_name", "strategy", "name"]) ?? "UnknownStrategy";
  const side = normalizeSide(firstDefined(value.side, value.action, value.signal, value.direction));
  const confidence = normalizeConfidence(firstDefined(value.confidence, value.conf, value.score, value.probability));
  const timestamp = normalizeTimestamp(firstDefined(value.timestamp, value.time, value.createdAt, value.created_at, value.ts));
  const price = pickNumber(value, ["price", "markPrice", "mark_price", "entryPrice", "entry_price"]);
  const id = pickString(value, ["id", "signalId", "signal_id"]) ?? `${strategy}-${timestamp ?? index}`;

  return { id, strategy, side, confidence, timestamp, price };
}

function winRatePct(value: number | null): number | null {
  if (value == null) return null;
  return value <= 1 ? value * 100 : value;
}

function parseStrategyStat(value: unknown, index: number, fallbackName?: string): ScalersStrategyStat | null {
  if (!isRecord(value)) {
    const name = fallbackName ?? stringValue(value);
    return name ? { id: `${name}-${index}`, name, trades: null, winRatePct: null } : null;
  }

  const name = pickString(value, ["strategyName", "strategy_name", "strategy", "name"]) ?? fallbackName ?? `Strategy ${index + 1}`;
  const trades = pickNumber(value, ["trades", "tradeCount", "trade_count", "total", "closedTrades", "closed_trades"]);
  const wr = winRatePct(pickNumber(value, ["winRatePct", "win_rate_pct", "winRate", "win_rate", "wr"]));
  const id = pickString(value, ["id", "strategyId", "strategy_id"]) ?? name;

  return { id, name, trades, winRatePct: wr };
}

function parseStrategyStats(value: unknown): ScalersStrategyStat[] {
  const rows = Array.isArray(value)
    ? value.map((row, index) => parseStrategyStat(row, index))
    : isRecord(value)
      ? Object.entries(value).map(([name, row], index) => parseStrategyStat(row, index, name))
      : [];

  return rows
    .filter((row): row is ScalersStrategyStat => row != null)
    .sort((a, b) => (b.trades ?? 0) - (a.trades ?? 0))
    .slice(0, MAX_LIVE_SIGNALS);
}

function parseScalersStats(payload: unknown): ScalersStatsState {
  const root = isRecord(payload) ? payload : {};
  const data = isRecord(root.data) ? root.data : {};
  const stats = isRecord(root.stats) ? root.stats : {};
  const regime = normalizeRegime(
    firstDefined(root.regime, root.marketRegime, root.market_regime, data.regime, data.marketRegime, stats.regime),
  );
  const signals = firstArray(root.signals, root.recentSignals, root.recent_signals, data.signals, data.recentSignals, data.recent_signals, stats.signals)
    .map((signal, index) => parseSignal(signal, index))
    .filter((signal): signal is ScalersSignal => signal != null)
    .slice(0, MAX_LIVE_SIGNALS);
  const strategyStats = parseStrategyStats(
    firstDefined(root.perStrategy, root.per_strategy, root.strategyStats, root.strategies, data.perStrategy, data.strategies, stats.perStrategy),
  );

  return { regime, signals, strategyStats };
}

function formatSignalTime(timestamp: string | null) {
  if (!timestamp) return "— UTC";
  const parsed = Date.parse(timestamp);
  if (!Number.isFinite(parsed)) return `${timestamp} UTC`;
  return new Intl.DateTimeFormat("en-US", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
    timeZone: "UTC",
  }).format(parsed) + " UTC";
}

function buildEquitySparklineValues(
  trades: readonly MockTrade[],
  currentEquity: number,
  realizedPnl: number,
  unrealizedPnl: number,
) {
  const closed = trades
    .filter((trade) => trade.status === "CLOSED" && Number.isFinite(trade.realizedPnl))
    .sort((a, b) => (a.closedAt ?? a.openedAt) - (b.closedAt ?? b.openedAt));
  if (closed.length === 0 || !Number.isFinite(currentEquity)) return [];

  let equity = currentEquity - realizedPnl - unrealizedPnl;
  const values = [equity];
  for (const trade of closed) {
    equity += trade.realizedPnl;
    values.push(equity);
  }
  if (values[values.length - 1] !== currentEquity) values.push(currentEquity);
  return values;
}

function TableEmptyState({ label }: { label: string }) {
  return (
    <div className="google-empty-state" role="status">
      <p className="m3-empty-state__title">{label}</p>
      <p className="m3-empty-state__subtitle">This section updates automatically when Trade Engine activity is available.</p>
    </div>
  );
}

function TerminalEmptyState({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div className="m3-terminal-empty-state">
      <svg className="m3-terminal-empty-state__icon" viewBox="0 0 32 32" fill="none" aria-hidden>
        <circle cx="16" cy="16" r="12" stroke="currentColor" strokeWidth="1.5" />
        <path d="M11 16h10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
      <div className="m3-terminal-empty-state__title">{title}</div>
      <div className="m3-terminal-empty-state__subtitle">{subtitle}</div>
    </div>
  );
}

export function RegimeBadge({ regime }: { regime: string }) {
  const cfg = (
    {
      TRENDING: { color: "var(--green)", bg: "var(--green-dim)", icon: "▲", label: "TRENDING" },
      RANGING: { color: "var(--info)", bg: "var(--info-dim)", icon: "↔", label: "RANGING" },
      VOLATILE: { color: "var(--amber)", bg: "var(--amber-dim)", icon: "⚡", label: "VOLATILE" },
      LIVE: { color: "var(--green)", bg: "var(--green-dim)", icon: "●", label: "LIVE" },
      UPDATING: { color: "var(--amber)", bg: "var(--amber-dim)", icon: "●", label: "UPDATING" },
      ATTENTION: { color: "var(--red)", bg: "var(--red-dim)", icon: "●", label: "ATTENTION" },
      UNKNOWN: { color: "var(--text-muted)", bg: "var(--surface-3)", icon: "?", label: "UNKNOWN" },
    } as Record<string, { color: string; bg: string; icon: string; label: string }>
  )[regime] ?? { color: "var(--text-muted)", bg: "var(--surface-3)", icon: "?", label: regime };

  return (
    <div
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 6,
        padding: "4px 10px",
        borderRadius: 999,
        background: cfg.bg,
        border: "1px solid var(--card-border)",
        fontSize: 12,
        fontWeight: 700,
        letterSpacing: "0.06em",
        color: cfg.color,
        textTransform: "uppercase",
      }}
    >
      <span>{cfg.icon}</span>
      <span>{cfg.label}</span>
    </div>
  );
}

function PositionsTableSkeleton() {
  return (
    <div style={{ overflowX: "auto" }} aria-busy="true" aria-label="Loading open positions">
      <table style={tableStyle}>
        <thead>
          <tr>
            {["STRATEGY", "SIDE", "SIZE", "ENTRY", "MARK", "UNREALIZED PnL", "SL", "TP", "AGE"].map((header, index) => (
              <th key={header} style={{ ...thStyle, textAlign: index === 0 || index === 1 ? "left" : "right" }}>
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {Array.from({ length: 3 }).map((_, rowIndex) => (
            <tr
              key={rowIndex}
              style={{
                background: rowBackground(rowIndex),
                borderBottom: 0,
              }}
            >
              {Array.from({ length: 9 }).map((__, cellIndex) => (
                <td key={cellIndex} style={{ ...tdStyle, textAlign: cellIndex <= 1 ? "left" : "right" }}>
                  <SkeletonBlock width={cellIndex === 0 ? 148 : cellIndex === 1 ? 48 : 68} height={12} rounded={3} />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function KpiStripSkeleton() {
  return (
    <>
      {Array.from({ length: 6 }).map((_, index) => (
        <div key={index} className="m3-stage-stat" style={{ borderTopColor: "var(--border)" }}>
          <SkeletonBlock width="56%" height={10} rounded={3} />
          <div style={{ marginTop: 9 }}>
            <SkeletonBlock width={index === 4 ? 96 : 82} height={24} rounded={4} />
          </div>
          <div style={{ marginTop: 8 }}>
            <SkeletonBlock width="72%" height={8} rounded={3} />
          </div>
        </div>
      ))}
    </>
  );
}

function LiveSignalsPanel({ stats }: { stats: ScalersStatsState }) {
  if (stats.signals.length > 0) {
    return (
      <div style={{ display: "grid", gap: 8 }}>
        {stats.signals.map((signal) => {
          const color = signalSideColor(signal.side);
          const fillWidth = Math.round(signal.confidence * 40);
          return (
            <article
              key={signal.id}
              style={{
                border: "1px solid var(--border-subtle, var(--border))",
                borderRadius: 8,
                background: "var(--surface-2)",
                padding: "8px 9px",
                transition: "all 0.3s ease",
                transform: "translateY(0)",
              }}
            >
              <div style={{ display: "flex", alignItems: "center", gap: 8, minWidth: 0 }}>
                <span style={{ width: 7, height: 7, borderRadius: 999, background: color, flex: "0 0 auto" }} />
                <span
                  style={{
                    color: "var(--text-primary)",
                    fontSize: 12,
                    fontWeight: 700,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {signal.strategy}
                </span>
                <span style={{ color, fontSize: 11, fontWeight: 800, marginLeft: "auto" }}>{signal.side}</span>
                <span
                  style={{
                    width: 40,
                    height: 4,
                    borderRadius: 2,
                    background: "var(--surface-3)",
                    overflow: "hidden",
                    flex: "0 0 auto",
                  }}
                >
                  <span style={{ display: "block", width: fillWidth, height: "100%", background: color }} />
                </span>
                <span style={{ color: "var(--text-primary)", fontFamily: "var(--font-mono)", fontSize: 12 }}>
                  {signal.confidence.toFixed(2)}
                </span>
              </div>
              <div
                style={{
                  marginTop: 5,
                  paddingLeft: 15,
                  color: "var(--text-muted)",
                  fontFamily: "var(--font-mono)",
                  fontSize: 10,
                }}
              >
                {formatSignalTime(signal.timestamp)} {signal.price != null ? `  ${fmtPrice(signal.price)}` : ""}
              </div>
            </article>
          );
        })}
      </div>
    );
  }

  if (stats.strategyStats.length > 0) {
    return (
      <div style={{ display: "grid", gap: 7 }}>
        {stats.strategyStats.map((row, index) => (
          <div
            key={row.id}
            style={{
              display: "grid",
              gridTemplateColumns: "auto 1fr",
              gap: 8,
              alignItems: "center",
              borderRadius: 8,
              background: index % 2 === 0 ? "var(--surface)" : "var(--surface-2)",
              border: "1px solid var(--border-subtle, var(--border))",
              padding: "7px 8px",
              color: "var(--text-secondary)",
              fontSize: 12,
            }}
          >
            <span style={{ color: "var(--text-muted)", fontFamily: "var(--font-mono)", fontSize: 10 }}>
              #{index + 1}
            </span>
            <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              <span style={{ color: "var(--text-primary)", fontWeight: 700 }}>{row.name}</span> ·{" "}
              {row.trades != null ? row.trades.toLocaleString() : "—"} trades · WR{" "}
              {row.winRatePct != null ? `${row.winRatePct.toFixed(0)}%` : "—"}
            </span>
          </div>
        ))}
      </div>
    );
  }

      return <TerminalEmptyState title="No live signals" subtitle="Waiting for live signal telemetry." />;
}

function OpenPositionsTable({ trades, nowMs, loading = false }: { trades: MockTrade[]; nowMs: number; loading?: boolean }) {
  if (loading) {
    return <PositionsTableSkeleton />;
  }

  if (trades.length === 0) {
    return <TableEmptyState label="No open positions" />;
  }

  return (
    <div style={{ overflowX: "auto" }}>
      <table style={tableStyle}>
        <thead>
          <tr>
            {["STRATEGY", "SIDE", "SIZE", "ENTRY", "MARK", "UNREALIZED PnL", "SL", "TP", "AGE"].map((header, index) => (
              <th key={header} style={{ ...thStyle, textAlign: index === 0 || index === 1 ? "left" : "right" }}>
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {trades.map((trade, index) => {
            const baseBg = rowBackground(index);
            return (
              <tr
                key={trade.id}
                style={{
                  background: baseBg,
                  borderBottom: 0,
                  transition: "background 120ms ease",
                }}
                onMouseEnter={(event) => {
                  event.currentTarget.style.background = "#f8fbff";
                }}
                onMouseLeave={(event) => {
                  event.currentTarget.style.background = baseBg;
                }}
              >
                <td style={{ ...tdStyle, maxWidth: 210, color: "var(--text-primary)", fontWeight: 600 }}>
                  <div style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {trade.strategyName}
                  </div>
                </td>
                <td style={{ ...tdStyle, color: sideColor(trade.side), fontWeight: 700 }}>
                  {trade.side}
                </td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtSize(trade)}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(trade.entryPrice)}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(trade.currentPrice)}</td>
                <td
                  style={{
                    ...tdStyle,
                    ...monoCellStyle,
                    textAlign: "right",
                    color: pnlColor(trade.unrealizedPnl),
                    fontWeight: 600,
                  }}
                >
                  {fmtUsd(trade.unrealizedPnl)}
                </td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(trade.stopLossPrice)}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(trade.takeProfitPrice)}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtAge(trade.openedAt, nowMs)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

const REASON_CHIP: Record<string, { label: string; background: string; color: string }> = {
  TAKE_PROFIT: { label: "TP", background: "var(--green-dim)", color: "var(--green)" },
  TP: { label: "TP", background: "var(--green-dim)", color: "var(--green)" },
  STOP_LOSS: { label: "SL", background: "var(--red-dim)", color: "var(--red)" },
  SL: { label: "SL", background: "var(--red-dim)", color: "var(--red)" },
  MAX_HOLD: { label: "TIME", background: "var(--amber-dim)", color: "var(--amber)" },
  TIME: { label: "TIME", background: "var(--amber-dim)", color: "var(--amber)" },
  TRAIL: { label: "TRAIL", background: "var(--info-dim)", color: "var(--info)" },
};

function ReasonChip({ reason }: { reason: MockTrade["exitReason"] }) {
  const config = reason ? REASON_CHIP[reason] : null;
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        borderRadius: 999,
        background: config?.background ?? "var(--surface-2)",
        color: config?.color ?? "var(--text-muted)",
        fontSize: 10,
        fontWeight: 700,
        letterSpacing: "0.04em",
        padding: "2px 7px",
        textTransform: "uppercase",
        whiteSpace: "nowrap",
      }}
    >
      {config?.label ?? reason ?? "—"}
    </span>
  );
}

function ClosedTradesTable({
  trades,
  analytics,
  visibleLimit,
  nowMs,
  onShowMore,
}: {
  trades: MockTrade[];
  analytics: ReturnType<typeof computeAnalytics>;
  visibleLimit: number;
  nowMs: number;
  onShowMore: () => void;
}) {
  const visibleTrades = trades.slice(0, visibleLimit);
  const hasMore = trades.length > visibleTrades.length;

  return (
    <div>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          gap: 12,
          marginBottom: 10,
          color: "var(--text-muted)",
          fontSize: 11,
        }}
      >
        <span>
          {analytics.closedTrades.toLocaleString()} trades · Win rate {fmtPct(analytics.winRate * 100)} · Total PnL{" "}
          {fmtUsd(analytics.realizedPnl)}
        </span>
        {hasMore ? (
          <button
            type="button"
            onClick={onShowMore}
            style={{
              border: 0,
              background: "transparent",
              color: "var(--accent)",
              cursor: "pointer",
              fontSize: 11,
              fontWeight: 700,
              padding: 0,
              textDecoration: "underline",
            }}
          >
            Show more
          </button>
        ) : null}
      </div>
      {visibleTrades.length === 0 ? (
        <TableEmptyState label="No closed trades" />
      ) : (
        <div style={{ overflowX: "auto" }}>
          <table style={tableStyle}>
            <thead>
              <tr>
                {["STRATEGY", "SIDE", "ENTRY", "EXIT", "PnL", "REASON", "DURATION"].map((header, index) => (
                  <th key={header} style={{ ...thStyle, textAlign: index === 0 || index === 1 || index === 5 ? "left" : "right" }}>
                    {header}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {visibleTrades.map((trade, index) => {
                const baseBg = rowBackground(index);
                return (
                  <tr
                    key={trade.id}
                    style={{
                      background: baseBg,
                      borderBottom: 0,
                      transition: "background 120ms ease",
                    }}
                    onMouseEnter={(event) => {
                      event.currentTarget.style.background = "#f8fbff";
                    }}
                    onMouseLeave={(event) => {
                      event.currentTarget.style.background = baseBg;
                    }}
                  >
                    <td style={{ ...tdStyle, maxWidth: 210, color: "var(--text-primary)", fontWeight: 600 }}>
                      <div style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                        {trade.strategyName}
                      </div>
                    </td>
                    <td style={{ ...tdStyle, color: sideColor(trade.side), fontWeight: 700 }}>{trade.side}</td>
                    <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(trade.entryPrice)}</td>
                    <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>
                      {trade.exitPrice != null ? fmtPrice(trade.exitPrice) : "—"}
                    </td>
                    <td
                      style={{
                        ...tdStyle,
                        ...monoCellStyle,
                        textAlign: "right",
                        color: pnlColor(trade.realizedPnl),
                        fontWeight: 700,
                      }}
                    >
                      {fmtUsd(trade.realizedPnl)}
                    </td>
                    <td style={tdStyle}>
                      <ReasonChip reason={trade.exitReason} />
                    </td>
                    <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>
                      {fmtAge(trade.openedAt, trade.closedAt ?? nowMs)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function rankBadgeColor(rank: number) {
  if (rank === 1) return "var(--gold)";
  if (rank === 2) return "#c0c0c0";
  if (rank === 3) return "#cd7f32";
  return "var(--text-muted)";
}

function LeaderboardTable({ rows }: { rows: StrategyRankRow[] }) {
  if (rows.length === 0) {
    return (
      <TerminalEmptyState
        title="No Strategy History"
        subtitle="Strategy scores appear after the engine records closed-trade history."
      />
    );
  }

  return (
    <div style={{ display: "grid", gap: 8 }}>
      {rows.map((row) => {
        const scorePct = Math.max(0, Math.min(100, row.score));
        return (
          <article
            key={row.strategyId}
            style={{
              border: "1px solid var(--border-subtle, var(--border))",
              borderRadius: 10,
                background: "#ffffff",
              padding: "10px 11px",
            }}
          >
            <div style={{ display: "grid", gridTemplateColumns: "auto 1fr auto", alignItems: "center", gap: 8 }}>
              <span
                style={{
                  color: rankBadgeColor(row.rank),
                  fontFamily: "var(--font-mono)",
                  fontSize: 12,
                  fontWeight: 800,
                }}
              >
                #{row.rank}
              </span>
              <div
                style={{
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  color: "var(--text-primary)",
                  fontSize: 13,
                  fontWeight: 700,
                }}
              >
                {row.strategyName}
              </div>
              <span
                style={{
                  color: "var(--text-primary)",
                  fontFamily: "var(--font-mono)",
                  fontSize: 14,
                  fontWeight: 700,
                }}
              >
                {Math.round(row.score)}
              </span>
            </div>
            <div style={{ height: 4, borderRadius: 2, background: "var(--surface-3)", marginTop: 8, overflow: "hidden" }}>
              <div
                style={{
                  width: `${scorePct}%`,
                  height: "100%",
                  borderRadius: 2,
                  background: "linear-gradient(90deg, var(--accent), var(--accent-strong))",
                }}
              />
            </div>
            <div style={{ marginTop: 7, color: "var(--text-muted)", fontSize: 11 }}>
              WR: {fmtPct(row.winRate * 100)} · PF: {row.profitFactor != null ? row.profitFactor.toFixed(2) : "—"} · PnL:{" "}
              <span style={{ color: pnlColor(row.netPnl), fontFamily: "var(--font-mono)", fontWeight: 700 }}>
                {fmtUsd(row.netPnl)}
              </span>
            </div>
          </article>
        );
      })}
    </div>
  );
}

function StageStat({
  label,
  value,
  accent,
  tone = "neutral",
  size = "md",
  children,
}: {
  label: string;
  value: string;
  accent: string;
  tone?: "neutral" | "positive" | "negative" | "warning";
  size?: "sm" | "md" | "lg";
  children?: ReactNode;
}) {
  const toneClass =
    tone === "positive"
      ? "m3-stage-stat__value--positive"
      : tone === "negative"
      ? "m3-stage-stat__value--negative"
      : tone === "warning"
      ? "m3-stage-stat__value--warning"
      : "";

  return (
    <div className="m3-stage-stat" style={{ borderTopColor: accent }}>
      <div className="m3-stage-stat__label">{label}</div>
      <div className={["m3-stage-stat__value", `m3-stage-stat__value--${size}`, toneClass].filter(Boolean).join(" ")}>
        {value}
      </div>
      {children ? <div style={{ marginTop: 6, minHeight: 28, display: "flex", alignItems: "end" }}>{children}</div> : null}
    </div>
  );
}

export function MockStageTradingSuite({
  status,
  showPipeline = false,
  renderShell,
}: MockStageTradingSuiteProps) {
  const live = useLiveBTCPrice();
  const [closedVisibleLimit, setClosedVisibleLimit] = useState(CLOSED_TRADE_INITIAL_LIMIT);
  const [nowMs, setNowMs] = useState(() => Date.now());
  const [scalersStats, setScalersStats] = useState<ScalersStatsState>(UNKNOWN_SCALERS_STATS);
  const engine = useMockTradingEngine({
    price: live.price,
    accountKey: mockAccountKeyForStage(status),
    pipelineStage: status === "RETIRED" ? null : "MAIN_ENGINE",
    initialConfig: getMockConfigForPipelineStage(status),
    disablePolling: true,
  });

  const sourceTrades = engine.portfolioTrades;

  const allOpenTrades = useMemo(
    () =>
      [...sourceTrades]
        .filter((t) => t.status === "OPEN")
        .sort((a, b) => b.openedAt - a.openedAt),
    [sourceTrades],
  );

  const allClosedTrades = useMemo(
    () =>
      [...sourceTrades]
        .filter((t) => t.status === "CLOSED")
        .sort((a, b) => (b.closedAt ?? 0) - (a.closedAt ?? 0)),
    [sourceTrades],
  );

  useEffect(() => {
    const timer = window.setInterval(() => setNowMs(Date.now()), 60_000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    let cancelled = false;
    let inFlight = false;

    const fetchScalersStats = async () => {
      if (inFlight) return;
      inFlight = true;
      try {
        const response = await fetch("/api/scalers/stats", { cache: "no-store" });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const data = await response.json();
        if (!cancelled) setScalersStats(parseScalersStats(data));
      } catch {
        if (!cancelled) {
          setScalersStats((prev) => ({ ...prev, regime: "UNKNOWN" }));
        }
      } finally {
        inFlight = false;
      }
    };

    void fetchScalersStats();
    const regimeTimer = window.setInterval(() => void fetchScalersStats(), SCALERS_REGIME_POLL_MS);
    const signalTimer = window.setInterval(() => void fetchScalersStats(), SCALERS_SIGNAL_POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(regimeTimer);
      window.clearInterval(signalTimer);
    };
  }, []);

  useEffect(() => {
    setClosedVisibleLimit(CLOSED_TRADE_INITIAL_LIMIT);
  }, [status]);

  const leaderboard = useMemo(
    () => rankStrategies({ trades: sourceTrades }).rows.slice(0, LEADERBOARD_LIMIT),
    [sourceTrades],
  );

  const analytics = useMemo(() => computeAnalytics(sourceTrades), [sourceTrades]);
  const equitySparklineValues = useMemo(
    () => buildEquitySparklineValues(sourceTrades, engine.account.equity, analytics.realizedPnl, analytics.unrealizedPnl),
    [analytics.realizedPnl, analytics.unrealizedPnl, engine.account.equity, sourceTrades],
  );

  const priceReady = Number.isFinite(live.price) && live.price > 0;
  const isInitialLoading = engine.loading || engine.persistence.loading || engine.history.loading;
  const maxOpen = engine.config.maxOpenMockTrades;
  const winRatePct = analytics.winRate * 100;
  const winRateAccent = analytics.closedTrades > 0 && winRatePct > 50 ? "var(--green)" : analytics.closedTrades > 0 && winRatePct < 50 ? "var(--amber)" : "var(--border)";
  const engineStatus = priceReady && !engine.error ? "Live" : engine.error ? "Attention" : "Updating";

  const summaryStrip = (
    <section
      aria-label={`${TRADE_ENGINE_TITLE} summary`}
      style={{
        overflowX: "auto",
        border: "1px solid var(--card-border, var(--border))",
        borderRadius: "var(--radius-card)",
        background: "var(--card-bg, var(--surface))",
        boxShadow: "var(--shadow-card)",
        padding: "18px",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
          marginBottom: 14,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", flexWrap: "wrap", gap: 10, minWidth: 0 }}>
          <h1
            style={{
              margin: 0,
              color: "var(--text-primary)",
              fontSize: 20,
              fontWeight: 800,
              letterSpacing: "-0.03em",
              whiteSpace: "nowrap",
            }}
          >
            {TRADE_ENGINE_TITLE}
          </h1>
          <RegimeBadge regime={scalersStats.regime} />
          <RegimeBadge regime={engineStatus.toUpperCase()} />
        </div>
      </div>
      <div
        className="m3-kpi-strip"
        style={{
          gridTemplateColumns: "repeat(8, minmax(150px, 1fr))",
          minWidth: 1100,
        }}
      >
        {isInitialLoading ? (
          <KpiStripSkeleton />
        ) : (
          <>
            <StageStat
              label="BTC Mark"
              value={priceReady ? fmtPrice(live.price) : "$0.00"}
              accent={priceReady ? "var(--green)" : "var(--amber)"}
              tone={priceReady ? "positive" : "warning"}
              size="lg"
            />
            <StageStat label="Equity" value={fmtUsd(engine.account.equity)} accent="var(--gold)" size="lg">
              <Sparkline
                data={equitySparklineValues}
                width={88}
                height={28}
                color={
                  equitySparklineValues.length < 2 || equitySparklineValues[equitySparklineValues.length - 1] >= equitySparklineValues[0]
                    ? "var(--green)"
                    : "var(--red)"
                }
              />
            </StageStat>
            <StageStat
              label="Open Positions"
              value={String(engine.account.openCount)}
              accent={engine.account.openCount > 0 ? "var(--amber)" : "var(--border)"}
              tone={engine.account.openCount > 0 ? "warning" : "neutral"}
              size="md"
            />
            <StageStat label="Closed Trades" value={String(analytics.closedTrades)} accent="var(--border)" size="sm" />
            <StageStat
              label="Win Rate"
              value={fmtPct(winRatePct)}
              accent={winRateAccent}
              tone={analytics.closedTrades > 0 && winRatePct > 50 ? "positive" : analytics.closedTrades > 0 && winRatePct < 50 ? "warning" : "neutral"}
              size="md"
            />
            <StageStat
              label="Realized PnL"
              value={fmtUsd(analytics.realizedPnl)}
              accent={analytics.realizedPnl >= 0 ? "var(--green)" : "var(--red)"}
              tone={analytics.realizedPnl >= 0 ? "positive" : "negative"}
              size="md"
            />
            <StageStat
              label="Unrealized PnL"
              value={fmtUsd(analytics.unrealizedPnl)}
              accent={analytics.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)"}
              tone={analytics.unrealizedPnl >= 0 ? "positive" : "negative"}
              size="md"
            />
            <StageStat
              label="Engine Status"
              value={engineStatus}
              accent={engineStatus === "Live" ? "var(--green)" : "var(--amber)"}
              tone={engineStatus === "Live" ? "positive" : "warning"}
              size="sm"
            />
          </>
        )}
      </div>

      {!priceReady || engine.error || engine.diagnostics.recentRejections[0] ? (
        <div style={{ marginTop: 8, display: "grid", gap: 3, fontSize: 10, color: "var(--text-muted)" }}>
          {!priceReady ? <p style={{ color: "var(--amber)" }}>Waiting for live BTC price before trade creation can start.</p> : null}
          {engine.error ? <p style={{ color: "var(--red)" }}>Signal tick: {engine.error}</p> : null}
          {engine.diagnostics.recentRejections[0] ? (
            <p>Latest rejection: {engine.diagnostics.recentRejections[0].message}</p>
          ) : null}
        </div>
      ) : null}
    </section>
  );

  const leftPanel = (
    <>
      <TerminalCard
        title="Open Positions"
        subtitle={`${allOpenTrades.length.toLocaleString()} open · cap ${maxOpen} · TP/SL targets live`}
      >
        <OpenPositionsTable trades={allOpenTrades} nowMs={nowMs} loading={isInitialLoading} />
      </TerminalCard>

      <TerminalCard
        title="Closed Trades"
        subtitle={`${allClosedTrades.length.toLocaleString()} closed · newest first · table starts with last ${CLOSED_TRADE_INITIAL_LIMIT}`}
      >
        <ClosedTradesTable
          trades={allClosedTrades}
          analytics={analytics}
          visibleLimit={closedVisibleLimit}
          nowMs={nowMs}
          onShowMore={() =>
            setClosedVisibleLimit((current) => Math.min(current + CLOSED_TRADE_LIMIT_STEP, allClosedTrades.length))
          }
        />
      </TerminalCard>
    </>
  );

  const rightPanel = (
    <>
      <TerminalCard
        title="Live Signals"
        subtitle="Scalers feed · last 10 signals · 15s refresh"
      >
        <LiveSignalsPanel stats={scalersStats} />
      </TerminalCard>

      <TerminalCard
        title="Strategy Leadership Board"
        subtitle="Composite score · win rate · profit factor · net PnL"
      >
        <LeaderboardTable rows={leaderboard} />
      </TerminalCard>

      {false && showPipeline ? null : null}
    </>
  );

  if (renderShell) {
    return <>{renderShell({ summaryStrip, leftPanel, rightPanel })}</>;
  }

  return (
    <div className="grid gap-3">
      {summaryStrip}
      <div
        className="icc-trade-engine-grid"
        style={{
          display: "grid",
          gridTemplateColumns: "3fr 2fr",
          gap: 12,
          minHeight: 0,
        }}
      >
        <div style={{ display: "grid", alignContent: "start", gap: 12, minHeight: 0 }}>
          {leftPanel}
        </div>
        <div style={{ display: "grid", alignContent: "start", gap: 12, minHeight: 0 }}>
          {rightPanel}
        </div>
      </div>
    </div>
  );
}
