"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { StrategyLeaderboardRow } from "@/lib/paperTradesAnalytics";
import { BTCFuturesScalperReadOnly } from "@/components/BTCFuturesScalperReadOnly";

import {
  useBTCFuturesScalperEngine,
  type BTCFuturesPosition,
  type BTCFuturesTrade,
  type BTCFuturesStrategyStatus,
  type BTCFuturesEngineOptions,
} from "@/hooks/useBTCFuturesScalperEngine";
import { FUTURES_WATCHLIST, type FuturesWatchItem } from "@/lib/futuresMarketData";
import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import { computeSessionEquityFromProduction, FUTURES_STRATEGY_PROFILES } from "@/lib/futuresSessionMetrics";
import { resolveCloudPaperTradesAccountKey } from "@/lib/paperTradesAuth";
import { PaperDeskAuthBar } from "@/components/PaperDeskAuthBar";
import { BTCFuturesDeskPanels } from "@/components/btcFutures/BTCFuturesDeskPanels";
import { EntryDebugPanel } from "@/components/btcFutures/EntryDebugPanel";
import { DeskHeroStrip } from "@/components/desk/DeskHeroStrip";
import { DeskThemeToggle } from "@/components/desk/DeskThemeToggle";
import { BTC_FT_DESK_BUILD, isDeploymentStale } from "@/lib/btcFtDeskBuild";
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
import { profitModeFromEnv } from "@/lib/futuresProfitMode";
import { deskUiCompactFromEnv } from "@/lib/deskUiCompact";
import { unifiedReadinessLabel } from "@/lib/futuresUnifiedReadiness";
import { ProfitModeChecklist } from "@/components/ProfitModeChecklist";
import { DeskRunModePanel } from "@/components/DeskRunModePanel";
import { buildChopCompatiblePaperRoster } from "@/lib/futuresDeskPolicy";
import { funnelSnapshotFromBrowserDebug } from "@/lib/deskEntryFunnelSnapshot";
import { isWorkerHeartbeatFresh } from "@/lib/paperDeskWorker/workerLease";
import { diagnoseNoTradeRootCause, type NoTradeRootCauseResult } from "@/lib/noTradeRootCause";

