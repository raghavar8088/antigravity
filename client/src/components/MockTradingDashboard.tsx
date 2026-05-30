"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import { useMockCandleBuilder } from "@/hooks/useMockCandleBuilder";
import { useMockResearchRunner } from "@/hooks/useMockResearchRunner";
import { useMockTradingEngine } from "@/hooks/useMockTradingEngine";
import { useMarketRegime } from "@/hooks/useMarketRegime";
import { useStrategyScoring } from "@/hooks/useStrategyScoring";
import { MockResearchChartsPanel } from "@/components/MockResearchChartsPanel";
import { WorkspaceNavPanel } from "@/components/desk/WorkspaceNavPanel";
import {
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskEmptyState,
  DeskMetricTile,
  DeskSectionHeader,
  type DeskColumn,
} from "@/components/desk/ui";
import {
  computeAnalytics,
  filterMockTrades,
  mockTradeAgeMinutes,
  MOCK_TRADE_SORT_OPTIONS,
  sortMockTrades,
  type MockAccountState,
  type MockTradeAgeFilterMode,
  type MockSide,
  type MockSizingMode,
  type MockTrade,
  type MockTradeFilter,
  type MockTradeSortKey,
  type MockTradeStatus,
  type MockTradingConfig,
} from "@/lib/mockTradingEngine";
import { ALL_RESEARCH_FAMILIES, type ResearchFamily } from "@/lib/mockResearchStrategies";
import {
  computeAdvancedResearchAnalytics,
  computeDailyPnlPoints,
  createEquitySnapshot,
  type AdvancedResearchAnalytics,
} from "@/lib/mockResearchAnalytics";
import { computeMockWalkForwardRows, type MockWalkForwardRow } from "@/lib/mockResearchWalkForward";
import { computePortfolioAllocation, type PortfolioAllocationResult } from "@/lib/mockResearchPortfolioAllocation";
import { computeStrategyHealth, type StrategyHealthRow } from "@/lib/strategyHealthEngine";
import type { StrategyScore } from "@/lib/strategyScoringEngine";
import { workspaceModuleDescription } from "@/lib/workspaceModuleDescription";

function fmtUsd(value: number, digits = 2): string {
  if (!Number.isFinite(value)) return "—";
  const sign = value < 0 ? "-" : "";
  return `${sign}$${Math.abs(value).toLocaleString("en-US", {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })}`;
}

function fmtUsdK(value: number): string {
  if (!Number.isFinite(value)) return "—";
  const abs = Math.abs(value);
  if (abs >= 1_000_000) return `${value < 0 ? "-" : ""}$${(abs / 1_000_000).toFixed(2)}M`;
  if (abs >= 10_000) return `${value < 0 ? "-" : ""}$${(abs / 1_000).toFixed(1)}K`;
  return fmtUsd(value, 0);
}

