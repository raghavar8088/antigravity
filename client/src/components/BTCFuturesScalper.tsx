"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { StrategyLeaderboardRow } from "@/lib/paperTradesAnalytics";
import {
  useBTCFuturesScalperEngine,
  type BTCFuturesPosition,
  type BTCFuturesTrade,
  type BTCFuturesStrategyStatus,
  type BTCFuturesEngineOptions,
} from "@/hooks/useBTCFuturesScalperEngine";
import { FUTURES_WATCHLIST, type FuturesWatchItem } from "@/lib/futuresMarketData";
import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import { FUTURES_STRATEGY_PROFILES } from "@/lib/futuresSessionMetrics";
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
  paperEnsureTrades?: BTCFuturesEngineOptions["paperEnsureTrades"];
  paperBootstrapProbe?: BTCFuturesEngineOptions["paperBootstrapProbe"];
  entryDebugEnabled?: BTCFuturesEngineOptions["entryDebugEnabled"];
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
  /**
   * Module identifier persisted with closed paper trades so the dashboard can
   * filter per-tab leaderboards and exports. Pass `"btc_futures_scalper"` from
   * the multi-symbol desk; `"btc_future_trading"` from the BTC FT wrapper.
   */
  moduleKey?: import("@/lib/paperTradesTypes").PaperTradeModuleKey;
  /** Promoted strategy IDs — premium strats only receive 2× notional when their ID appears here. */
  promotedStrategyIds?: BTCFuturesEngineOptions["promotedStrategyIds"];
};

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
  paperEnsureTrades,
  paperBootstrapProbe,
  entryDebugEnabled,
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
  moduleKey = "btc_futures_scalper",
  promotedStrategyIds,
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
    paperEnsureTrades,
    paperBootstrapProbe,
    entryDebugEnabled,
    entryUtcSessionOverride,
    storageNamespace,
    supabaseUserId: authUser?.id ?? null,
    moduleKey,
    promotedStrategyIds,
  });

  const [showAllStrategies, setShowAllStrategies] = useState(false);
  const [showAllTrades, setShowAllTrades] = useState(false);
  const [watchSearch, setWatchSearch] = useState("");
  const [exportingCsv, setExportingCsv] = useState(false);
  const exportInFlightRef = useRef(false);

  const downloadPaperTradesCsv = useCallback(async () => {
    if (!cloudAccountKey || exportInFlightRef.current) return;
    exportInFlightRef.current = true;
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
      exportInFlightRef.current = false;
    }
  }, [cloudAccountKey]);

  const sessionPnL = equity - baseBalance;
  const pnlPositive = sessionPnL >= 0;
  const totalReturn = ((equity - baseBalance) / baseBalance) * 100;

  const { longCount, shortCount, totalUnrealized } = useMemo(() => ({
    longCount: positions.filter((p) => p.side === "LONG").length,
    shortCount: positions.filter((p) => p.side === "SHORT").length,
    totalUnrealized: positions.reduce((s, p) => s + p.unrealizedPnl, 0),
  }), [positions]);

  // POOL overlay only in research mode. Production / winners-only shows active roster only.
  const augmentedStrategyStatuses = useMemo<BTCFuturesStrategyStatus[]>(() => {
    const activeIds = new Set(strategyStatuses.map((s) => s.id));
    const poolOverlayIds = researchPoolIds && researchPoolIds.length > 0
      ? researchPoolIds
      : strategyIds && strategyIds.length > 0
        ? strategyIds
        : [];
    const poolOnly: BTCFuturesStrategyStatus[] = [];
    for (const id of poolOverlayIds) {
      if (activeIds.has(id)) continue;
      const def = FUTURES_STRAT_DEFS.find((d) => d.id === id);
      if (!def) continue;
      poolOnly.push({
        id: def.id,
        name: def.name,
        category: def.category,
        status: "POOL",
        disabled: false,
        openCount: 0,
        lastTradeAt: null,
        score: 0,
        totalTrades: 0,
        wins: 0,
        losses: 0,
        totalPnl: 0,
        winRate: 0,
      });
    }
    return [...strategyStatuses, ...poolOnly];
  }, [strategyStatuses, researchPoolIds]);

  const sortedStrategies = useMemo(
    () =>
      [...augmentedStrategyStatuses].sort((a, b) => {
        // Active roster entries first (POOL last), then by score descending
        const aPool = a.status === "POOL" ? 1 : 0;
        const bPool = b.status === "POOL" ? 1 : 0;
        if (aPool !== bPool) return aPool - bPool;
        return b.score - a.score;
      }),
    [augmentedStrategyStatuses],
  );
  const visibleStrategies = useMemo(
    () => (showAllStrategies ? sortedStrategies : sortedStrategies.slice(0, 12)),
    [showAllStrategies, sortedStrategies],
  );

  const sortedTrades = useMemo(() => [...trades].reverse(), [trades]);
  const visibleTrades = useMemo(
    () => (showAllTrades ? sortedTrades : sortedTrades.slice(0, 10)),
    [showAllTrades, sortedTrades],
  );
  const visibleWatchlist = useMemo(() => {
    const q = watchSearch.trim().toLowerCase();
    if (!q) return watchlist;
    return watchlist.filter(
      (item) =>
        item.symbol.toLowerCase().includes(q) ||
        item.name.toLowerCase().includes(q),
    );
  }, [watchSearch, watchlist]);

  const tradesByDay = useMemo(
    () =>
      trades.reduce<Record<string, { trades: number; wins: number; losses: number; pnl: number }>>(
        (acc, t) => {
          const day = t.closedAt.split("T")[0];
          if (!acc[day]) acc[day] = { trades: 0, wins: 0, losses: 0, pnl: 0 };
          acc[day].trades++;
          if (t.netPnl > 0) acc[day].wins++;
          else acc[day].losses++;
          acc[day].pnl += t.netPnl;
          return acc;
        },
        {},
      ),
    [trades],
  );


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
        strategyStatuses={augmentedStrategyStatuses}
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