const deskTestnetOpsEnabled = process.env.NEXT_PUBLIC_DESK_TESTNET_OPS === "1";
const deskShadowIntentsEnabled = process.env.NEXT_PUBLIC_DESK_SHADOW_INTENTS === "1";
const ENGINE_AUTHORITY_CLIENT = process.env.NEXT_PUBLIC_ENGINE_AUTHORITY !== "0";


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
  /** Short status pills for the app bar (e.g. "20 strategies", "Paper", "BUILD abc"). */
  statusChips?: string[];
  /** Show every roster strategy as AVAILABLE and keep Active toggles on. */
  allStrategiesAvailable?: boolean;
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
  statusChips,
  allStrategiesAvailable = false,
}: BTCFuturesScalperProps = {}) {
  if (ENGINE_AUTHORITY_CLIENT) {
    return <BTCFuturesScalperReadOnly title={title} />;
  }

  const { user: authUser, configured: authConfigured } = usePaperDeskAuth();
  const deskMounted = useDeskMounted();
  const [deskDark, setDeskDark] = useState(false);

  const profitModeCfg = useMemo(() => profitModeFromEnv(), []);
  const uiCompact = useMemo(() => deskUiCompactFromEnv(profitModeCfg.enabled), [profitModeCfg.enabled]);

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
    updateSuspendedStrategies,
    restoreRotationStrategy,
    strategyStatuses,
    dataHealth,
    entryDebug,
    setReplaySignFlipRate,
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
    allStrategiesAvailable,
  });

  const [showAllStrategies, setShowAllStrategies] = useState(false);
  const [showAllTrades, setShowAllTrades] = useState(false);
  const [watchSearch, setWatchSearch] = useState("");
  const [exportingCsv, setExportingCsv] = useState(false);
  const exportInFlightRef = useRef(false);
  const [repairingState, setRepairingState] = useState(false);
  const [repairMessage, setRepairMessage] = useState<string | null>(null);
  const [serverBuildSha, setServerBuildSha] = useState<string | null>(null);
  const [noTradeRootCause, setNoTradeRootCause] = useState<NoTradeRootCauseResult | null>(null);
  const deploymentStale = isDeploymentStale(serverBuildSha);
  const chopCompatibleEnv = useMemo(
    () => `DESK_WORKER_STRATEGY_IDS=${buildChopCompatiblePaperRoster().join(",")}`,
    [],
  );

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

  // Fetch server-side build SHA once to detect stale Vercel deployments.
  useEffect(() => {
    fetch("/api/health/desk-worker", { cache: "no-store" })
      .then((r) => r.json())
      .then((data: { buildSha?: string }) => {
        if (data.buildSha) setServerBuildSha(data.buildSha);
      })
      .catch(() => {});
  }, []);

  const repairPaperState = useCallback(async () => {
    if (!cloudAccountKey || repairingState) return;
    setRepairingState(true);
    setRepairMessage(null);
    try {
      const res = await fetch("/api/paper-state/repair", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ accountKey: cloudAccountKey, initialBalance: baseBalance }),
        credentials: "include",
      });
      const data = (await res.json()) as { ok: boolean; message?: string; error?: string };
      if (!res.ok || !data.ok) {
        window.alert(data.error ?? "Repair failed");
        return;
      }
      setRepairMessage(data.message ?? "Paper state repaired.");
      // Let the engine re-hydrate from the new cleared_at
      clearTradeHistory();
    } catch (e) {
      window.alert(e instanceof Error ? e.message : "Repair failed");
    } finally {
      setRepairingState(false);
    }
  }, [cloudAccountKey, repairingState, baseBalance, clearTradeHistory]);

  // Production-aligned PnL excludes probe/bootstrap balance mutations.
  const productionPnl = computeSessionEquityFromProduction({
    initialBalance: baseBalance,
    productionClosedNetPnl: stats.realizedPnl,
    productionUnrealizedPnl: stats.unrealizedPnl,
  });
  const sessionPnL = stats.totalTrades === 0 && stats.openPositions === 0 ? 0 : productionPnl.sessionPnL;
  const pnlPositive = sessionPnL >= 0;
  const totalReturn = stats.totalTrades === 0 && stats.openPositions === 0 ? 0 : productionPnl.totalReturnPct;
  const balanceDriftUsd = stats.balanceDriftUsd ?? 0;

  // Probe-dominant: the last 5 closed trades are all probe/bootstrap trades.
  const probeDominant = useMemo(() => {
    if (trades.length === 0) return false;
    const recent = trades.slice(-5);
    return recent.every(
      (t) =>
        /BOOTSTRAP|PROBE|DEV_FORCE/i.test(t.strategyName ?? ""),
    );
  }, [trades]);

  useEffect(() => {
    if (!cloudAccountKey) return;
    let cancelled = false;
    const load = async () => {
      try {
        const q = `account_key=${encodeURIComponent(cloudAccountKey)}`;
        const [funnelRes, traceRes, healthRes, stateRes] = await Promise.all([
          fetch(`/api/desk-entry-funnel?${q}`, { cache: "no-store" }),
          fetch(`/api/strategy-signal-trace?${q}&limit=500`, { cache: "no-store" }),
          fetch("/api/health/desk-worker", { cache: "no-store" }),
          fetch(`/api/paper-state?${q}`, { cache: "no-store" }),
        ]);
        const [funnelJson, traceJson, healthJson, stateJson] = await Promise.all([
          funnelRes.json().catch(() => null),
          traceRes.json().catch(() => null),
          healthRes.json().catch(() => null),
          stateRes.json().catch(() => null),
        ]);
        if (cancelled) return;
        setNoTradeRootCause(
          diagnoseNoTradeRootCause({
            funnel: funnelJson?.snapshot ?? null,
            signalTrace: {
              summary: traceJson?.summary ?? null,
              rows: traceJson?.rows ?? [],
              ageSeconds: traceJson?.ageSeconds ?? null,
            },
            workerHealth: {
              stale: healthJson?.stale,
              workerLastPollAt: healthJson?.workerLastPollAt ?? null,
            },
            paperState: {
              ...(stateJson?.state ?? {}),
              balanceDriftUsd,
              probeDominant,
            },
            rotationReport: stats.rotationReport ?? null,
          }),
        );
      } catch {
        if (!cancelled) setNoTradeRootCause(null);
      }
    };
    void load();
    const id = window.setInterval(() => void load(), 10_000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [cloudAccountKey, balanceDriftUsd, probeDominant, stats.rotationReport]);

  // (whyNoTradesVisible removed — replaced by always-visible EntryFunnelCard)

  const { longCount, shortCount, totalUnrealized } = useMemo(() => ({
    longCount: positions.filter((p) => p.side === "LONG").length,
    shortCount: positions.filter((p) => p.side === "SHORT").length,
    totalUnrealized: positions.reduce((s, p) => s + p.unrealizedPnl, 0),
  }), [positions]);

  // Include roster strategies not wired into the live engine (e.g. research batch waitlist).
  const augmentedStrategyStatuses = useMemo<BTCFuturesStrategyStatus[]>(() => {
    const activeIds = new Set(strategyStatuses.map((s) => s.id));
    const poolOverlayIds = researchPoolIds && researchPoolIds.length > 0
      ? researchPoolIds
      : strategyIds && strategyIds.length > 0
        ? strategyIds
        : [];
    const waiting: BTCFuturesStrategyStatus[] = [];
    for (const id of poolOverlayIds) {
      if (activeIds.has(id)) continue;
      const def = FUTURES_STRAT_DEFS.find((d) => d.id === id);
      if (!def) continue;
      waiting.push({
        id: def.id,
        name: def.name,
        category: def.category,
        status: "AVAILABLE",
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
    return [...strategyStatuses, ...waiting];
  }, [strategyStatuses, researchPoolIds, strategyIds]);

  const sortedStrategies = useMemo(
    () => [...augmentedStrategyStatuses].sort((a, b) => b.score - a.score),
    [augmentedStrategyStatuses],
  );
  const visibleStrategies = useMemo(
    () => (showAllStrategies ? sortedStrategies : sortedStrategies.slice(0, 12)),
    [showAllStrategies, sortedStrategies],
  );

  const sortedTrades = useMemo(
    () =>
      [...trades].sort((a, b) => {
        const bTime = new Date(b.closedAt || b.openedAt).getTime();
        const aTime = new Date(a.closedAt || a.openedAt).getTime();
        return (Number.isFinite(bTime) ? bTime : 0) - (Number.isFinite(aTime) ? aTime : 0);
      }),
    [trades],
  );
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

  const appBarChips = useMemo(() => {
    if (statusChips && statusChips.length > 0) return statusChips;
    const count = strategyIds?.length ?? 0;
    return [
      "Paper",
      "25×",
      count > 0 ? `${count} strategies` : "No roster",
      BTC_FT_DESK_BUILD !== "dev" ? `BUILD ${BTC_FT_DESK_BUILD}` : "",
    ].filter(Boolean);
  }, [statusChips, strategyIds]);

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
          statusChips={appBarChips}
          equity={equity}
          equityDetail={
            deskMounted
              ? uiCompact
                ? unifiedReadinessLabel(stats.unifiedReadiness)
                : `Session ${pnlPositive ? "+" : ""}${formatDeskUsd(sessionPnL)} · ${formatDeskPct(totalReturn, { signed: true })}`
              : undefined
          }
          status={engineStatus}
          authSlot={<PaperDeskAuthBar compact />}
          themeToggle={<DeskThemeToggle dark={deskDark} onToggle={() => setDeskDark((d) => !d)} />}
        />
      }
    >
      {!uiCompact && (
        <DeskHeroStrip
          metrics={[
            {
              label: `Session PnL${deskMounted && stats.totalTrades > 0 ? ` (${stats.totalTrades} trades)` : ""}`,
              value: deskMounted ? formatDeskUsd(sessionPnL, { signed: true }) : "—",
              detail: deskMounted ? `${formatDeskPct(totalReturn, { signed: true })} return` : undefined,
              valueClassName: deskMounted ? pnlToneClass(sessionPnL) : undefined,
              highlight: true,
            },
            {
              label: "Equity",
              value: deskMounted ? formatDeskUsd(equity, { decimals: 0 }) : "—",
              detail: `Base ${formatDeskUsd(baseBalance, { decimals: 0 })}`,
            },
            {
              label: "Open positions",
              value: `${stats.openPositions} / ${stats.maxPositions}`,
              detail: `${longCount} long · ${shortCount} short`,
            },
            {
              label: "Win rate",
              value: deskMounted && stats.totalTrades > 0 ? formatDeskPct(stats.winRate) : "—",
              detail: deskMounted ? `${stats.totalTrades} closed trades` : undefined,
            },
          ]}
        />
      )}

      <div className="desk-toolbar">
        <p className="desk-label-md" style={{ margin: 0, maxWidth: "min(100%, 480px)" }}>
          {moduleTagline}
        </p>
        <div className="desk-toolbar__actions">
          <DeskButton variant="tonal" onClick={togglePause}>
            {pauseEntries ? "Resume entries" : "Pause entries"}
          </DeskButton>
          <DeskButton variant="danger-tonal" onClick={resetPaperAccount}>
            Reset account
          </DeskButton>
          <DeskButton variant="outlined" onClick={clearTradeHistory}>
            Clear trades
          </DeskButton>
          {cloudAccountKey && (
            <DeskButton
              variant="outlined"
              onClick={() => void repairPaperState()}
              disabled={repairingState}
            >
              {repairingState ? "Repairing…" : "Repair state"}
            </DeskButton>
          )}
        </div>
      </div>

      {deskMounted && deploymentStale && (
        <DeskBanner variant="warning" title="Stale deployment detected">
          The browser is running build <code>{BTC_FT_DESK_BUILD}</code> but the server reports{" "}
          <code>{serverBuildSha?.slice(0, 7)}</code>. Redeploy on Vercel and restart the VPS worker
          (<code>pm2 restart btc-ft-worker</code>) to pick up the latest code.
        </DeskBanner>
      )}

      {deskMounted && repairMessage && (
        <DeskBanner variant="info" title="Paper state repaired">
          {repairMessage}
        </DeskBanner>
      )}

      {deskMounted && balanceDriftUsd > 1 && (
        <DeskBanner variant="warning" title="Balance drift detected">
          Raw balance diverges from production trade sum by {formatDeskUsd(balanceDriftUsd)}. This usually means probe or bootstrap trades inflated the balance. Use <strong>Repair state</strong> to reset to clean accounting. PnL shown above is recomputed from closed production trades only.
        </DeskBanner>
      )}

      {process.env.NEXT_PUBLIC_DESK_WORKER_ENABLED === "1" && deskMounted && (
        <DeskRunModePanel
          workerLastPollAt={stats.workerLastPollAt}
          workerOwner={stats.workerOwner}
          workerMonitorMode={stats.workerMonitorMode}
        />
      )}

      {deskMounted && stats.workerMonitorMode && (
        <DeskBanner variant="info" title="24/7 worker active — browser in monitor mode">
          The VPS worker owns entry and exit execution. Last tick {
            stats.workerLastPollAt ? `${Math.round((Date.now() - stats.workerLastPollAt) / 1000)}s ago` : "unknown"
          }. Close this tab safely — trades continue on the VPS.
        </DeskBanner>
      )}

      {deskMounted && noTradeRootCause && stats.openPositions === 0 && (
        <DeskBanner
          variant={noTradeRootCause.canOpenIfSignalQualifies ? "info" : "warning"}
          title={`No-trade root cause: ${noTradeRootCause.rootCause}`}
        >
          {noTradeRootCause.safeFix}
          {noTradeRootCause.evidence.length > 0 ? (
            <span> Evidence: {noTradeRootCause.evidence.slice(0, 3).join(" · ")}</span>
          ) : null}
          {noTradeRootCause.rootCause === "REGIME_BLOCKING" ? (
            <span> Suggested paper roster: <code>{chopCompatibleEnv}</code></span>
          ) : null}
        </DeskBanner>
      )}

      {process.env.NEXT_PUBLIC_DESK_WORKER_ENABLED === "1" && deskMounted && !stats.workerMonitorMode && (
        <DeskBanner variant="warning" title="Browser-only execution — keep tab visible">
          No active 24/7 worker detected. Desk pauses when this tab is hidden or the PC sleeps.
          Start the worker: <code>cd client {"&&"} npx tsx scripts/btc-ft-paper-worker.ts</code>
        </DeskBanner>
      )}

      {pauseEntries || stats.isDrawdownLocked ? (
        <DeskBanner variant="warning" title="Paper entries paused">
          {pauseEntries ? "Pause entries is on — no new paper opens until you resume. " : null}
          {stats.isDrawdownLocked
            ? `Drawdown lock active (${stats.drawdownPct.toFixed(1)}% vs session peak) — entries resume after partial recovery. `
            : null}
          This desk is paper-only; Testnet Ops does not auto-open these strategy slots.
        </DeskBanner>
      ) : null}

      {deskMounted && probeDominant && (
        <DeskBanner variant="warning" title="Probe trades dominating history">
          The last 5 closed trades are all probe or bootstrap entries. These are excluded from PnL and win-rate metrics but inflate the raw balance. Use <strong>Repair state</strong> to reset balance/positions to a clean baseline (historical trade records are kept).
        </DeskBanner>
      )}

      {/* ── Entry Funnel Diagnostic Card ─────────────────────────────────────
           Always visible when engine is live. Shows all 10 fields so the
           operator never has to guess why trades are or are not opening.
           Recommendations are action-oriented; none suggest lowering threshold.
      ─────────────────────────────────────────────────────────────────────── */}
      {deskMounted && isReady && (
        <EntryFunnelCard
          entryDebug={entryDebug}
          workerLastPollAt={stats.workerLastPollAt}
          workerEnabled={process.env.NEXT_PUBLIC_DESK_WORKER_ENABLED === "1"}
          workerMonitorMode={stats.workerMonitorMode}
          markPrice={quote?.markPrice ?? 0}
          balanceDriftUsd={balanceDriftUsd}
          probeDominant={probeDominant}
          deskSkippedByRegime={stats.deskSkippedByRegime}
          deskLastRegimeTag={stats.deskLastRegimeTag ?? null}
          deskSkippedMinExpectedMove={stats.deskSkippedMinExpectedMove}
          rotationSuspendedCount={stats.rotationReport?.suspended?.length ?? 0}
          deskSkippedOutsideSession={stats.deskSkippedOutsideSession}
          deskSkippedSpread={stats.deskSkippedSpread}
        />
      )}

      {deskMounted && (
        <TradePlacementVerificationPanel
          workerFresh={isWorkerHeartbeatFresh(stats.workerLastPollAt)}
          mongoConnected={Boolean(cloudAccountKey)}
          rosterNonEmpty={(strategyIds?.length ?? strategyStatuses.length) > 0}
          dataReady={isReady && (quote?.markPrice ?? 0) > 0}
          evaluatedStrategies={entryDebug?.evalPairs ?? 0}
          signalTraceFresh={noTradeRootCause?.evidence.some((x) => x.includes("trace evaluated=")) ?? false}
          stateClean={balanceDriftUsd <= 1 && !probeDominant}
          capClear={stats.openPositions < stats.maxPositions}
          smokeTestEnabled={process.env.NEXT_PUBLIC_DESK_SMOKE_TEST === "1"}
        />
      )}

      {/* In compact mode entry debug moves to Command Center Advanced tab */}
      {!uiCompact && (
        <EntryDebugPanel
          entryDebug={entryDebug}
          pauseEntries={pauseEntries}
          drawdownLocked={stats.isDrawdownLocked}
          profitModeEnabled={profitModeCfg.enabled}
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
      )}

      {uiCompact && profitModeCfg.enabled && stats.totalTrades < 10 && (
        <ProfitModeChecklist
          storageNamespace={storageNamespace?.trim() || "btc_futures_scalper"}
          totalTrades={stats.totalTrades}
        />
      )}

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
            highlight
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
          <DeskMetricTile
            label="Unrealized"
            value={deskMounted ? formatDeskUsd(totalUnrealized, { signed: true }) : "—"}
            valueClassName={deskMounted ? pnlToneClass(totalUnrealized) : undefined}
            compact
          />
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
        onRotationReport={updateSuspendedStrategies}
        onRestoreRotationStrategy={restoreRotationStrategy}
        setReplaySignFlipRate={setReplaySignFlipRate}
        entryDebug={entryDebug}
        pauseEntries={pauseEntries}
        probeDominant={probeDominant}
        workerEnabled={process.env.NEXT_PUBLIC_DESK_WORKER_ENABLED === "1"}
        workerStale={stats.workerLastPollAt == null || !stats.workerMonitorMode}
        serverBuildSha={serverBuildSha}
        enabledStrategyIds={strategyIds ?? []}
      />

    </DeskShell>
  );
}

// ── EntryFunnelCard ──────────────────────────────────────────────────────────
// Always-visible root-cause panel. Shows all 10 fields from the entry funnel
// so the operator knows exactly why trades are or are not opening each tick.
// Recommendations are explicit and action-oriented; none suggest lowering the
// signal threshold.
// ─────────────────────────────────────────────────────────────────────────────

const FUNNEL_BLOCKER_ACTIONS: Record<string, string> = {
  noData: "Delta candle/mark feed unavailable — check DELTA_API_BASE_URL env or Delta Exchange status.",
  noStrategies: "Roster/env strategy IDs invalid — check DESK_WORKER_STRATEGY_IDS matches FUTURES_STRAT_DEFS.",
  signal: "No strategy crossed signal threshold this tick. Wait for a stronger market setup.",
  confirm: "Signal fired but entry confirmation failed. Wait for all confirmation factors to align.",
  regime: "Current regime blocks this roster. Wait for a trend/chop-compatible setup.",
  atrFees: "Expected move too small vs fees — correct no-trade. Wait for higher-volatility setups.",
  rotation: "Rotation gate is blocking PROBATION/SUSPENDED strategies. Inspect Edge Candidates panel.",
  suspended: "Rotation has suspended most strategies. Wait for performance recovery.",
  spread: "Last/mark spread too wide for safe entry. Wait for tighter market conditions.",
  session: "Outside UTC entry session window. Entries resume when the configured session opens.",
  category: "Per-category concurrent position cap is full. Wait for existing positions to close.",
  sameSide: "Same-side correlation cap reached. Wait for directional balance.",
  margin: "Insufficient margin. Repair paper state or wait for existing positions to close.",
  maxOpen: "Maximum concurrent positions reached. Wait for exits before new entries.",
  cooldown: "All qualifying strategies are in cooldown. Wait for cooldown expiry.",
  none: "Conditions look favorable. Monitor for signal qualification on the next poll.",
};

function FunnelRow({ label, value, highlight }: { label: string; value: string | number; highlight?: boolean }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "3px 0", borderBottom: "1px solid var(--desk-outline, rgba(255,255,255,0.07))" }}>
      <span style={{ color: "#8b949e", fontSize: 11, minWidth: 140 }}>{label}</span>
      <span style={{ fontWeight: highlight ? 700 : 400, fontSize: 12, color: highlight ? "#e6edf3" : "#c9d1d9", textAlign: "right" }}>{value}</span>
    </div>
  );
}