function fmtPrice(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "—";
  return value.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function pnlClass(value: number): string {
  if (value > 0) return "desk-pnl-positive";
  if (value < 0) return "desk-pnl-negative";
  return "";
}

function fmtPct(value: number, digits = 2): string {
  if (!Number.isFinite(value)) return "—";
  return `${(value * 100).toFixed(digits)}%`;
}

function fmtAge(openedAt: number, ref: number = Date.now()): string {
  const secs = Math.max(0, Math.floor((ref - openedAt) / 1000));
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  return `${hrs}h ${mins % 60}m`;
}

function fmtTradeAge(trade: MockTrade, ref: number = Date.now()): string {
  const ageMs = mockTradeAgeMinutes(trade, ref) * 60_000;
  return fmtAge(0, ageMs);
}

function parseMinuteInput(value: string): number | null {
  if (value.trim() === "") return null;
  const next = Number(value);
  return Number.isFinite(next) && next >= 0 ? next : null;
}

function safeRatio(value: number, max: number): number {
  if (!Number.isFinite(value) || !Number.isFinite(max) || max <= 0) return 0;
  return value / max;
}

function logColor(event: string): string {
  if (event === "MOCK_TRADE_CREATED") return "var(--desk-success)";
  if (event === "MOCK_TRADE_TP_HIT") return "var(--desk-success)";
  if (event === "MOCK_TRADE_SL_HIT") return "var(--desk-error)";
  if (event === "MOCK_TRADE_REJECTED") return "var(--desk-warning)";
  if (event === "MOCK_TRADE_LIMIT_REACHED") return "var(--desk-warning)";
  return "var(--desk-primary)";
}

export const MOCK_TRADE_TABLE_REQUIRED_HEADERS = [
  "Current PnL",
  "TP Profit $",
  "SL Loss $",
  "Exit Reason",
] as const;

function exitReasonBadge(reason: MockTrade["exitReason"]) {
  if (!reason) return <span style={{ color: "var(--desk-on-surface-variant)" }}>—</span>;
  const tone =
    reason === "TAKE_PROFIT"
      ? "success"
      : reason === "STOP_LOSS"
        ? "error"
        : reason === "MAX_HOLD"
          ? "warning"
          : "default";
  return <DeskChip tone={tone}>{reason}</DeskChip>;
}

export default function MockTradingDashboard() {
  const router = useRouter();
  const live = useLiveBTCPrice();
  const engine = useMockTradingEngine({ price: live.price });
  const candles = useMockCandleBuilder(live.price);
  const regime = useMarketRegime({ candles: candles.snapshot, newCandleReady: candles.newCandleReady });
  const research = useMockResearchRunner({
    candles: candles.snapshot,
    newCandleReady: candles.newCandleReady,
    ingestResearchSignals: engine.ingestResearchSignals,
    price: live.price,
    currentRegime: regime.regime,
  });
  const scoring = useStrategyScoring({
    trades: engine.trades,
    currentRegime: regime.regime,
    newCandleReady: candles.newCandleReady,
    topNCount: research.config.topN,
  });

  const [filter, setFilter] = useState<MockTradeFilter>({});
  const [showOpenOnly, setShowOpenOnly] = useState(false);
  const [sortKey, setSortKey] = useState<MockTradeSortKey>("most_profitable");
  const [strategySearch, setStrategySearch] = useState("");
  const [displayPage, setDisplayPage] = useState(1);
  const [displayPageSize, setDisplayPageSize] = useState(100);
  const [ageFilterNow, setAgeFilterNow] = useState(() => Date.now());

  const tableSourceTrades =
    engine.persistence.status === "mongo" || engine.historyTrades.length > 0
      ? engine.historyTrades
      : engine.trades;

  const trades = useMemo(() => {
    const combined: MockTradeFilter = { ...filter };
    if (showOpenOnly) combined.status = "OPEN";
    const filtered = filterMockTrades(tableSourceTrades, combined, ageFilterNow);
    const search = strategySearch.trim().toLowerCase();
    const searched = search
      ? filtered.filter((t) =>
          String(t.strategyId).includes(search) ||
          t.strategyName.toLowerCase().includes(search) ||
          (t.strategyFamily ?? "").toLowerCase().includes(search)
        )
      : filtered;
    return sortMockTrades(searched, sortKey);
  }, [tableSourceTrades, filter, showOpenOnly, sortKey, strategySearch, ageFilterNow]);

  const maxOpenMockTrades = Math.max(1, Math.floor(engine.config.maxOpenMockTrades));
  const openUsage = Math.min(1, safeRatio(engine.account.openCount, maxOpenMockTrades));
  const openUsagePct = openUsage * 100;
  const openUsageWarning = engine.account.openCount >= maxOpenMockTrades * 0.8;
  const displayTotalPages = Math.max(1, Math.ceil(trades.length / displayPageSize));
  const displayStart = trades.length === 0 ? 0 : (displayPage - 1) * displayPageSize + 1;
  const displayEnd = Math.min(trades.length, displayPage * displayPageSize);
  const displayedTrades = useMemo(
    () => trades.slice((displayPage - 1) * displayPageSize, displayPage * displayPageSize),
    [displayPage, displayPageSize, trades],
  );

  useEffect(() => {
    setDisplayPage(1);
  }, [filter, showOpenOnly, sortKey, strategySearch, displayPageSize]);

  useEffect(() => {
    const nextApproved =
      research.config.selectionMode === "REGIME_MODE"
        ? scoring.approvedRegimeIds
        : scoring.approvedProfitIds;
    if (research.config.approvedStrategyIds === nextApproved) return;
    research.setConfig({ ...research.config, approvedStrategyIds: nextApproved });
  }, [research.config, research.setConfig, scoring.approvedProfitIds, scoring.approvedRegimeIds]);

  useEffect(() => {
    if (!regime.snapshot || typeof fetch !== "function") return;
    void fetch("/api/mock-trading/regime", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ snapshot: regime.snapshot }),
    }).catch(() => {
      // Regime persistence is best-effort; dashboard state remains local.
    });
  }, [regime.snapshot]);

  useEffect(() => {
    if (!candles.newCandleReady || typeof fetch !== "function") return;
    const snapshot = createEquitySnapshot({ account: engine.account, trades: engine.trades, regime: regime.regime });
    void fetch("/api/mock-trading/equity", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        point: {
          timestamp: snapshot.timestamp,
          equity: snapshot.equity,
          realizedPnl: snapshot.realizedPnl,
          unrealizedPnl: snapshot.unrealizedPnl,
          drawdownPct: snapshot.drawdownPct,
          dailyPnl: snapshot.dailyPnl,
          regime: snapshot.regime ?? undefined,
        },
      }),
    }).catch(() => {
      // Equity persistence is best-effort; local chart state continues updating.
    });

    const latestDaily = computeDailyPnlPoints(engine.trades).at(-1);
    if (latestDaily) {
      const dayStart = Math.floor(latestDaily.timestamp / 86_400_000) * 86_400_000;
      const tradeCount = engine.trades.filter(
        (trade) => trade.status === "CLOSED" && trade.closedAt != null && trade.closedAt >= dayStart,
      ).length;
      void fetch("/api/mock-trading/daily-pnl", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          point: {
            day: dayStart,
            pnl: latestDaily.value,
            tradeCount,
            regime: regime.regime ?? undefined,
          },
        }),
      }).catch(() => {
        // Daily PnL history is best-effort.
      });
    }
  }, [candles.newCandleReady, engine.account, engine.trades, regime.regime]);

  useEffect(() => {
    if (scoring.scores.length === 0 || typeof fetch !== "function") return;
    void fetch("/api/mock-trading/scores", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ scores: scoring.scores }),
    }).catch(() => {
      // Latest ranking and ranking-history persistence is best-effort.
    });
  }, [scoring.lastScoredAt, scoring.scores]);

  useEffect(() => {
    setDisplayPage((page) => Math.min(page, displayTotalPages));
  }, [displayTotalPages]);

  useEffect(() => {
    const id = setInterval(() => setAgeFilterNow(Date.now()), 30_000);
    return () => clearInterval(id);
  }, []);

  const strategyOptions = useMemo(() => {
    const seen = new Map<number, string>();
    for (const t of engine.trades) {
      if (!seen.has(t.strategyId)) seen.set(t.strategyId, t.strategyName);
    }
    return [...seen.entries()].sort(([a], [b]) => a - b);
  }, [engine.trades]);

  const blockerOptions = useMemo(() => {
    const set = new Set<string>();
    for (const t of engine.trades) for (const b of t.blockers) set.add(b.gate);
    return [...set].sort();
  }, [engine.trades]);

  const familyOptions = useMemo(() => {
    const set = new Set<string>(ALL_RESEARCH_FAMILIES);
    for (const t of engine.trades) if (t.strategyFamily) set.add(t.strategyFamily);
    return [...set].sort();
  }, [engine.trades]);

  const columns = useMemo<DeskColumn<MockTrade>[]>(
    () => [
      { id: "ts", header: "Opened", cell: (t) => new Date(t.openedAt).toLocaleTimeString() },
      { id: "strategy", header: "Strategy", cell: (t) => `#${t.strategyId} ${t.strategyName}` },
      {
        id: "side",
        header: "Side",
        cell: (t) => (
          <span style={{ color: t.side === "BUY" ? "var(--desk-success)" : "var(--desk-error)" }}>
            {t.side}
          </span>
        ),
      },
      { id: "entry", header: "Entry", align: "right", cell: (t) => fmtPrice(t.entryPrice) },
      {
        id: "tpUsd",
        header: "TP Profit $",
        align: "right",
        cell: (t) => (
          <span style={{ color: "var(--desk-success)" }}>{fmtUsd(t.takeProfitUsd)}</span>
        ),
      },
      {
        id: "slUsd",
        header: "SL Loss $",
        align: "right",
        cell: (t) => (
          <span style={{ color: "var(--desk-error)" }}>{fmtUsd(-t.stopLossUsd)}</span>
        ),
      },
      {
        id: "pnl",
        header: "Current PnL",
        align: "right",
        cell: (t) => {
          const pnl = t.status === "OPEN" ? t.unrealizedPnl : t.realizedPnl;
          return <span className={pnlClass(pnl)}>{fmtUsd(pnl)}</span>;
        },
      },
      {
        id: "status",
        header: "Status",
        cell: (t) => (
          <DeskChip tone={t.status === "OPEN" ? "primary" : "default"}>{t.status}</DeskChip>
        ),
      },
      {
        id: "exitReason",
        header: "Exit Reason",
        cell: (t) => exitReasonBadge(t.exitReason),
      },
      {
        id: "blockers",
        header: "Blockers",
        cell: (t) =>
          t.blockers.length === 0 ? (
            <span style={{ color: "var(--desk-on-surface-variant)" }}>—</span>
          ) : (
            <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
              {t.blockers.map((b, i) => (
                <DeskChip key={`${t.id}-b-${i}`} tone="warning">
                  {b.gate}
                </DeskChip>
              ))}
            </div>
          ),
      },
      { id: "age", header: "Age", align: "right", cell: (t) => fmtTradeAge(t, ageFilterNow) },
      {
        id: "actions",
        header: "",
        cell: (t) =>
          t.status === "OPEN" ? (
            <DeskButton
              variant="outlined"
              onClick={() => engine.closeTrade(t.id)}
              style={{ minHeight: 28, fontSize: "0.75rem", padding: "0 10px" }}
            >
              Close
            </DeskButton>
          ) : (
            <span style={{ color: "var(--desk-on-surface-variant)", fontSize: 11 }}>—</span>
          ),
      },
    ],
    [engine, ageFilterNow],
  );

  const acct = engine.account;
  const totalPnlClass = pnlClass(engine.analytics.totalPnl);
  const realizedClass = pnlClass(engine.analytics.realizedPnl);
  const unrealizedClass = pnlClass(engine.analytics.unrealizedPnl);
  const advancedResearch = useMemo(
    () => computeAdvancedResearchAnalytics({ trades: engine.trades, scores: scoring.scores, account: acct }),
    [acct, engine.trades, scoring.scores],
  );
  const strategyHealth = useMemo(
    () => computeStrategyHealth(scoring.scores, engine.trades),
    [engine.trades, scoring.scores],
  );
  const walkForwardRows = useMemo(
    () => computeMockWalkForwardRows(engine.trades),
    [engine.trades],
  );
  const strategyFamilyById = useMemo(() => {
    const map = new Map<number, string>();
    for (const trade of engine.trades) {
      if (trade.strategyFamily) map.set(trade.strategyId, trade.strategyFamily);
    }
    return map;
  }, [engine.trades]);
  const portfolioAllocation = useMemo(
    () =>
      computePortfolioAllocation({
        scores: scoring.scores,
        healthRows: strategyHealth,
        equity: acct.equity,
        strategyFamilyById,
      }),
    [acct.equity, scoring.scores, strategyFamilyById, strategyHealth],
  );

  return (
    <main className="trading-shell">
      <div className="trading-shell__inner">
        <header className="trading-landing-header">
          <div>
            <h1 className="trading-landing-header__title">Mock Trading</h1>
            <p className="trading-landing-header__desc">
              Simulated ${acct.startingBalance.toLocaleString("en-US")} paper account — strategy
              signals are ranked, passed through blocker/risk/exposure controls, then executed with
              futures-style sizing, fees, slippage, funding, TP/SL, and max-hold lifecycle rules.
            </p>
          </div>
          <div className="trading-landing-header__chips">
            <DeskChip tone="warning">Mock</DeskChip>
            <DeskChip tone="default">${(acct.startingBalance / 1_000_000).toFixed(1)}M paper</DeskChip>
            <DeskChip tone="default">{engine.config.leverage}× leverage</DeskChip>
            <DeskChip tone={live.connected ? "success" : "error"}>
              {live.connected ? "Live feed" : "Disconnected"}
            </DeskChip>
            <DeskChip
              tone={
                engine.persistence.status === "mongo"
                  ? "success"
                  : engine.persistence.status === "hydrating"
                    ? "primary"
                    : "warning"
              }
            >
              {engine.persistence.status === "mongo"
                ? "Mongo persisted"
                : engine.persistence.status === "hydrating"
                  ? "Hydrating Mongo"
                  : "local cache fallback"}
            </DeskChip>
            <DeskChip tone={openUsageWarning ? "warning" : "default"}>
              {acct.openCount.toLocaleString("en-US")} / {maxOpenMockTrades.toLocaleString("en-US")} open
            </DeskChip>
          </div>
        </header>

        <div className="workspace-nav--slim">
          <WorkspaceNavPanel
            activeModule="mockTrading"
            onModuleChange={(next) => {
              if (next === "btcFutureTrading") router.push("/btc-future-trading");
            }}
            actionsEnabled={false}
            onActionsEnabledChange={() => {}}
            actionToggleTitle="Controls are not applicable in Mock Trading."
            moduleDescription={workspaceModuleDescription("mockTrading")}
          />
        </div>

        <DeskBanner
          variant="warning"
          title={`Mock Trading uses simulated $${acct.startingBalance.toLocaleString("en-US")} paper balance. No real orders are placed.`}
        >
          The production safety pipeline is untouched. This module subscribes to the same BTC price
          feed and strategy signal trace, then rejects any signal with blockers or failed risk
          checks. Sizing, leverage, fees, slippage, funding, cooldown, and exposure limits are
          simulated before a mock position is created.
        </DeskBanner>

        {engine.persistence.error && (
          <DeskBanner variant="warning" title="Mock Trading persistence is degraded">
            {engine.persistence.error}. The dashboard will keep using localStorage as a cache until
            MongoDB writes recover.
          </DeskBanner>
        )}

        {openUsageWarning && (
          <DeskBanner
            variant="warning"
            title={`Open mock trades are at ${openUsagePct.toFixed(1)}% of the configured limit`}
          >
            Mock Trading remains simulation-only and will reject new mock entries after{" "}
            {maxOpenMockTrades.toLocaleString("en-US")} OPEN mock trades.
          </DeskBanner>
        )}

        <AccountSummaryCard
          account={acct}
          maxOpenMockTrades={maxOpenMockTrades}
          openUsagePct={openUsagePct}
        />

        <DeskCard padding="md">
          <DeskSectionHeader title="Live ticker" subtitle={`BTCUSD · ${live.connected ? "Binance stream" : "reconnecting"}`} />
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))",
              gap: 12,
            }}
          >
            <DeskMetricTile label="BTC Price" value={fmtPrice(live.price)} compact />
            <DeskMetricTile label="24h Change" value={`${live.change24h.toFixed(2)}%`} valueClassName={pnlClass(live.change24h)} compact />
            <DeskMetricTile label="24h High" value={fmtPrice(live.high24h)} compact />
            <DeskMetricTile label="24h Low" value={fmtPrice(live.low24h)} compact />
            <DeskMetricTile label="Ticks / s" value={live.ticksPerSecond} compact />
            <DeskMetricTile
              label="Trace age"
              value={engine.traceAgeSeconds == null ? "—" : `${engine.traceAgeSeconds}s`}
              compact
            />
            <DeskMetricTile label="Closed candles" value={candles.closedCount} compact />
            <DeskMetricTile label="Research eval" value={research.lastEvalCount} compact />
            <DeskMetricTile label="Research signals" value={research.lastSignalCount} compact />
          </div>
        </DeskCard>

        <ResearchControlsCard research={research} closedCandles={candles.closedCount} />

        <ResearchLabOverview
          snapshot={regime.snapshot}
          scoring={scoring}
          research={research}
          healthRows={strategyHealth}
          walkForwardRows={walkForwardRows}
          allocation={portfolioAllocation}
        />

        <MockResearchChartsPanel trades={engine.trades} account={acct} regime={regime.regime} />

        <AdvancedResearchAnalyticsPanel analytics={advancedResearch} scores={scoring.scores} />

        <DeskCard padding="md">
          <DeskSectionHeader title="Trade analytics" subtitle="Realized PnL is net of round-trip fees and slippage; unrealized PnL marks every OPEN trade to the live BTC mark and subtracts the round-trip fee debt that would crystallize on close." />
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))",
              gap: 12,
            }}
          >
            <DeskMetricTile label="Total trades" value={engine.analytics.totalTrades} compact />
            <DeskMetricTile
              label="Open / Max"
              value={`${engine.analytics.openTrades.toLocaleString("en-US")} / ${maxOpenMockTrades.toLocaleString("en-US")}`}
              compact
            />
            <DeskMetricTile label="Closed" value={engine.analytics.closedTrades} compact />
            <DeskMetricTile label="Risk rejects" value={engine.mockLimitRejectedSignals.toLocaleString("en-US")} compact />
            <DeskMetricTile label="Win rate" value={fmtPct(engine.analytics.winRate, 1)} compact />
            <DeskMetricTile label="Total PnL" value={fmtUsd(engine.analytics.totalPnl)} valueClassName={totalPnlClass} compact />
            <DeskMetricTile label="Realized" value={fmtUsd(engine.analytics.realizedPnl)} valueClassName={realizedClass} compact />
            <DeskMetricTile label="Unrealized" value={fmtUsd(engine.analytics.unrealizedPnl)} valueClassName={unrealizedClass} compact />
            <DeskMetricTile label="Avg realized PnL" value={fmtUsd(engine.analytics.averageRealizedPnl)} compact />
            <DeskMetricTile label="Average trade" value={fmtUsd(engine.analytics.averageTrade)} compact />
            <DeskMetricTile label="Avg win" value={fmtUsd(engine.analytics.averageWin)} valueClassName={pnlClass(engine.analytics.averageWin)} compact />
            <DeskMetricTile label="Avg loss" value={fmtUsd(engine.analytics.averageLoss)} valueClassName={pnlClass(engine.analytics.averageLoss)} compact />
            <DeskMetricTile label="TP wins" value={engine.analytics.takeProfitWins} compact />
            <DeskMetricTile label="SL losses" value={engine.analytics.stopLossLosses} compact />
            <DeskMetricTile label="TP hit rate" value={fmtPct(engine.analytics.takeProfitHitRate, 1)} compact />
            <DeskMetricTile label="SL hit rate" value={fmtPct(engine.analytics.stopLossHitRate, 1)} compact />
            <DeskMetricTile
              label="Profit factor"
              value={engine.analytics.profitFactor == null ? "—" : engine.analytics.profitFactor.toFixed(2)}
              compact
            />
            <DeskMetricTile
              label="Sharpe ratio"
              value={engine.analytics.sharpeRatio == null ? "—" : engine.analytics.sharpeRatio.toFixed(2)}
              compact
            />
          </div>
        </DeskCard>

        <MockConfigCard config={engine.config} onChange={engine.setConfig} />

        <DeskCard padding="md">
          <DeskSectionHeader
            title="Open Positions & Trade History"
            subtitle={`${displayStart}-${displayEnd} of ${trades.length} matching displayed · ${engine.history.total || engine.trades.length} persisted total`}
          />
          <div
            style={{
              display: "flex",
              flexWrap: "wrap",
              gap: 8,
              alignItems: "center",
              marginBottom: 12,
            }}
          >
            <select
              value={filter.strategyId ?? ""}
              onChange={(e) =>
                setFilter((f) => ({
                  ...f,
                  strategyId: e.target.value ? Number(e.target.value) : null,
                }))
              }
              style={selectStyle}
            >
              <option value="">All strategies</option>
              {strategyOptions.map(([id, name]) => (
                <option key={id} value={id}>
                  #{id} {name}
                </option>
              ))}
            </select>

            <select
              value={filter.side ?? ""}
              onChange={(e) => setFilter((f) => ({ ...f, side: (e.target.value || null) as MockSide | null }))}
              style={selectStyle}
            >
              <option value="">All sides</option>
              <option value="BUY">BUY</option>
              <option value="SELL">SELL</option>
            </select>

            <select
              value={filter.status ?? ""}
              onChange={(e) =>
                setFilter((f) => ({ ...f, status: (e.target.value || null) as MockTradeStatus | null }))
              }
              style={selectStyle}
            >
              <option value="">All statuses</option>
              <option value="OPEN">OPEN</option>
              <option value="CLOSED">CLOSED</option>
            </select>

            <select
              value={filter.blockerGate ?? ""}
              onChange={(e) => setFilter((f) => ({ ...f, blockerGate: e.target.value || null }))}
              style={selectStyle}
            >
              <option value="">All blockers</option>
              {blockerOptions.map((g) => (
                <option key={g} value={g}>
                  {g}
                </option>
              ))}
            </select>

            <select
              value={filter.strategyFamily ?? ""}
              onChange={(e) => setFilter((f) => ({ ...f, strategyFamily: e.target.value || null }))}
              style={selectStyle}
            >
              <option value="">All families</option>
              {familyOptions.map((family) => (
                <option key={family} value={family}>
                  {research.familyLabels[family as ResearchFamily] ?? family}
                </option>
              ))}
            </select>

            <select
              value={filter.profitability ?? ""}
              onChange={(e) =>
                setFilter((f) => ({
                  ...f,
                  profitability: (e.target.value || null) as "profit" | "loss" | null,
                }))
              }
              style={selectStyle}
            >
              <option value="">Profit & loss</option>
              <option value="profit">Profitable</option>
              <option value="loss">Unprofitable</option>
            </select>

            <select
              value={filter.ageMode ?? ""}
              onChange={(e) => {
                const ageMode = (e.target.value || null) as MockTradeAgeFilterMode | null;
                setFilter((f) => ({
                  ...f,
                  ageMode,
                  ageMinMinutes: ageMode === "less" ? null : f.ageMinMinutes ?? null,
                  ageMaxMinutes: ageMode === "more" ? null : f.ageMaxMinutes ?? null,
                }));
              }}
              style={selectStyle}
              aria-label="Age filter"
              title="Age filter"
            >
              <option value="">All ages</option>
              <option value="less">Less than</option>
              <option value="more">More than</option>
              <option value="between">Between</option>
            </select>

            {filter.ageMode === "less" && (
              <input
                type="number"
                min={0}
                step={1}
                value={filter.ageMaxMinutes ?? ""}
                onChange={(e) =>
                  setFilter((f) => ({ ...f, ageMaxMinutes: parseMinuteInput(e.target.value) }))
                }
                placeholder="Minutes"
                aria-label="Age less than minutes"
                style={minuteInputStyle}
              />
            )}

            {filter.ageMode === "more" && (
              <input
                type="number"
                min={0}
                step={1}
                value={filter.ageMinMinutes ?? ""}
                onChange={(e) =>
                  setFilter((f) => ({ ...f, ageMinMinutes: parseMinuteInput(e.target.value) }))
                }
                placeholder="Minutes"
                aria-label="Age more than minutes"
                style={minuteInputStyle}
              />
            )}

            {filter.ageMode === "between" && (
              <>
                <input
                  type="number"
                  min={0}
                  step={1}
                  value={filter.ageMinMinutes ?? ""}
                  onChange={(e) =>
                    setFilter((f) => ({ ...f, ageMinMinutes: parseMinuteInput(e.target.value) }))
                  }
                  placeholder="Min"
                  aria-label="Minimum age minutes"
                  style={minuteInputStyle}
                />
                <input
                  type="number"
                  min={0}
                  step={1}
                  value={filter.ageMaxMinutes ?? ""}
                  onChange={(e) =>
                    setFilter((f) => ({ ...f, ageMaxMinutes: parseMinuteInput(e.target.value) }))
                  }
                  placeholder="Max"
                  aria-label="Maximum age minutes"
                  style={minuteInputStyle}
                />
              </>
            )}

            <select
              value={sortKey}
              onChange={(e) => setSortKey(e.target.value as MockTradeSortKey)}
              style={selectStyle}
              aria-label="Sort trades"
              title="Sort order"
            >
              {MOCK_TRADE_SORT_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>

            <select
              value={displayPageSize}
              onChange={(e) => setDisplayPageSize(Number(e.target.value))}
              style={selectStyle}
              aria-label="Rows per page"
              title="Rows per page"
            >
              {[50, 100, 250, 500, 1_000].map((size) => (
                <option key={size} value={size}>
                  {size.toLocaleString("en-US")} rows
                </option>
              ))}
            </select>

            <input
              type="search"
              value={strategySearch}
              onChange={(e) => setStrategySearch(e.target.value)}
              placeholder="Search strategy/family"
              style={{ ...selectStyle, minWidth: 190 }}
            />

            <label style={{ display: "flex", gap: 4, alignItems: "center", fontSize: 12 }}>
              <input
                type="checkbox"
                checked={showOpenOnly}
                onChange={(e) => setShowOpenOnly(e.target.checked)}
              />
              Open only
            </label>

            <label style={{ display: "flex", gap: 4, alignItems: "center", fontSize: 12 }}>
              <input
                type="checkbox"
                checked={Boolean(filter.researchOnly)}
                onChange={(e) => setFilter((f) => ({ ...f, researchOnly: e.target.checked ? true : null }))}
              />
              Research pack only
            </label>

            <div style={{ marginLeft: "auto", display: "flex", gap: 8 }}>
              <DeskButton
                variant="outlined"
                onClick={() => {
                  setFilter({});
                  setStrategySearch("");
                }}
              >
                Clear filters
              </DeskButton>
              <DeskButton variant="danger-tonal" onClick={engine.reset}>
                Reset persisted state
              </DeskButton>
            </div>
          </div>

          <div
            style={{
              display: "flex",
              flexWrap: "wrap",
              gap: 8,
              alignItems: "center",
              marginBottom: 12,
              color: "var(--desk-on-surface-variant)",
              fontSize: 12,
            }}
          >
            <span>
              Mongo history page {engine.history.page} / {engine.history.totalPages}
              {engine.history.loading ? " · loading" : ""}
            </span>
            <span>
              Display page {displayPage} / {displayTotalPages}
            </span>
            <DeskButton
              variant="outlined"
              onClick={() => engine.history.setPage(Math.max(1, engine.history.page - 1))}
              disabled={engine.history.page <= 1 || engine.history.loading}
              style={{ minHeight: 28, fontSize: "0.75rem", padding: "0 10px" }}
            >
              Prev
            </DeskButton>
            <DeskButton
              variant="outlined"
              onClick={() => engine.history.setPage(Math.min(engine.history.totalPages, engine.history.page + 1))}
              disabled={engine.history.page >= engine.history.totalPages || engine.history.loading}
              style={{ minHeight: 28, fontSize: "0.75rem", padding: "0 10px" }}
            >
              Next
            </DeskButton>
            <DeskButton
              variant="outlined"
              onClick={() => setDisplayPage(Math.max(1, displayPage - 1))}
              disabled={displayPage <= 1}
              style={{ minHeight: 28, fontSize: "0.75rem", padding: "0 10px" }}
            >
              Display Prev
            </DeskButton>
            <DeskButton
              variant="outlined"
              onClick={() => setDisplayPage(Math.min(displayTotalPages, displayPage + 1))}
              disabled={displayPage >= displayTotalPages}
              style={{ minHeight: 28, fontSize: "0.75rem", padding: "0 10px" }}
            >
              Display Next
            </DeskButton>
            {engine.history.error && <span style={{ color: "var(--desk-error)" }}>{engine.history.error}</span>}
          </div>

          <DeskDataTable
            columns={columns}
            rows={displayedTrades}
            getRowKey={(t) => t.id}
            empty={
              <DeskEmptyState
                title="No mock trades yet"
                subtitle={
                  engine.error
                    ? `Trace fetch error: ${engine.error}`
                    : engine.persistence.loading
                      ? "Hydrating persisted Mock Trading state from MongoDB."
                    : "Waiting for the next strategy signal trace tick. The live BTC feed is connected; mock trades will appear as soon as any strategy raises a signal."
                }
              />
            }
            minWidth={1540}
          />
        </DeskCard>

        <PerStrategyCard analytics={engine.analytics} startingBalance={acct.startingBalance} />
        <ResearchAnalyticsCards analytics={engine.analytics} startingBalance={acct.startingBalance} />
        <PerBlockerCard analytics={engine.analytics} />

        <DeskCard padding="md">
          <DeskSectionHeader title="Mock trade log" subtitle="Most recent events — newest first" />
          {engine.logs.length === 0 ? (
            <DeskEmptyState
              title="No log events"
              subtitle="MOCK_TRADE_CREATED, MOCK_TRADE_TP_HIT, MOCK_TRADE_SL_HIT, and MOCK_TRADE_CLOSED events will appear here as signals arrive."
            />
          ) : (
            <div
              style={{
                fontFamily: "var(--desk-font-mono, monospace)",
                fontSize: 11,
                lineHeight: 1.5,
                maxHeight: 260,
                overflowY: "auto",
                background: "var(--desk-surface-container)",
                padding: 12,
                borderRadius: 6,
              }}
            >
              {engine.logs.map((log, i) => (
                <div key={i}>
                  <span style={{ color: "var(--desk-on-surface-variant)" }}>
                    {new Date(log.ts).toLocaleTimeString()}
                  </span>{" "}
                  <span style={{ color: logColor(log.event) }}>
                    {log.event}
                  </span>{" "}
                  {log.tradeId ? `id=${log.tradeId} ` : ""}
                  strategy=#{log.strategyId} {log.strategyName} side={log.side} price=
                  {fmtPrice(log.price)}
                  {log.message ? ` ${log.message}` : ""}
                  {log.entryPrice != null ? ` entry=${fmtPrice(log.entryPrice)}` : ""}
                  {log.exitPrice != null ? ` exit=${fmtPrice(log.exitPrice)}` : ""}
                  {log.exitReason ? ` reason=${log.exitReason}` : ""}
                  {log.notional != null ? ` notional=${fmtUsdK(log.notional)}` : ""}
                  {log.pnl != null ? ` pnl=${fmtUsd(log.pnl)}` : ""}
                </div>
              ))}
            </div>
          )}
          <div style={{ marginTop: 8, fontSize: 11, color: "var(--desk-on-surface-variant)" }}>
            Rejected signals are logged for diagnostics; blockers no longer create mock trades.
          </div>
        </DeskCard>
      </div>
    </main>
  );
}

