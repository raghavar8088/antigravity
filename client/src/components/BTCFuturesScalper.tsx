"use client";

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import type { StrategyLeaderboardRow } from "@/lib/paperTradesAnalytics";
import {
  useBTCFuturesScalperEngine,
  type BTCFuturesPosition,
  type BTCFuturesTrade,
  type BTCFuturesStrategyStatus,
  type BTCFuturesEngineOptions,
} from "@/hooks/useBTCFuturesScalperEngine";
import { FUTURES_WATCHLIST, type FuturesWatchItem } from "@/lib/futuresMarketData";
import { FUTURES_STRATEGY_PROFILES } from "@/lib/futuresSessionMetrics";
import { paperPriceMovePctOnNotional } from "@/lib/futuresPaperMath";
import { resolveCloudPaperTradesAccountKey } from "@/lib/paperTradesAuth";
import { PaperDeskAuthBar } from "@/components/PaperDeskAuthBar";
import { BTCFuturesDeskPanels } from "@/components/btcFutures/BTCFuturesDeskPanels";
import { EntryDebugPanel } from "@/components/btcFutures/EntryDebugPanel";
import { DeskThemeToggle } from "@/components/desk/DeskThemeToggle";
import {
  DeskAppBar,
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  type DeskColumn,
  DeskDataTable,
  DeskEmptyState,
  DeskMetricTile,
  DeskSearchField,
  DeskSectionHeader,
  DeskShell,
  type DeskEngineStatus,
} from "@/components/desk/ui";
import { usePaperDeskAuth } from "@/hooks/usePaperDeskAuth";
import { useDeskMounted } from "@/hooks/useDeskMounted";
import { formatDeskPct, formatDeskUsd, pnlToneClass } from "@/lib/deskFormat";

const deskTestnetOpsEnabled = process.env.NEXT_PUBLIC_DESK_TESTNET_OPS === "1";
const deskShadowIntentsEnabled = process.env.NEXT_PUBLIC_DESK_SHADOW_INTENTS === "1";