function TradePlacementVerificationPanel({
  workerFresh,
  mongoConnected,
  rosterNonEmpty,
  dataReady,
  evaluatedStrategies,
  signalTraceFresh,
  stateClean,
  capClear,
  smokeTestEnabled,
}: {
  workerFresh: boolean;
  mongoConnected: boolean;
  rosterNonEmpty: boolean;
  dataReady: boolean;
  evaluatedStrategies: number;
  signalTraceFresh: boolean;
  stateClean: boolean;
  capClear: boolean;
  smokeTestEnabled: boolean;
}) {
  const items = [
    ["Worker fresh", workerFresh],
    ["Mongo connected", mongoConnected],
    ["Roster non-empty", rosterNonEmpty],
    ["Data bars >= minimum", dataReady],
    ["Evaluated strategies > 0", evaluatedStrategies > 0],
    ["Signal trace fresh", signalTraceFresh],
    ["State clean", stateClean],
    ["No open-position cap", capClear],
    [`Smoke test ${smokeTestEnabled ? "available" : "off"}`, !smokeTestEnabled],
  ] as const;

  return (
    <DeskCard>
      <DeskSectionHeader
        title="Trade Placement Verification"
        subtitle="Paper-only checks that prove whether execution plumbing or gates are blocking entries."
      />
      <div className="desk-metrics-row">
        {items.map(([label, ok]) => (
          <DeskChip key={label} tone={ok ? "success" : "error"}>
            {ok ? "OK" : "CHECK"} {label}
          </DeskChip>
        ))}
      </div>
    </DeskCard>
  );
}