const selectStyle: React.CSSProperties = {
  padding: "6px 10px",
  borderRadius: 6,
  border: "1px solid var(--desk-outline)",
  background: "var(--desk-surface)",
  color: "var(--desk-on-surface)",
  fontSize: "0.8125rem",
  minWidth: 130,
};

const minuteInputStyle: React.CSSProperties = {
  ...selectStyle,
  minWidth: 82,
  width: 92,
};

function AccountSummaryCard({
  account,
  maxOpenMockTrades,
  openUsagePct,
}: {
  account: MockAccountState;
  maxOpenMockTrades: number;
  openUsagePct: number;
}) {
  const equityClass = pnlClass(account.equity - account.startingBalance);
  const realizedClass = pnlClass(account.realizedPnl);
  const unrealClass = pnlClass(account.unrealizedPnl);
  const returnClass = pnlClass(account.returnPct);
  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="Account summary"
        subtitle="Mirrors the main paper desk: starting balance, equity = balance + realized + unrealized; margin = notional / leverage; available = equity − margin."
      />
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))",
          gap: 12,
        }}
      >
        <DeskMetricTile label="Starting Balance" value={fmtUsd(account.startingBalance, 0)} compact />
        <DeskMetricTile label="Cash Balance" value={fmtUsd(account.cashBalance, 0)} compact />
        <DeskMetricTile label="Equity" value={fmtUsd(account.equity, 0)} valueClassName={equityClass} compact />
        <DeskMetricTile label="Realized PnL" value={fmtUsd(account.realizedPnl, 2)} valueClassName={realizedClass} compact />
        <DeskMetricTile label="Unrealized PnL" value={fmtUsd(account.unrealizedPnl, 2)} valueClassName={unrealClass} compact />
        <DeskMetricTile label="Exposure" value={fmtUsd(account.exposure, 0)} compact />
        <DeskMetricTile label="Margin Used" value={fmtUsd(account.marginUsed, 0)} compact />
        <DeskMetricTile label="Available" value={fmtUsd(account.availableBalance, 0)} compact />
        <DeskMetricTile
          label="Open Mock Trades / Max"
          value={`${account.openCount.toLocaleString("en-US")} / ${maxOpenMockTrades.toLocaleString("en-US")}`}
          compact
        />
        <DeskMetricTile label="Limit Used" value={`${openUsagePct.toFixed(1)}%`} compact />
        <DeskMetricTile label="Return %" value={fmtPct(account.returnPct, 2)} valueClassName={returnClass} compact />
        <DeskMetricTile label="Max Drawdown" value={fmtPct(account.maxDrawdownPct, 2)} compact />
        <DeskMetricTile label="Peak Equity" value={fmtUsd(account.peakEquity, 0)} compact />
      </div>
      <div style={{ marginTop: 14 }}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            fontSize: 12,
            color: "var(--desk-on-surface-variant)",
            marginBottom: 6,
          }}
        >
          <span>Open mock trade usage</span>
          <span>{openUsagePct.toFixed(1)}%</span>
        </div>
        <div
          style={{
            height: 8,
            borderRadius: 999,
            overflow: "hidden",
            background: "var(--desk-surface-container-highest)",
          }}
        >
          <div
            style={{
              width: `${Math.min(100, openUsagePct)}%`,
              height: "100%",
              background: openUsagePct >= 80 ? "var(--desk-warning)" : "var(--desk-primary)",
            }}
          />
        </div>
      </div>
    </DeskCard>
  );
}