// ========== FORMATTERS ==========
function fmtUSD(value: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(value).toLocaleString("en-US", { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
  if (signed) return `${value >= 0 ? "+" : "-"}$${abs}`;
  return `$${abs}`;
}

function fmtPct(value: number, signed = false, decimals = 2) {
  const prefix = signed ? (value >= 0 ? "+" : "-") : "";
  return `${prefix}${Math.abs(value).toFixed(decimals)}%`;
}

function fmtContracts(n: number) {
  return n.toLocaleString("en-US", { maximumFractionDigits: 0 });
}

function tradePriceMovePct(t: BTCFuturesTrade): number {
  return typeof t.priceMovePct === "number" && Number.isFinite(t.priceMovePct)
    ? t.priceMovePct
    : paperPriceMovePctOnNotional(t.entryPrice, t.exitPrice, t.side);
}

function formatShortTime(iso: string) {
  const d = new Date(iso);
  return d.toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", hour12: false });
}

function formatShortDate(iso: string) {
  const d = new Date(iso);
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function formatDate(iso: string) {
  const d = new Date(iso);
  return d.toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" });
}

// ========== DESIGN PRIMITIVES ==========
type BadgeTone = "neutral" | "positive" | "negative" | "info" | "warning";

function BadgePill({ label, tone = "neutral" }: { label: string; tone?: BadgeTone }) {
  const map: Record<BadgeTone, string> = {
    neutral:  "border-zinc-200 bg-white text-zinc-600",
    positive: "border-emerald-200 bg-emerald-50 text-emerald-700",
    negative: "border-rose-200 bg-rose-50 text-rose-700",
    info:     "border-blue-200 bg-blue-50 text-blue-700",
    warning:  "border-amber-200 bg-amber-50 text-amber-700",
  };
  return (
    <span className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-[10px] font-medium uppercase tracking-[0.1em] ${map[tone]}`}>
      {label}
    </span>
  );
}

function SideBadge({ side }: { side: "LONG" | "SHORT" }) {
  return (
    <span className={`rounded px-1.5 py-0.5 text-[10px] font-bold tracking-wider ${
      side === "LONG"
        ? "bg-emerald-100 text-emerald-700"
        : "bg-rose-100 text-rose-700"
    }`}>{side}</span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, string> = {
    READY:       "bg-emerald-100 text-emerald-700",
    IN_POSITION: "bg-blue-100 text-blue-700",
    COOLING:     "bg-amber-100 text-amber-700",
    AVAILABLE:   "bg-zinc-100 text-zinc-600",
  };
  return (
    <span className={`rounded px-1.5 py-0.5 text-[10px] font-bold tracking-wider ${map[status] ?? "bg-zinc-100 text-zinc-500"}`}>
      {status.replace("_", " ")}
    </span>
  );
}

function CompactMetric({ label, value, detail, accent = "" }: {
  label: string; value: string; detail?: string; accent?: string;
}) {
  return (
    <div className="metric-card flex min-h-[104px] flex-col justify-between gap-3">
      <div>
        <div className="metric-label">{label}</div>
        <div className={`metric-value ${accent}`}>{value}</div>
      </div>
      <div className="text-xs" style={{ color: "var(--text-secondary)", minHeight: 18 }}>{detail ?? ""}</div>
    </div>
  );
}

function SummaryCard({ label, value, accent }: { label: string; value: string; accent: string }) {
  return (
    <div className="summary-card flex min-h-[112px] flex-col justify-between gap-3">
      <div className="summary-label">{label}</div>
      <div className={`summary-value ${accent}`}>{value}</div>
    </div>
  );
}

// ========== PREMIUM BAR (Green/Red bars like options selling) ==========
function PremiumBar({ progress, isPositive }: { progress: number; isPositive: boolean }) {
  const width = Math.min(100, Math.max(5, Math.abs(progress) * 2));
  return (
    <div className="h-1.5 w-20 rounded-full bg-zinc-200 overflow-hidden">
      <div
        className={`h-full rounded-full ${isPositive ? "bg-emerald-500" : "bg-rose-500"}`}
        style={{ width: `${width}%` }}
      />
    </div>
  );
}

type BTCFuturesScalperProps = {
  title?: string;
  moduleTagline?: string;
  strategyIds?: BTCFuturesEngineOptions["strategyIds"];
  symbols?: BTCFuturesEngineOptions["symbols"];
  signalThreshold?: BTCFuturesEngineOptions["signalThreshold"];
  strategyProfile?: BTCFuturesEngineOptions["strategyProfile"];
  relaxEntryConfirmation?: BTCFuturesEngineOptions["relaxEntryConfirmation"];
  forceProbeOpen?: BTCFuturesEngineOptions["forceProbeOpen"];
  cooldownMultiplier?: BTCFuturesEngineOptions["cooldownMultiplier"];
  minMoveKMultiplier?: BTCFuturesEngineOptions["minMoveKMultiplier"];
  slippageBpsOverride?: BTCFuturesEngineOptions["slippageBpsOverride"];
  disableAutoKill?: BTCFuturesEngineOptions["disableAutoKill"];
  researchEnsureTrades?: BTCFuturesEngineOptions["researchEnsureTrades"];
  entryUtcSessionOverride?: BTCFuturesEngineOptions["entryUtcSessionOverride"];
  researchMode?: boolean;
  researchPoolIds?: number[];
  researchWinners?: number[];
  researchRetiredIds?: ReadonlySet<number>;
  onPromoteResearchWinner?: (id: number) => void;
  onRetireResearchStrategy?: (id: number) => void;
  onUnretireResearchStrategy?: (id: number) => void;
  onAutoRetireResearchLosers?: (ids: number[]) => void;
  watchlist?: FuturesWatchItem[];
  storageNamespace?: string;
  baseBalance?: number;
};

const LEADERBOARD_WINDOW_DAYS = 30;
const LEADERBOARD_TABLE_LIMIT = 10;
const PAPER_EXPORT_WINDOW_DAYS = 30;

type LeaderboardState = {
  top: StrategyLeaderboardRow[];
  bottom: StrategyLeaderboardRow[];
};

// ========== MAIN COMPONENT ==========
export function BTCFuturesScalper({
  title = "Future Trading",
  moduleTagline = "MULTI-ASSET PERPETUAL FUTURES · 25x · STRATEGIES PER SYMBOL",
  strategyIds,
  symbols,
  signalThreshold = 28,
  strategyProfile,
  relaxEntryConfirmation,
  forceProbeOpen,
  cooldownMultiplier,
  minMoveKMultiplier,
  slippageBpsOverride,
  disableAutoKill,
  researchEnsureTrades,
  entryUtcSessionOverride,
  researchMode = false,
  researchPoolIds,
  researchWinners = [],
  researchRetiredIds = new Set(),
  onPromoteResearchWinner,
  onRetireResearchStrategy,
  onUnretireResearchStrategy,
  onAutoRetireResearchLosers,
  watchlist = FUTURES_WATCHLIST,
  storageNamespace,
  baseBalance = 1000,
}: BTCFuturesScalperProps = {}) {
  const { user: authUser, configured: authConfigured } = usePaperDeskAuth();
  const deskMounted = useDeskMounted();
  const [deskDark, setDeskDark] = useState(false);

  useEffect(() => {
    const root = document.documentElement;
    if (deskDark) {
      document.body.classList.add("combat-mode");
      root.setAttribute("data-theme", "dark");
    } else {
      document.body.classList.remove("combat-mode");
      root.setAttribute("data-theme", "light");
    }
  }, [deskDark]);
  const cloudAccountKey = useMemo(
    () =>
      resolveCloudPaperTradesAccountKey({
        supabaseUserId: authUser?.id,
        storageNamespace: storageNamespace?.trim() || "btc_futures_scalper",
      }),
    [authUser?.id, storageNamespace],
  );

  const {
    positions,
    trades,
    balance,
    equity,
    stats,
    quote,
    isReady,
    pauseEntries,
    disabledStrategies,
    togglePause,
    resetPaperAccount,
    clearTradeHistory,
    setDisabledStrategies,
    addDisabledStrategyIds,
    strategyStatuses,
    dataHealth,
    entryDebug,
  } = useBTCFuturesScalperEngine({
    strategyIds,
    symbols,
    signalThreshold,
    strategyProfile,
    relaxEntryConfirmation,
    forceProbeOpen,
    cooldownMultiplier,
    minMoveKMultiplier,
    slippageBpsOverride,
    disableAutoKill,
    researchEnsureTrades,
    entryUtcSessionOverride,
    storageNamespace,
    supabaseUserId: authUser?.id ?? null,
  });

  const [showAllStrategies, setShowAllStrategies] = useState(false);
  const [showAllTrades, setShowAllTrades] = useState(false);
  const [watchSearch, setWatchSearch] = useState("");
  const [exportingCsv, setExportingCsv] = useState(false);

  const downloadPaperTradesCsv = useCallback(async () => {
    if (!cloudAccountKey) return;
    setExportingCsv(true);
    try {
      const params = new URLSearchParams({ window_days: String(PAPER_EXPORT_WINDOW_DAYS) });
      const res = await fetch(`/api/paper-trades/export?${params.toString()}`, {
        credentials: "include",
        cache: "no-store",
      });
      if (!res.ok) {
        let message = `Export failed (${res.status})`;
        try {
          const body = (await res.json()) as { error?: string };
          if (body.error) message = body.error;
        } catch {
          // CSV error body unlikely
        }
        window.alert(message);
        return;
      }
      const blob = await res.blob();
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = objectUrl;
      anchor.download = `paper-trades-${PAPER_EXPORT_WINDOW_DAYS}d.csv`;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(objectUrl);
    } catch (e) {
      window.alert(e instanceof Error ? e.message : "Export failed");
    } finally {
      setExportingCsv(false);
    }
  }, [cloudAccountKey]);

  const sessionPnL = equity - baseBalance;
  const pnlPositive = sessionPnL >= 0;
  const totalReturn = ((equity - baseBalance) / baseBalance) * 100;

  const longCount = positions.filter(p => p.side === "LONG").length;
  const shortCount = positions.filter(p => p.side === "SHORT").length;
  const totalUnrealized = positions.reduce((s, p) => s + p.unrealizedPnl, 0);

  const sortedStrategies = [...strategyStatuses].sort((a, b) => b.score - a.score);
  const visibleStrategies = showAllStrategies ? sortedStrategies : sortedStrategies.slice(0, 12);

  const sortedTrades = [...trades].reverse();
  const visibleTrades = showAllTrades ? sortedTrades : sortedTrades.slice(0, 10);
  const visibleWatchlist = useMemo(() => {
    const q = watchSearch.trim().toLowerCase();
    if (!q) return watchlist;
    return watchlist.filter(
      (item) =>
        item.symbol.toLowerCase().includes(q) ||
        item.name.toLowerCase().includes(q),
    );
  }, [watchSearch, watchlist]);

  // Daily ledger
  const tradesByDay = trades.reduce((acc, t) => {
    const day = t.closedAt.split("T")[0];
    if (!acc[day]) acc[day] = { trades: 0, wins: 0, losses: 0, pnl: 0 };
    acc[day].trades++;
    if (t.netPnl > 0) acc[day].wins++;
    else acc[day].losses++;
    acc[day].pnl += t.netPnl;
    return acc;
  }, {} as Record<string, { trades: number; wins: number; losses: number; pnl: number }>);


  const watchColumns = useMemo((): DeskColumn<FuturesWatchItem>[] => [
    {
      id: "symbol",
      header: "Symbol",
      cell: (item) => (
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
            <span className="desk-body-md" style={{ fontWeight: 600 }}>{item.symbol}</span>
            <DeskChip tone="warning">{item.leverage}</DeskChip>
          </div>
          <p className="desk-label-md" style={{ marginTop: 4 }}>{item.name}</p>
        </div>
      ),
    },
    {
      id: "last",
      header: "Last",
      align: "right",
      cell: (item) => <span className="desk-mono">{item.lastPrice}</span>,
    },
    {
      id: "chg",
      header: "24h",
      align: "right",
      cell: (item) => {
        const positive = !item.change24h.startsWith("-");
        return (
          <span className={positive ? "desk-pnl-positive desk-mono" : "desk-pnl-negative desk-mono"}>
            {item.change24h}
          </span>
        );
      },
    },
    {
      id: "vol",
      header: "Volume",
      align: "right",
      cell: (item) => <span className="desk-mono">{item.volume24h}</span>,
    },
  ], []);

  const engineStatus: DeskEngineStatus = !authConfigured
    ? "cloud-off"
    : dataHealth.showFeedWarning
      ? "degraded"
      : pauseEntries
        ? "paused"
        : isReady
          ? "live"
          : "syncing";

  return (
    <DeskShell
      loading={!isReady || !quote}
      appBar={
        <DeskAppBar
          title={title}
          subtitle={`${moduleTagline} · ${watchlist.length} markets`}
          equity={equity}
          equityDetail={`Base ${formatDeskUsd(baseBalance, { decimals: 0 })} paper wallet`}
          status={engineStatus}
          authSlot={<PaperDeskAuthBar compact />}
          themeToggle={<DeskThemeToggle dark={deskDark} onToggle={() => setDeskDark((d) => !d)} />}
        />
      }
    >
      <DeskCard>
        <DeskSectionHeader
          title={title}
          subtitle={moduleTagline}
          actions={
            <>
              <DeskButton variant="tonal" onClick={togglePause}>
                {pauseEntries ? "Resume entries" : "Pause entries"}
              </DeskButton>
              <DeskButton variant="danger-tonal" onClick={resetPaperAccount}>
                Reset account
              </DeskButton>
              <DeskButton variant="outlined" onClick={clearTradeHistory}>
                Clear trades
              </DeskButton>
            </>
          }
        />
        <PaperDeskAuthBar />
      </DeskCard>

      {pauseEntries || stats.isDrawdownLocked ? (
        <DeskBanner variant="warning" title="Paper entries paused">
          {pauseEntries ? "Pause entries is on — no new paper opens until you resume. " : null}
          {stats.isDrawdownLocked
            ? `Drawdown lock active (${stats.drawdownPct.toFixed(1)}% vs session peak) — entries resume after partial recovery. `
            : null}
          This desk is paper-only; Testnet Ops does not auto-open these strategy slots.
        </DeskBanner>
      ) : null}

      <EntryDebugPanel
        entryDebug={entryDebug}
        pauseEntries={pauseEntries}
        drawdownLocked={stats.isDrawdownLocked}
        sessionSkips={{
          minMove: stats.deskSkippedMinExpectedMove,
          regime: stats.deskSkippedByRegime,
          spread: stats.deskSkippedSpread,
          session: stats.deskSkippedOutsideSession,
          category: stats.deskSkippedCategoryCap,
          lowPriority: stats.deskSkippedLowPriorityEntry,
          regimeBreakdown: stats.deskSkippedByRegimeBreakdown,
        }}
      />

      {dataHealth.showFeedWarning || dataHealth.lastError ? (
        <DeskBanner variant="warning" title="Futures kline feed is degraded">
          {dataHealth.lastError ? <span>{dataHealth.lastError} </span> : null}
          Signals may be stale until data recovers ({dataHealth.payloadsReady}/{dataHealth.symbolsRequested} symbols ready).
        </DeskBanner>
      ) : null}

      {quote ? (
        <div className="desk-metrics-row">
          <DeskMetricTile
            label="BTC mark"
            value={deskMounted ? formatDeskUsd(quote.markPrice, { decimals: 0 }) : "—"}
          />
          <DeskMetricTile
            label="24h change"
            value={deskMounted ? formatDeskPct(quote.changePct24h, { signed: true }) : "—"}
            valueClassName={deskMounted ? pnlToneClass(quote.changePct24h) : undefined}
          />
          <DeskMetricTile
            label="Funding"
            value={deskMounted ? formatDeskPct(quote.fundingRate * 100, { signed: true, decimals: 4 }) : "—"}
            valueClassName={deskMounted ? pnlToneClass(quote.fundingRate) : undefined}
          />
          <DeskMetricTile label="Markets" value={String(watchlist.length)} detail="On watchlist" compact />
        </div>
      ) : null}

      <DeskCard>
        <DeskSectionHeader
          title="Watchlist"
          subtitle="Perpetual futures symbols"
          actions={
            <DeskSearchField
              value={watchSearch}
              onChange={(e) => setWatchSearch(e.target.value)}
              placeholder="Search symbol or name"
              aria-label="Search watchlist"
            />
          }
        />
        <DeskDataTable
          minWidth={640}
          columns={watchColumns}
          rows={visibleWatchlist}
          getRowKey={(item) => item.symbol}
          empty={<DeskEmptyState title="No symbols match" subtitle="Try a different search term." />}
        />
      </DeskCard>

      <BTCFuturesDeskPanels
        title={title}
        baseBalance={baseBalance}
        equity={equity}
        sessionPnL={sessionPnL}
        totalReturn={totalReturn}
        pnlPositive={pnlPositive}
        stats={stats}
        isReady={isReady}
        longCount={longCount}
        shortCount={shortCount}
        totalUnrealized={totalUnrealized}
        sortedTrades={sortedTrades}
        trades={trades}
        strategyStatuses={strategyStatuses}
        visibleStrategies={visibleStrategies}
        showAllStrategies={showAllStrategies}
        setShowAllStrategies={setShowAllStrategies}
        positions={positions}
        visibleTrades={visibleTrades}
        showAllTrades={showAllTrades}
        setShowAllTrades={setShowAllTrades}
        tradesByDay={tradesByDay}
        cloudAccountKey={cloudAccountKey}
        exportingCsv={exportingCsv}
        onExportCsv={() => void downloadPaperTradesCsv()}
        disabledStrategies={disabledStrategies}
        setDisabledStrategies={setDisabledStrategies}
        deskShadowIntentsEnabled={deskShadowIntentsEnabled}
        deskTestnetOpsEnabled={deskTestnetOpsEnabled}
        researchMode={researchMode}
        researchPoolIds={researchPoolIds}
        researchWinners={researchWinners}
        researchRetiredIds={researchRetiredIds}
        onPromoteResearchWinner={onPromoteResearchWinner}
        onRetireResearchStrategy={onRetireResearchStrategy}
        onUnretireResearchStrategy={onUnretireResearchStrategy}
        onAutoRetireResearchLosers={onAutoRetireResearchLosers}
        storageNamespace={storageNamespace?.trim() || "btc_futures_scalper"}
      />

    </DeskShell>
  );
}