function EntryFunnelCard({
  entryDebug,
  workerLastPollAt,
  workerEnabled,
  workerMonitorMode,
  markPrice,
  balanceDriftUsd,
  probeDominant,
  deskSkippedByRegime,
  deskLastRegimeTag,
  deskSkippedMinExpectedMove,
  rotationSuspendedCount,
  deskSkippedOutsideSession,
  deskSkippedSpread,
}: {
  entryDebug: import("@/lib/futuresEntryDebug").DeskEntryPollDebug | null;
  workerLastPollAt: number | null;
  workerEnabled: boolean;
  workerMonitorMode: boolean;
  markPrice: number;
  balanceDriftUsd: number;
  probeDominant: boolean;
  deskSkippedByRegime: number;
  deskLastRegimeTag: string | null;
  deskSkippedMinExpectedMove: number;
  rotationSuspendedCount: number;
  deskSkippedOutsideSession: number;
  deskSkippedSpread: number;
}) {
  const workerFresh = isWorkerHeartbeatFresh(workerLastPollAt);

  // Derive funnel snapshot from browser debug when available.
  const snap = entryDebug
    ? funnelSnapshotFromBrowserDebug(entryDebug, {
        workerFresh,
        symbol: "BTCUSD",
        markPrice,
        bars: 0,
      })
    : null;

  // Compute signal/confirm passed from raw debug counts (cleaner than snap).
  const signalPassed = entryDebug
    ? Math.max(0, (entryDebug.candidatesBuilt ?? 0) + (entryDebug.failConfirm ?? 0))
    : null;
  const confirmPassed = entryDebug ? (entryDebug.candidatesBuilt ?? 0) : null;

  // Display blocker — string (not EntryFunnelBlockerKey) so we can inject
  // structural overrides (workerStale, repairRequired) that trump the pipeline.
  let displayBlocker: string = snap?.dominantBlocker ?? "none";
  let recommendation: string = FUNNEL_BLOCKER_ACTIONS[displayBlocker] ?? FUNNEL_BLOCKER_ACTIONS.none;

  if (workerEnabled && !workerFresh) {
    displayBlocker = "workerStale";
    recommendation = "Restart AWS pm2 worker: ssh vps → pm2 restart btc-ft-worker";
  } else if (balanceDriftUsd > 1 || probeDominant) {
    displayBlocker = "repairRequired";
    recommendation = "Repair paper state required before judging entries. Historical trades are kept.";
  }

  const blockerColor =
    displayBlocker === "none" ? "#3fb950" :
    displayBlocker === "workerStale" || displayBlocker === "repairRequired" ? "#f85149" :
    displayBlocker === "signal" || displayBlocker === "regime" || displayBlocker === "atrFees" ? "#d29922" :
    "#58a6ff";

  return (
    <DeskBanner variant="info" title="Why no BTC trades? — entry funnel">
      <div style={{ display: "flex", gap: 24, flexWrap: "wrap", alignItems: "flex-start" }}>
        {/* Left column: funnel field table */}
        <div style={{ flex: "1 1 260px", minWidth: 220 }}>
          <FunnelRow
            label="Worker status"
            value={
              !workerEnabled ? "browser-only" :
              workerFresh ? `fresh (${workerLastPollAt ? Math.round((Date.now() - workerLastPollAt) / 1000) : "?"}s ago)` :
              "STALE — restart pm2"
            }
            highlight={workerEnabled && !workerFresh}
          />
          <FunnelRow label="Active strategies" value={snap?.activeStrategies ?? (entryDebug?.activeStratCount ?? "—")} />
          <FunnelRow label="Evaluated strategies" value={snap?.evaluatedStrategies ?? (entryDebug?.evalPairs ?? "—")} />
          <FunnelRow label="Signal passed" value={signalPassed ?? "—"} />
          <FunnelRow label="Confirm passed" value={confirmPassed ?? "—"} />
          <FunnelRow label="Candidates" value={entryDebug?.candidatesBuilt ?? "—"} />
          <FunnelRow label="Open attempts" value={entryDebug?.openAttempts ?? "—"} />
          <FunnelRow label="Opened last tick" value={entryDebug?.openedThisPoll ?? "—"} />
          {deskSkippedByRegime > 0 && <FunnelRow label={`Regime blocks (${deskLastRegimeTag ?? "?"})`} value={deskSkippedByRegime} />}
          {deskSkippedMinExpectedMove > 0 && <FunnelRow label="ATR/fee blocks" value={deskSkippedMinExpectedMove} />}
          {rotationSuspendedCount > 0 && <FunnelRow label="Rotation suspended" value={rotationSuspendedCount} />}
          {deskSkippedOutsideSession > 0 && <FunnelRow label="Session gate blocks" value={deskSkippedOutsideSession} />}
          {deskSkippedSpread > 0 && <FunnelRow label="Spread blocks" value={deskSkippedSpread} />}
        </div>

        {/* Right column: dominant blocker + recommendation */}
        <div style={{ flex: "1 1 240px", minWidth: 200 }}>
          <div style={{ marginBottom: 8 }}>
            <span style={{ fontSize: 10, color: "#8b949e", textTransform: "uppercase", letterSpacing: 1 }}>Dominant gate</span>
            <div style={{ fontSize: 18, fontWeight: 700, color: blockerColor, marginTop: 2 }}>{displayBlocker}</div>
          </div>
          <div style={{ fontSize: 12, color: "#e6edf3", lineHeight: 1.5, background: "rgba(0,0,0,0.15)", borderRadius: 6, padding: "8px 10px" }}>
            {recommendation}
          </div>
          {snap && (
            <div style={{ marginTop: 8, fontSize: 10, color: "#8b949e" }}>
              Last tick: {snap.tickAt ? new Date(snap.tickAt).toLocaleTimeString() : "—"}
              {" · "}{snap.workerMode} mode
            </div>
          )}
        </div>
      </div>
    </DeskBanner>
  );
}