function MockConfigCard({
  config,
  onChange,
}: {
  config: MockTradingConfig;
  onChange: (next: MockTradingConfig) => void;
}) {
  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="Mock account & sizing"
        subtitle="Signals are ranked, risk-checked, and sized from account equity before mock futures execution costs are applied."
      />
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(170px, 1fr))", gap: 12 }}>
        <ConfigNumber
          label="Starting balance ($)"
          value={config.startingBalanceUsd}
          step={10_000}
          onChange={(v) => onChange({ ...config, startingBalanceUsd: v })}
        />
        <ConfigNumber
          label="Max open mock trades"
          value={config.maxOpenMockTrades}
          step={1}
          onChange={(v) => onChange({ ...config, maxOpenMockTrades: Math.max(1, Math.floor(v)) })}
        />
        <ConfigNumber
          label="Max long positions"
          value={config.maxOpenLongTrades}
          step={1}
          onChange={(v) => onChange({ ...config, maxOpenLongTrades: Math.max(1, Math.floor(v)) })}
        />
        <ConfigNumber
          label="Max short positions"
          value={config.maxOpenShortTrades}
          step={1}
          onChange={(v) => onChange({ ...config, maxOpenShortTrades: Math.max(1, Math.floor(v)) })}
        />
        <ConfigNumber
          label="Cooldown (min)"
          value={config.tradeCooldownMinutes}
          step={1}
          onChange={(v) => onChange({ ...config, tradeCooldownMinutes: Math.max(15, v) })}
        />
        <ConfigNumber
          label="Min signal score"
          value={config.minSignalScore}
          step={1}
          onChange={(v) => onChange({ ...config, minSignalScore: Math.max(0, Math.min(100, v)) })}
        />
        <ConfigNumber
          label="Max signals / batch"
          value={config.maxSignalsPerBatch}
          step={1}
          onChange={(v) => onChange({ ...config, maxSignalsPerBatch: Math.max(1, Math.floor(v)) })}
        />
        <ConfigSelect
          label="Sizing mode"
          value={config.sizingMode}
          options={[
            { value: "fixed_pct_equity", label: "Fixed % of equity" },
            { value: "fixed_notional", label: "Fixed notional ($)" },
            { value: "risk_pct_equity", label: "% risk per trade" },
          ]}
          onChange={(v) => onChange({ ...config, sizingMode: v as MockSizingMode })}
        />
        {config.sizingMode === "fixed_pct_equity" && (
          <ConfigNumber
            label="Fixed % of equity"
            value={config.fixedPctOfEquity}
            step={0.1}
            onChange={(v) => onChange({ ...config, fixedPctOfEquity: v })}
          />
        )}
        {config.sizingMode === "fixed_notional" && (
          <ConfigNumber
            label="Fixed notional ($)"
            value={config.fixedNotionalUsd}
            step={100}
            onChange={(v) => onChange({ ...config, fixedNotionalUsd: v })}
          />
        )}
        {config.sizingMode === "risk_pct_equity" && (
          <ConfigNumber
            label="Risk % per trade"
            value={config.riskPctOfEquity}
            step={0.1}
            onChange={(v) => onChange({ ...config, riskPctOfEquity: v })}
          />
        )}
        <ConfigNumber
          label="Leverage (×)"
          value={config.leverage}
          step={1}
          onChange={(v) => onChange({ ...config, leverage: Math.max(1, Math.min(125, Math.floor(v))) })}
        />
        <ConfigNumber
          label="Take profit %"
          value={config.takeProfitPct}
          step={0.1}
          onChange={(v) => onChange({ ...config, takeProfitPct: Math.max(0.01, v) })}
        />
        <ConfigNumber
          label="Stop loss %"
          value={config.stopLossPct}
          step={0.1}
          onChange={(v) => onChange({ ...config, stopLossPct: Math.max(0.01, v) })}
        />
        <ConfigNumber
          label="Max hold (min)"
          value={config.maxHoldMinutes}
          step={1}
          onChange={(v) => onChange({ ...config, maxHoldMinutes: v })}
        />
        <ConfigNumber
          label="Taker fee per side (%)"
          value={config.takerFeePct * 100}
          step={0.01}
          onChange={(v) => onChange({ ...config, takerFeePct: Math.max(0, v / 100) })}
        />
        <ConfigNumber
          label="Slippage per side (bps)"
          value={config.slippageBpsPerSide}
          step={1}
          onChange={(v) => onChange({ ...config, slippageBpsPerSide: Math.max(0, v) })}
        />
        <ConfigNumber
          label="Min risk/reward"
          value={config.minRiskRewardRatio}
          step={0.1}
          onChange={(v) => onChange({ ...config, minRiskRewardRatio: Math.max(1.5, v) })}
        />
        <ConfigNumber
          label="Daily loss limit (%)"
          value={config.dailyLossLimitPct}
          step={0.5}
          onChange={(v) => onChange({ ...config, dailyLossLimitPct: Math.max(0, v) })}
        />
        <ConfigNumber
          label="Weekly loss limit (%)"
          value={config.weeklyLossLimitPct}
          step={0.5}
          onChange={(v) => onChange({ ...config, weeklyLossLimitPct: Math.max(0, v) })}
        />
        <ConfigNumber
          label="Max drawdown stop (%)"
          value={config.maxDrawdownPct}
          step={0.5}
          onChange={(v) => onChange({ ...config, maxDrawdownPct: Math.max(0, v) })}
        />
        <ConfigNumber
          label="Funding rate / 8h (%)"
          value={config.fundingRatePctPer8h}
          step={0.001}
          onChange={(v) => onChange({ ...config, fundingRatePctPer8h: v })}
        />
        <ConfigNumber
          label="Funding interval (h)"
          value={config.fundingIntervalHours}
          step={1}
          onChange={(v) => onChange({ ...config, fundingIntervalHours: Math.max(0.01, v) })}
        />
      </div>
    </DeskCard>
  );
}

function ResearchControlsCard({
  research,
  closedCandles,
}: {
  research: ReturnType<typeof useMockResearchRunner>;
  closedCandles: number;
}) {
  const updateFamily = (family: ResearchFamily, enabled: boolean) => {
    const next = new Set(research.config.enabledFamilies);
    if (enabled) next.add(family);
    else next.delete(family);
    research.setConfig({ ...research.config, enabledFamilies: next });
  };

  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="500-strategy research runner"
        subtitle="Mock-only: evaluates BTC strategy variants on each closed 1-minute candle and sends capped BUY/SELL signals only into the Mock Trading engine."
      />
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(170px, 1fr))", gap: 12 }}>
        <label style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 13 }}>
          <input
            type="checkbox"
            checked={research.config.enabled}
            onChange={(e) => research.setConfig({ ...research.config, enabled: e.target.checked })}
          />
          Enable research strategies
        </label>
        <ConfigNumber
          label="Max signals / minute"
          value={research.config.maxSignalsPerMinute}
          step={1}
          onChange={(v) =>
            research.setConfig({
              ...research.config,
              maxSignalsPerMinute: Math.max(1, Math.min(500, Math.floor(v))),
            })
          }
        />
        <ConfigNumber
          label="Minimum confidence"
          value={research.config.minConfidence}
          step={1}
          onChange={(v) =>
            research.setConfig({
              ...research.config,
              minConfidence: Math.max(0, Math.min(100, v)),
            })
          }
        />
        <DeskMetricTile label="Registry" value={`${research.strategies.length} strategies`} compact />
        <DeskMetricTile label="Closed candles" value={closedCandles} compact />
        <DeskMetricTile label="Last signals" value={research.lastSignalCount} compact />
      </div>

      <div style={{ marginTop: 14, display: "flex", gap: 8, flexWrap: "wrap" }}>
        <DeskButton
          variant="outlined"
          onClick={() => research.setConfig({ ...research.config, enabledFamilies: new Set(ALL_RESEARCH_FAMILIES) })}
        >
          Enable all families
        </DeskButton>
        <DeskButton
          variant="outlined"
          onClick={() => research.setConfig({ ...research.config, enabledFamilies: new Set<ResearchFamily>() })}
        >
          Disable all families
        </DeskButton>
      </div>

      <div
        style={{
          marginTop: 12,
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))",
          gap: 8,
          maxHeight: 170,
          overflowY: "auto",
        }}
      >
        {ALL_RESEARCH_FAMILIES.map((family) => (
          <label key={family} style={{ display: "flex", gap: 6, alignItems: "center", fontSize: 12 }}>
            <input
              type="checkbox"
              checked={research.config.enabledFamilies.has(family)}
              onChange={(e) => updateFamily(family, e.target.checked)}
            />
            {research.familyLabels[family]}
          </label>
        ))}
      </div>

      <div style={{ marginTop: 10, fontSize: 12, color: "var(--desk-on-surface-variant)" }}>
        Last evaluation:{" "}
        {research.lastEvalAt == null ? "waiting for a closed candle" : new Date(research.lastEvalAt).toLocaleTimeString()}
        {research.hasRun ? ` · evaluated ${research.lastEvalCount} strategies` : ""}
      </div>
    </DeskCard>
  );
}

function ConfigNumber({
  label,
  value,
  step,
  onChange,
}: {
  label: string;
  value: number;
  step: number;
  onChange: (v: number) => void;
}) {
  return (
    <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <span className="desk-label-md">{label}</span>
      <input
        type="number"
        value={value}
        step={step}
        min={0}
        onChange={(e) => {
          const v = Number(e.target.value);
          if (Number.isFinite(v) && v >= 0) onChange(v);
        }}
        style={{
          padding: "6px 10px",
          borderRadius: 6,
          border: "1px solid var(--desk-outline)",
          background: "var(--desk-surface)",
          color: "var(--desk-on-surface)",
          fontSize: "0.875rem",
          fontFamily: "var(--desk-font-mono, monospace)",
        }}
      />
    </label>
  );
}

function ConfigSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
}) {
  return (
    <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <span className="desk-label-md">{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        style={{
          padding: "6px 10px",
          borderRadius: 6,
          border: "1px solid var(--desk-outline)",
          background: "var(--desk-surface)",
          color: "var(--desk-on-surface)",
          fontSize: "0.875rem",
        }}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}

function regimeTone(regime: string | null | undefined): "success" | "warning" | "error" | "primary" | "default" {
  if (regime === "TRENDING") return "success";
  if (regime === "HIGH_VOLATILITY_BREAKOUT") return "warning";
  if (regime === "LOW_VOLATILITY_CHOP") return "default";
  if (regime === "RANGING") return "primary";
  return "default";
}

function ResearchLabOverview({
  snapshot,
  scoring,
  research,
  healthRows,
  walkForwardRows,
  allocation,
}: {
  snapshot: ReturnType<typeof useMarketRegime>["snapshot"];
  scoring: ReturnType<typeof useStrategyScoring>;
  research: ReturnType<typeof useMockResearchRunner>;
  healthRows: StrategyHealthRow[];
  walkForwardRows: MockWalkForwardRow[];
  allocation: PortfolioAllocationResult;
}) {
  return (
    <>
      <DeskCard padding="md">
        <DeskSectionHeader
          title="BTC Research Lab"
          subtitle="Research/Profit/Regime modes remain fully mock-only. Profit and Regime modes only allow ranked research-backed strategy IDs."
        />
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 12 }}>
          <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <span className="desk-label-md">Selection mode</span>
            <select
              value={research.config.selectionMode}
              onChange={(event) =>
                research.setConfig({
                  ...research.config,
                  selectionMode: event.target.value as typeof research.config.selectionMode,
                })
              }
              style={{
                padding: "6px 10px",
                borderRadius: 6,
                border: "1px solid var(--desk-outline)",
                background: "var(--desk-surface)",
                color: "var(--desk-on-surface)",
                fontSize: "0.875rem",
              }}
            >
              <option value="RESEARCH_MODE">Research Mode — collect all valid signals</option>
              <option value="PROFIT_MODE">Profit Mode — ranked BTC research strategies only</option>
              <option value="REGIME_MODE">Regime Mode — ranked BTC research strategies only</option>
            </select>
          </label>
          <ConfigNumber
            label="Top ranked strategies"
            value={research.config.topN}
            step={1}
            onChange={(value) =>
              research.setConfig({
                ...research.config,
                topN: Math.max(1, Math.min(50, Math.floor(value))),
              })
            }
          />
          <DeskMetricTile label="Ranked strategies" value={scoring.scores.length} compact />
          <DeskMetricTile
            label="Approved IDs"
            value={
              research.config.selectionMode === "REGIME_MODE"
                ? scoring.approvedRegimeIds.size
                : scoring.approvedProfitIds.size
            }
            compact
          />
          <DeskMetricTile
            label="Last scored"
            value={scoring.lastScoredAt == null ? "waiting" : new Date(scoring.lastScoredAt).toLocaleTimeString()}
            compact
          />
        </div>
      </DeskCard>

      <DeskCard padding="md">
        <DeskSectionHeader title="Current market regime" subtitle="Classified from closed BTC OHLCV candles using ADX, ATR%, Bollinger width, realized volatility, and EMA slope." />
        {snapshot == null ? (
          <DeskEmptyState title="Waiting for candle history" subtitle="Regime classification starts after enough closed 1-minute candles are available." />
        ) : (
          <>
            <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 12 }}>
              <DeskChip tone={regimeTone(snapshot.regime)}>{snapshot.regime}</DeskChip>
              <div style={{ flex: 1, height: 8, borderRadius: 999, background: "var(--desk-surface-variant)", overflow: "hidden" }}>
                <div
                  style={{
                    width: `${Math.max(0, Math.min(100, snapshot.confidence))}%`,
                    height: "100%",
                    background: "var(--desk-primary)",
                  }}
                />
              </div>
              <span style={{ fontSize: 12, color: "var(--desk-on-surface-variant)" }}>
                {snapshot.confidence.toFixed(0)}% confidence
              </span>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))", gap: 12 }}>
              <DeskMetricTile label="ADX" value={snapshot.adx.toFixed(1)} compact />
              <DeskMetricTile label="ATR %" value={`${snapshot.atrPct.toFixed(3)}%`} compact />
              <DeskMetricTile label="BB width %" value={`${snapshot.bbWidthPct.toFixed(3)}%`} compact />
              <DeskMetricTile label="BB percentile" value={`${(snapshot.bbWidthPercentile * 100).toFixed(0)}%`} compact />
              <DeskMetricTile label="EMA slope" value={`${(snapshot.emaSlope * 100).toFixed(3)}%`} compact />
              <DeskMetricTile label="Realized vol" value={`${snapshot.realizedVol.toFixed(1)}%`} compact />
            </div>
          </>
        )}
      </DeskCard>

      <BestWorstResearchPanel scores={scoring.scores} />
      <StrategyScoringCard scores={scoring.scores} healthRows={healthRows} />
      <StrategyHealthCard rows={healthRows} />
      <WalkForwardValidationCard rows={walkForwardRows} />
      <PortfolioAllocationCard allocation={allocation} />
    </>
  );
}

function BestWorstResearchPanel({ scores }: { scores: StrategyScore[] }) {
  const best = scores.slice(0, 5);
  const worst = [...scores]
    .filter((score) => score.metrics.sampleSizeConfidence >= 40)
    .sort((a, b) => a.overallScore - b.overallScore)
    .slice(0, 5);

  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(320px, 1fr))", gap: 12 }}>
      <DeskCard padding="md">
        <DeskSectionHeader title="Best ranked strategies" subtitle="Weighted score: PnL, profit factor, win rate, drawdown quality, Sharpe, recency, and sample size." />
        {best.length === 0 ? (
          <DeskEmptyState title="No ranked strategies yet" subtitle="Scores populate after mock research trades are created and evaluated." />
        ) : (
          <DeskDataTable
            columns={[
              { id: "rank", header: "Rank", align: "right", cell: (score) => score.rank },
              { id: "name", header: "Strategy", cell: (score) => `#${score.strategyId} ${score.strategyName}` },
              { id: "score", header: "Score", align: "right", cell: (score) => score.overallScore.toFixed(1) },
              { id: "pnl", header: "PnL", align: "right", cell: (score) => <span className={pnlClass(score.metrics.netPnl)}>{fmtUsd(score.metrics.netPnl)}</span> },
            ]}
            rows={best}
            getRowKey={(score) => String(score.strategyId)}
            minWidth={520}
          />
        )}
      </DeskCard>

      <DeskCard padding="md">
        <DeskSectionHeader title="Worst ranked strategies" subtitle="Requires a minimum sample size before a strategy is shown as underperforming." />
        {worst.length === 0 ? (
          <DeskEmptyState title="No underperformers yet" subtitle="Worst-strategy ranking appears after enough closed trades exist." />
        ) : (
          <DeskDataTable
            columns={[
              { id: "rank", header: "Rank", align: "right", cell: (score) => score.rank },
              { id: "name", header: "Strategy", cell: (score) => `#${score.strategyId} ${score.strategyName}` },
              { id: "score", header: "Score", align: "right", cell: (score) => score.overallScore.toFixed(1) },
              { id: "pnl", header: "PnL", align: "right", cell: (score) => <span className={pnlClass(score.metrics.netPnl)}>{fmtUsd(score.metrics.netPnl)}</span> },
            ]}
            rows={worst}
            getRowKey={(score) => String(score.strategyId)}
            minWidth={520}
          />
        )}
      </DeskCard>
    </div>
  );
}

function StrategyScoringCard({
  scores,
  healthRows,
}: {
  scores: StrategyScore[];
  healthRows: StrategyHealthRow[];
}) {
  const healthById = new Map(healthRows.map((row) => [row.strategyId, row]));
  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="Strategy scoring table"
        subtitle="Objective ranking of strategies after realistic mock costs. Default sorting is most profitable/highest scoring first."
      />
      {scores.length === 0 ? (
        <DeskEmptyState title="No strategy scores yet" subtitle="The scoring engine updates from closed and open mock trade history." />
      ) : (
        <DeskDataTable
          columns={[
            { id: "rank", header: "Rank", align: "right", cell: (score) => score.rank },
            { id: "strategy", header: "Strategy", cell: (score) => `#${score.strategyId} ${score.strategyName}` },
            {
              id: "health",
              header: "Health",
              cell: (score) => {
                const health = healthById.get(score.strategyId);
                const tone =
                  health?.state === "ACTIVE"
                    ? "success"
                    : health?.state === "DISABLED"
                      ? "error"
                      : "warning";
                return <DeskChip tone={tone}>{health?.state ?? "WATCHLIST"}</DeskChip>;
              },
            },
            { id: "overall", header: "Overall", align: "right", cell: (score) => score.overallScore.toFixed(1) },
            { id: "regime", header: "Regime", align: "right", cell: (score) => score.currentRegimeScore.toFixed(1) },
            { id: "pnl", header: "Net PnL", align: "right", cell: (score) => <span className={pnlClass(score.metrics.netPnl)}>{fmtUsd(score.metrics.netPnl)}</span> },
            { id: "win", header: "Win %", align: "right", cell: (score) => fmtPct(score.metrics.winRate, 1) },
            { id: "pf", header: "PF", align: "right", cell: (score) => score.metrics.profitFactor.toFixed(2) },
            { id: "sharpe", header: "Sharpe", align: "right", cell: (score) => score.metrics.sharpeRatio.toFixed(2) },
            { id: "dd", header: "Max DD", align: "right", cell: (score) => fmtUsd(score.metrics.maxDrawdown) },
            { id: "conf", header: "Confidence", cell: (score) => <DeskChip tone={score.confidenceRating === "HIGH" ? "success" : score.confidenceRating === "MEDIUM" ? "primary" : "warning"}>{score.confidenceRating}</DeskChip> },
            { id: "trades", header: "Trades", align: "right", cell: (score) => score.metrics.closedTrades },
          ]}
          rows={scores.slice(0, 100)}
          getRowKey={(score) => String(score.strategyId)}
          minWidth={1120}
        />
      )}
    </DeskCard>
  );
}

function StrategyHealthCard({ rows }: { rows: StrategyHealthRow[] }) {
  const active = rows.filter((row) => row.state === "ACTIVE").length;
  const watchlist = rows.filter((row) => row.state === "WATCHLIST").length;
  const disabled = rows.filter((row) => row.state === "DISABLED").length;

  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="Strategy health engine"
        subtitle="Auto-classifies strategies from sample size, expectancy, profit factor, drawdown, and persistent losing streaks."
      />
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))", gap: 12, marginBottom: 12 }}>
        <DeskMetricTile label="Active" value={active} compact />
        <DeskMetricTile label="Watchlist" value={watchlist} compact />
        <DeskMetricTile label="Disabled" value={disabled} compact />
      </div>
      {rows.length === 0 ? (
        <DeskEmptyState title="No health data yet" subtitle="Health states populate after strategy scores are available." />
      ) : (
        <DeskDataTable
          columns={[
            { id: "strategy", header: "Strategy", cell: (row) => `#${row.strategyId} ${row.strategyName}` },
            {
              id: "state",
              header: "State",
              cell: (row) => (
                <DeskChip tone={row.state === "ACTIVE" ? "success" : row.state === "DISABLED" ? "error" : "warning"}>
                  {row.state}
                </DeskChip>
              ),
            },
            { id: "trust", header: "Trust", align: "right", cell: (row) => `${row.trustScore.toFixed(0)}%` },
            { id: "trades", header: "Closed", align: "right", cell: (row) => row.closedTrades },
            { id: "expectancy", header: "Expectancy", align: "right", cell: (row) => <span className={pnlClass(row.expectancy)}>{fmtUsd(row.expectancy)}</span> },
            { id: "pf", header: "PF", align: "right", cell: (row) => row.profitFactor.toFixed(2) },
            { id: "losses", header: "Loss streak", align: "right", cell: (row) => row.consecutiveLosses },
            { id: "reason", header: "Reason", cell: (row) => row.reasons.join("; ") },
          ]}
          rows={rows.slice(0, 60)}
          getRowKey={(row) => String(row.strategyId)}
          minWidth={980}
        />
      )}
    </DeskCard>
  );
}

function WalkForwardValidationCard({ rows }: { rows: MockWalkForwardRow[] }) {
  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="Walk-forward validation"
        subtitle="Rolling in-sample training and out-of-sample validation windows. Strategies need OOS replication, not just in-sample profit."
      />
      {rows.length === 0 ? (
        <DeskEmptyState title="Collecting walk-forward data" subtitle="Requires enough closed trade history across training and validation windows." />
      ) : (
        <DeskDataTable
          columns={[
            { id: "strategy", header: "Strategy", cell: (row) => `#${row.strategyId} ${row.strategyName}` },
            {
              id: "status",
              header: "Status",
              cell: (row) => (
                <DeskChip tone={row.status === "PASS" ? "success" : row.status === "FAIL" ? "error" : "warning"}>
                  {row.status}
                </DeskChip>
              ),
            },
            { id: "windows", header: "Windows", align: "right", cell: (row) => row.windows },
            { id: "train", header: "Train exp", align: "right", cell: (row) => <span className={pnlClass(row.inSampleExpectancy)}>{fmtUsd(row.inSampleExpectancy)}</span> },
            { id: "oos", header: "OOS exp", align: "right", cell: (row) => <span className={pnlClass(row.outOfSampleExpectancy)}>{fmtUsd(row.outOfSampleExpectancy)}</span> },
            { id: "oosPnl", header: "OOS PnL", align: "right", cell: (row) => <span className={pnlClass(row.outOfSampleNetPnl)}>{fmtUsd(row.outOfSampleNetPnl)}</span> },
            { id: "score", header: "WF score", align: "right", cell: (row) => `${row.walkForwardScore.toFixed(0)}%` },
          ]}
          rows={rows.slice(0, 40)}
          getRowKey={(row) => String(row.strategyId)}
          minWidth={920}
        />
      )}
    </DeskCard>
  );
}

function PortfolioAllocationCard({ allocation }: { allocation: PortfolioAllocationResult }) {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(360px, 1fr))", gap: 12 }}>
      <DeskCard padding="md">
        <DeskSectionHeader
          title="Strategy allocation weights"
          subtitle="Score-weighted capital allocation for ACTIVE strategies only. Disabled/watchlist strategies receive no allocation."
        />
        {allocation.strategyRows.length === 0 ? (
          <DeskEmptyState title="No active allocation yet" subtitle="Allocations appear after strategies have enough healthy scored history." />
        ) : (
          <DeskDataTable
            columns={[
              { id: "strategy", header: "Strategy", cell: (row) => `#${row.strategyId} ${row.strategyName}` },
              { id: "weight", header: "Weight", align: "right", cell: (row) => fmtPct(row.allocationPct, 1) },
              { id: "capital", header: "Capital", align: "right", cell: (row) => fmtUsd(row.capitalUsd) },
              { id: "trust", header: "Trust", align: "right", cell: (row) => `${row.trustScore.toFixed(0)}%` },
            ]}
            rows={allocation.strategyRows.slice(0, 30)}
            getRowKey={(row) => String(row.strategyId)}
            minWidth={760}
          />
        )}
      </DeskCard>
      <DeskCard padding="md">
        <DeskSectionHeader
          title="Family allocation weights"
          subtitle={`Unallocated capital: ${fmtPct(allocation.unallocatedPct, 1)}. Family weights roll up active strategy allocations.`}
        />
        {allocation.familyRows.length === 0 ? (
          <DeskEmptyState title="No family allocation yet" subtitle="Family allocation appears when active strategy weights exist." />
        ) : (
          <DeskDataTable
            columns={[
              { id: "family", header: "Family", cell: (row) => row.family },
              { id: "strategies", header: "Strategies", align: "right", cell: (row) => row.strategyCount },
              { id: "weight", header: "Weight", align: "right", cell: (row) => fmtPct(row.allocationPct, 1) },
              { id: "capital", header: "Capital", align: "right", cell: (row) => fmtUsd(row.capitalUsd) },
            ]}
            rows={allocation.familyRows}
            getRowKey={(row) => row.family}
            minWidth={620}
          />
        )}
      </DeskCard>
    </div>
  );
}

function AdvancedResearchAnalyticsPanel({
  analytics,
  scores,
}: {
  analytics: AdvancedResearchAnalytics;
  scores: StrategyScore[];
}) {
  const confidenceRows = scores.slice(0, 20);

  return (
    <>
      {analytics.warnings.length > 0 && (
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Risk and quality warnings"
            subtitle="Warnings highlight sample-size, drawdown, exposure, and correlation risks in the mock research lab."
          />
          <div style={{ display: "grid", gap: 8 }}>
            {analytics.warnings.map((warning) => (
              <DeskBanner
                key={warning.code}
                variant={warning.severity === "CRITICAL" ? "error" : warning.severity === "WARNING" ? "warning" : "info"}
                title={warning.code}
              >
                {warning.message}
              </DeskBanner>
            ))}
          </div>
        </DeskCard>
      )}

      <DeskCard padding="md">
        <DeskSectionHeader
          title="Exposure and side bias"
          subtitle="Open exposure, family concentration, and long/short realized bias for mock-only positions."
        />
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))", gap: 12 }}>
          <DeskMetricTile label="Open exposure" value={fmtUsdK(analytics.exposure.totalExposure)} compact />
          <DeskMetricTile label="Long exposure" value={fmtUsdK(analytics.exposure.longExposure)} compact />
          <DeskMetricTile label="Short exposure" value={fmtUsdK(analytics.exposure.shortExposure)} compact />
          <DeskMetricTile label="Long concentration" value={fmtPct(analytics.exposure.longConcentrationPct, 1)} compact />
          <DeskMetricTile label="Short concentration" value={fmtPct(analytics.exposure.shortConcentrationPct, 1)} compact />
          <DeskMetricTile label="Open trades" value={analytics.exposure.openTrades} compact />
          <DeskMetricTile label="Long win rate" value={fmtPct(analytics.bias.longWinRate, 1)} compact />
          <DeskMetricTile label="Short win rate" value={fmtPct(analytics.bias.shortWinRate, 1)} compact />
          <DeskMetricTile label="Long PnL" value={fmtUsd(analytics.bias.longPnl)} valueClassName={pnlClass(analytics.bias.longPnl)} compact />
          <DeskMetricTile label="Short PnL" value={fmtUsd(analytics.bias.shortPnl)} valueClassName={pnlClass(analytics.bias.shortPnl)} compact />
        </div>
      </DeskCard>

      <DeskCard padding="md">
        <DeskSectionHeader
          title="PnL by strategy family"
          subtitle="Family profitability, regime fit, profit factor, exposure, and long/short mix."
        />
        {analytics.familyRows.length === 0 ? (
          <DeskEmptyState title="No family analytics yet" subtitle="Family analytics populate as mock strategy trades are generated." />
        ) : (
          <DeskDataTable
            columns={[
              { id: "family", header: "Family", cell: (row) => row.family },
              { id: "trades", header: "Trades", align: "right", cell: (row) => row.trades },
              { id: "win", header: "Win %", align: "right", cell: (row) => fmtPct(row.winRate, 1) },
              { id: "pf", header: "PF", align: "right", cell: (row) => row.profitFactor.toFixed(2) },
              { id: "pnl", header: "Net PnL", align: "right", cell: (row) => <span className={pnlClass(row.netPnl)}>{fmtUsd(row.netPnl)}</span> },
              { id: "exp", header: "Exposure", align: "right", cell: (row) => fmtUsdK(row.exposure) },
              { id: "bias", header: "L/S", align: "right", cell: (row) => `${row.longTrades}/${row.shortTrades}` },
              { id: "best", header: "Best regime", cell: (row) => row.bestRegime },
              { id: "worst", header: "Worst regime", cell: (row) => row.worstRegime },
            ]}
            rows={analytics.familyRows.slice(0, 40)}
            getRowKey={(row) => row.family}
            minWidth={980}
          />
        )}
      </DeskCard>

      <DeskCard padding="md">
        <DeskSectionHeader
          title="Strategy correlation matrix"
          subtitle="Pearson correlation of top strategy cumulative PnL curves. High positive values can indicate stacking risk."
        />
        {analytics.correlationRows.length < 2 ? (
          <DeskEmptyState title="No correlation matrix yet" subtitle="At least two strategies with closed trades are required." />
        ) : (
          <DeskDataTable
            columns={[
              { id: "strategy", header: "Strategy", cell: (row) => `#${row.strategyId}` },
              ...analytics.correlationRows.map((col) => ({
                id: String(col.strategyId),
                header: `#${col.strategyId}`,
                align: "right" as const,
                cell: (row: (typeof analytics.correlationRows)[number]) => {
                  const value = row.correlations[col.strategyId] ?? 0;
                  const color =
                    value > 0.8
                      ? "var(--desk-warning)"
                      : value < -0.5
                        ? "var(--desk-error)"
                        : "var(--desk-on-surface)";
                  return <span style={{ color }}>{value.toFixed(2)}</span>;
                },
              })),
            ]}
            rows={analytics.correlationRows}
            getRowKey={(row) => String(row.strategyId)}
            minWidth={760}
          />
        )}
      </DeskCard>

      <DeskCard padding="md">
        <DeskSectionHeader
          title="Strategy confidence dashboard"
          subtitle="Trust score comes from sample-size confidence and ranking confidence, not from any claim of future profits."
        />
        {confidenceRows.length === 0 ? (
          <DeskEmptyState title="No confidence data yet" subtitle="Strategy confidence appears once scores are computed." />
        ) : (
          <DeskDataTable
            columns={[
              { id: "strategy", header: "Strategy", cell: (score) => `#${score.strategyId} ${score.strategyName}` },
              { id: "rating", header: "Rating", cell: (score) => <DeskChip tone={score.confidenceRating === "HIGH" ? "success" : score.confidenceRating === "MEDIUM" ? "primary" : "warning"}>{score.confidenceRating}</DeskChip> },
              { id: "sample", header: "Sample trust", align: "right", cell: (score) => `${score.metrics.sampleSizeConfidence.toFixed(0)}%` },
              { id: "trades", header: "Closed trades", align: "right", cell: (score) => score.metrics.closedTrades },
              { id: "overall", header: "Overall score", align: "right", cell: (score) => score.overallScore.toFixed(1) },
              { id: "regime", header: "Regime score", align: "right", cell: (score) => score.currentRegimeScore.toFixed(1) },
            ]}
            rows={confidenceRows}
            getRowKey={(score) => String(score.strategyId)}
            minWidth={860}
          />
        )}
      </DeskCard>
    </>
  );
}

function PerStrategyCard({
  analytics,
  startingBalance,
}: {
  analytics: ReturnType<typeof computeAnalytics>;
  startingBalance: number;
}) {
  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="Per-strategy roll-up"
        subtitle="Sorted by total PnL. Return % is PnL / starting balance, so strategies are comparable in account-level terms. Exposure is summed across this strategy's currently OPEN positions."
      />
      {analytics.perStrategy.length === 0 ? (
        <DeskEmptyState title="No data yet" subtitle="Per-strategy roll-up populates as mock trades are created." />
      ) : (
        <DeskDataTable
          columns={[
            { id: "id", header: "Strategy", cell: (r) => `#${r.strategyId} ${r.strategyName}` },
            { id: "total", header: "Total", align: "right", cell: (r) => r.total },
            { id: "open", header: "Open", align: "right", cell: (r) => r.open },
            { id: "closed", header: "Closed", align: "right", cell: (r) => r.closed },
            { id: "wins", header: "W/L", align: "right", cell: (r) => `${r.wins}/${r.losses}` },
            { id: "win", header: "Win %", align: "right", cell: (r) => fmtPct(r.winRate, 1) },
            { id: "exp", header: "Exposure", align: "right", cell: (r) => fmtUsdK(r.exposure) },
            {
              id: "pnl",
              header: "Total PnL",
              align: "right",
              cell: (r) => <span className={pnlClass(r.totalPnl)}>{fmtUsd(r.totalPnl)}</span>,
            },
            {
              id: "ret",
              header: "Return %",
              align: "right",
              cell: (r) => (
                <span className={pnlClass(r.totalPnl)}>
                  {fmtPct(startingBalance > 0 ? r.totalPnl / startingBalance : 0, 2)}
                </span>
              ),
            },
            {
              id: "realized",
              header: "Realized",
              align: "right",
              cell: (r) => <span className={pnlClass(r.realizedPnl)}>{fmtUsd(r.realizedPnl)}</span>,
            },
          ]}
          rows={analytics.perStrategy}
          getRowKey={(r) => String(r.strategyId)}
          minWidth={760}
        />
      )}
    </DeskCard>
  );
}

function ResearchAnalyticsCards({
  analytics,
  startingBalance,
}: {
  analytics: ReturnType<typeof computeAnalytics>;
  startingBalance: number;
}) {
  const top = analytics.perStrategy.filter((r) => r.total > 0).slice(0, 20);
  const worst = analytics.perStrategy
    .filter((r) => r.total > 0)
    .slice()
    .sort((a, b) => a.totalPnl - b.totalPnl)
    .slice(0, 20);
  const families = analytics.perFamily.slice(0, 30);

  return (
    <>
      <DeskCard padding="md">
        <DeskSectionHeader
          title="Top profitable strategies"
          subtitle="Best strategy rows by total net PnL, including trade count and win rate."
        />
        {top.length === 0 ? (
          <DeskEmptyState title="No research results yet" subtitle="Top strategies populate after mock research trades close or mark to profit." />
        ) : (
          <StrategyRankTable rows={top} startingBalance={startingBalance} />
        )}
      </DeskCard>

      <DeskCard padding="md">
        <DeskSectionHeader
          title="Worst strategies"
          subtitle="Lowest strategy rows by total net PnL, capped to keep rendering light."
        />
        {worst.length === 0 ? (
          <DeskEmptyState title="No research results yet" subtitle="Worst-strategy rows populate after mock research trades are created." />
        ) : (
          <StrategyRankTable rows={worst} startingBalance={startingBalance} />
        )}
      </DeskCard>

      <DeskCard padding="md">
        <DeskSectionHeader
          title="PnL by strategy family"
          subtitle="Family-level PnL, trade count, and win rate for the research pack."
        />
        {families.length === 0 ? (
          <DeskEmptyState title="No family data yet" subtitle="Family roll-up appears once research-pack trades are generated." />
        ) : (
          <DeskDataTable
            columns={[
              { id: "family", header: "Family", cell: (r) => r.family },
              { id: "count", header: "Trades", align: "right", cell: (r) => r.tradeCount },
              { id: "open", header: "Open", align: "right", cell: (r) => r.open },
              { id: "closed", header: "Closed", align: "right", cell: (r) => r.closed },
              { id: "win", header: "Win %", align: "right", cell: (r) => fmtPct(r.winRate, 1) },
              {
                id: "pnl",
                header: "Total PnL",
                align: "right",
                cell: (r) => <span className={pnlClass(r.totalPnl)}>{fmtUsd(r.totalPnl)}</span>,
              },
              {
                id: "realized",
                header: "Realized",
                align: "right",
                cell: (r) => <span className={pnlClass(r.realizedPnl)}>{fmtUsd(r.realizedPnl)}</span>,
              },
            ]}
            rows={families}
            getRowKey={(r) => r.family}
            minWidth={700}
          />
        )}
      </DeskCard>
    </>
  );
}

function StrategyRankTable({
  rows,
  startingBalance,
}: {
  rows: ReturnType<typeof computeAnalytics>["perStrategy"];
  startingBalance: number;
}) {
  return (
    <DeskDataTable
      columns={[
        { id: "id", header: "Strategy", cell: (r) => `#${r.strategyId} ${r.strategyName}` },
        { id: "count", header: "Trades", align: "right", cell: (r) => r.total },
        { id: "open", header: "Open", align: "right", cell: (r) => r.open },
        { id: "closed", header: "Closed", align: "right", cell: (r) => r.closed },
        { id: "win", header: "Win %", align: "right", cell: (r) => fmtPct(r.winRate, 1) },
        {
          id: "pnl",
          header: "Total PnL",
          align: "right",
          cell: (r) => <span className={pnlClass(r.totalPnl)}>{fmtUsd(r.totalPnl)}</span>,
        },
        {
          id: "ret",
          header: "Return %",
          align: "right",
          cell: (r) => (
            <span className={pnlClass(r.totalPnl)}>
              {fmtPct(startingBalance > 0 ? r.totalPnl / startingBalance : 0, 2)}
            </span>
          ),
        },
      ]}
      rows={rows}
      getRowKey={(r) => String(r.strategyId)}
      minWidth={720}
    />
  );
}

function PerBlockerCard({ analytics }: { analytics: ReturnType<typeof computeAnalytics> }) {
  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="Blockers by frequency"
        subtitle="How often each production gate would have stopped these mock trades, and whether the trades that hit each gate would have been profitable on this $1M paper account."
      />
      {analytics.perBlocker.length === 0 ? (
        <DeskEmptyState
          title="No blockers recorded yet"
          subtitle="When mock trades are created from signals that the production pipeline would have rejected, the blocker reason appears here."
        />
      ) : (
        <DeskDataTable
          columns={[
            { id: "gate", header: "Blocker gate", cell: (r) => <DeskChip tone="warning">{r.gate}</DeskChip> },
            { id: "total", header: "Mock trades", align: "right", cell: (r) => r.total },
            { id: "wl", header: "W/L", align: "right", cell: (r) => `${r.wins}/${r.losses}` },
            { id: "wr", header: "Win %", align: "right", cell: (r) => fmtPct(r.winRate, 1) },
            {
              id: "pnl",
              header: "Total PnL",
              align: "right",
              cell: (r) => <span className={pnlClass(r.totalPnl)}>{fmtUsd(r.totalPnl)}</span>,
            },
          ]}
          rows={analytics.perBlocker}
          getRowKey={(r) => r.gate}
          minWidth={560}
        />
      )}
    </DeskCard>
  );
}
