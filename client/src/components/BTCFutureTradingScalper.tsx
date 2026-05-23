"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { BTCFuturesScalper } from "@/components/BTCFuturesScalper";
import type { BTCFuturesEngineOptions } from "@/hooks/useBTCFuturesScalperEngine";
import { usePaperDeskAuth } from "@/hooks/usePaperDeskAuth";
import { resolveCloudPaperTradesAccountKey } from "@/lib/paperTradesAuth";
import { getOrCreateAnonAccountKey } from "@/lib/anonAccountKey";
import {
  btcFtEntryDebugEnabledFromEnv,
  btcFtMinMoveKMulFromEnv,
  btcFtPaperEnsureTradesFromEnv,
  btcFtSignalThresholdFromEnv,
} from "@/lib/futuresDeskPolicy";
import { FUTURES_WATCHLIST } from "@/lib/futuresMarketData";
import { DeskBanner } from "@/components/desk/ui";
import {
  isResearchModeEnabled,
  isWinnersOnlyModeEnabled,
  loadResearchStateFromMongo,
  researchCooldownMul,
  researchDisableAutoKill,
  researchEnsureTradesEnabled,
  researchEntryUtcSession,
  researchMinMoveKMul,
  researchRelaxConfirm,
  researchSignalThreshold,
  researchSlippageBps,
  resolveBtcFtActiveStrategyIds,
  resolveCoreWinners,
  resolveResearchActiveIds,
  resolveResearchPool,
  poolGeneratedCount,
  saveRetiredToStorage,
  saveWinnersToStorage,
} from "@/lib/btcFtResearch";
import { BTC_FT_DESK_BUILD } from "@/lib/btcFtDeskBuild";

// ── Constants ─────────────────────────────────────────────────────────────────
const BTC_ONLY_SYMBOLS = ["BTCUSD"] as const;
const BTC_ONLY_WATCHLIST = FUTURES_WATCHLIST.filter((item) => item.symbol === "BTCUSD");
const STORAGE_NS = "btc_future_trading_v4";

// ── Types ─────────────────────────────────────────────────────────────────────
type RosterInfo = {
  ids: number[];
  source: string;
  isLargeRoster: boolean;
  batchIndex: number | null;
  totalBatches: number | null;
  poolSize: number | null;
};

// ── Hook: lazy env reads ──────────────────────────────────────────────────────
function useResolvedModes() {
  return useMemo(() => {
    const research = isResearchModeEnabled();
    const winnersOnly = isWinnersOnlyModeEnabled();
    return {
      RESEARCH_MODE: research,
      WINNERS_ONLY_MODE: winnersOnly,
      EFFECTIVE_RESEARCH_MODE: research && !winnersOnly,
    };
  }, []);
}

// ── Component ─────────────────────────────────────────────────────────────────
export function BTCFutureTradingScalper({
  strategyProfile: strategyProfileProp,
}: {
  strategyProfile?: BTCFuturesEngineOptions["strategyProfile"];
} = {}) {
  // scalp_aggro_v1 lowers the signal threshold by 4 — too aggressive for the
  // production BTC FT desk. Remap it to baseline so a misconfigured prop
  // cannot override the production gate.
  const strategyProfile = strategyProfileProp === "scalp_aggro_v1" ? "baseline" : strategyProfileProp;
  const auth = usePaperDeskAuth();
  const { RESEARCH_MODE, WINNERS_ONLY_MODE, EFFECTIVE_RESEARCH_MODE } = useResolvedModes();

  // Resolve the same account key the engine will use so research state is co-located
  const accountKey = useMemo(
    () =>
      resolveCloudPaperTradesAccountKey({
        supabaseUserId: auth.user?.id,
        storageNamespace: STORAGE_NS,
      }) ?? getOrCreateAnonAccountKey() ?? "",
    [auth.user?.id],
  );

  // State — default empty; loaded from MongoDB in useEffect
  const [retiredIds, setRetiredIds] = useState<Set<number>>(new Set());
  const [winners, setWinners] = useState<number[]>([]);
  const [researchLoaded, setResearchLoaded] = useState(false);

  // Load research state from MongoDB on mount
  useEffect(() => {
    if (!RESEARCH_MODE && !WINNERS_ONLY_MODE) {
      // Non-research: use core winners from env (no async needed)
      setWinners(resolveCoreWinners(STORAGE_NS));
      setResearchLoaded(true);
      return;
    }
    let cancelled = false;
    void loadResearchStateFromMongo(STORAGE_NS, accountKey).then(({ winners: w, retiredIds: r }) => {
      if (cancelled) return;
      // Also migrate any old localStorage data
      if (typeof localStorage !== "undefined") {
        try {
          const legacyWinnersRaw = localStorage.getItem(`${STORAGE_NS}_btc_ft_winners`);
          const legacyRetiredRaw = localStorage.getItem(`${STORAGE_NS}_btc_ft_retired`);
          if (legacyWinnersRaw) {
            const legacyW = JSON.parse(legacyWinnersRaw) as number[];
            if (Array.isArray(legacyW) && legacyW.length > 0) {
              const merged = [...new Set([...w, ...legacyW])].slice(0, 20);
              saveWinnersToStorage(STORAGE_NS, merged, accountKey);
              w = merged;
            }
            localStorage.removeItem(`${STORAGE_NS}_btc_ft_winners`);
          }
          if (legacyRetiredRaw) {
            const legacyR = new Set(JSON.parse(legacyRetiredRaw) as number[]);
            if (legacyR.size > 0) {
              const merged = new Set([...r, ...legacyR]);
              saveRetiredToStorage(STORAGE_NS, merged, accountKey);
              r = merged;
            }
            localStorage.removeItem(`${STORAGE_NS}_btc_ft_retired`);
          }
        } catch { /* ignore corrupt legacy */ }
      }
      setWinners(w);
      setRetiredIds(r);
      setResearchLoaded(true);
    });
    return () => { cancelled = true; };
  }, [RESEARCH_MODE, WINNERS_ONLY_MODE, accountKey]);

  // Cloud-sync promoted winners for WINNERS_ONLY mode
  useEffect(() => {
    if (!WINNERS_ONLY_MODE || !researchLoaded) return;
    let cancelled = false;
    void loadResearchStateFromMongo(STORAGE_NS, accountKey).then(({ winners: w }) => {
      if (!cancelled && w.length > 0) setWinners(w);
    });
    return () => { cancelled = true; };
  }, [WINNERS_ONLY_MODE, researchLoaded, accountKey, auth.user?.id]);

  // Persist winners to MongoDB whenever they change
  useEffect(() => {
    if (!researchLoaded) return;
    if (RESEARCH_MODE || WINNERS_ONLY_MODE) saveWinnersToStorage(STORAGE_NS, winners, accountKey);
  }, [RESEARCH_MODE, WINNERS_ONLY_MODE, winners, accountKey, researchLoaded]);

  // Persist retired IDs to MongoDB whenever they change
  useEffect(() => {
    if (!researchLoaded || !EFFECTIVE_RESEARCH_MODE) return;
    saveRetiredToStorage(STORAGE_NS, retiredIds, accountKey);
  }, [EFFECTIVE_RESEARCH_MODE, retiredIds, accountKey, researchLoaded]);

  // ── Roster resolution ──────────────────────────────────────────────────────
  const rosterInfo = useMemo((): RosterInfo => {
    if (WINNERS_ONLY_MODE) {
      const r = resolveBtcFtActiveStrategyIds({ storageNamespace: STORAGE_NS, winnerIds: winners });
      return { ids: r.ids, source: r.source, isLargeRoster: r.isLargeRoster, batchIndex: null, totalBatches: null, poolSize: null };
    }
    if (!EFFECTIVE_RESEARCH_MODE) {
      const r = resolveBtcFtActiveStrategyIds({ storageNamespace: STORAGE_NS });
      return { ids: r.ids, source: r.source, isLargeRoster: r.isLargeRoster, batchIndex: null, totalBatches: null, poolSize: null };
    }
    const pool = resolveResearchPool();
    const batch = resolveResearchActiveIds({ pool, retiredIds, batchSize: 30, rotateEveryHours: 24 });
    return {
      ids: batch.activeIds,
      source: "research" as const,
      isLargeRoster: false,
      batchIndex: batch.batchIndex,
      totalBatches: batch.totalBatches,
      poolSize: batch.poolSize,
    };
  }, [retiredIds, winners, WINNERS_ONLY_MODE, EFFECTIVE_RESEARCH_MODE]);

  // ── Derived engine options ─────────────────────────────────────────────────
  const threshold = useMemo(() => {
    if (WINNERS_ONLY_MODE) return btcFtSignalThresholdFromEnv(26);
    if (EFFECTIVE_RESEARCH_MODE) return researchSignalThreshold();
    return btcFtSignalThresholdFromEnv(20);
  }, [WINNERS_ONLY_MODE, EFFECTIVE_RESEARCH_MODE]);

  const relaxConfirm = useMemo(() => {
    if (EFFECTIVE_RESEARCH_MODE) return researchRelaxConfirm();
    return true;
  }, [EFFECTIVE_RESEARCH_MODE]);

  const minMoveKMul = useMemo(() => {
    if (WINNERS_ONLY_MODE) return btcFtMinMoveKMulFromEnv(1.0);
    if (EFFECTIVE_RESEARCH_MODE) return researchMinMoveKMul();
    return btcFtMinMoveKMulFromEnv(0.45);
  }, [WINNERS_ONLY_MODE, EFFECTIVE_RESEARCH_MODE]);

  const paperEnsureTrades =
    !EFFECTIVE_RESEARCH_MODE && !WINNERS_ONLY_MODE && btcFtPaperEnsureTradesFromEnv();

  const entryDebugEnabled = btcFtEntryDebugEnabledFromEnv() || rosterInfo.isLargeRoster;

  // ── Stable callbacks ───────────────────────────────────────────────────────
  const handlePromote = useCallback((id: number) => {
    setWinners((prev) => [...new Set([...prev, id])].slice(0, 20));
  }, []);

  const handleRetire = useCallback((id: number) => {
    setRetiredIds((prev) => new Set([...prev, id]));
  }, []);

  const handleUnretire = useCallback((id: number) => {
    setRetiredIds((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  }, []);

  const handleAutoRetireLosers = useCallback((ids: number[]) => {
    setRetiredIds((prev) => new Set([...prev, ...ids]));
  }, []);

  // ── Labels ─────────────────────────────────────────────────────────────────
  const sourceLabel = useMemo(() => {
    const map: Record<string, string> = {
      research: `research batch ${(rosterInfo.batchIndex ?? 0) + 1}/${rosterInfo.totalBatches ?? 1}`,
      "winners-empty": "no winners",
      winners: "winners",
      env: "env",
      "core+ranked": "core + ranked",
      full: "full roster",
      core: "core",
    };
    return map[rosterInfo.source] ?? rosterInfo.source;
  }, [rosterInfo]);

  const tagline = useMemo(() => {
    const count = rosterInfo.ids.length;
    if (WINNERS_ONLY_MODE) return `BTC PERPETUAL FUTURES - 25x - ${count} WINNER STRATEGIES - THRESHOLD ${threshold} - PRODUCTION`;
    if (EFFECTIVE_RESEARCH_MODE) return `BTC PERPETUAL FUTURES - 25x - ${count} ACTIVE OF ${rosterInfo.poolSize ?? count} POOL - THRESHOLD ${threshold} - RESEARCH`;
    return `BTC PERPETUAL FUTURES - 25x - ${count} STRATEGIES (${sourceLabel.toUpperCase()}) - THRESHOLD ${threshold} - BUILD ${BTC_FT_DESK_BUILD}`;
  }, [WINNERS_ONLY_MODE, EFFECTIVE_RESEARCH_MODE, rosterInfo, threshold, sourceLabel]);

  const shouldRenderEngine = !WINNERS_ONLY_MODE || rosterInfo.ids.length > 0;

  return (
    <>
      <ResearchBanners
        researchMode={EFFECTIVE_RESEARCH_MODE}
        rosterInfo={rosterInfo}
        threshold={threshold}
        relaxConfirm={relaxConfirm}
        winnersCount={winners.length}
        generatedCount={poolGeneratedCount()}
      />

      {WINNERS_ONLY_MODE && rosterInfo.ids.length > 0 && (
        <DeskBanner variant="info" title={`Production winners mode - ${rosterInfo.ids.length} strategies`}>
          Running only promoted BTC FT winners with production gates: threshold 26, relax-confirm OFF,
          cooldown 1x, min-move K 1x, and auto-kill ON. No Delta mainnet orders are enabled by this mode.
        </DeskBanner>
      )}

      {process.env.NEXT_PUBLIC_BTC_FT_FIREHOSE === "1" && (
        <DeskBanner variant="warning" title="⚠ Firehose mode active — research only, NOT for live trading">
          MAX_OPEN_POSITIONS raised to 60. Per-side cap loosened to 30. Per-template cap loosened to 10.
          Daily strat cap disabled. Expect 10–20+ concurrent trades and heavy fee bleed.
          Disable by removing NEXT_PUBLIC_BTC_FT_FIREHOSE from env.
        </DeskBanner>
      )}

      {!EFFECTIVE_RESEARCH_MODE && !WINNERS_ONLY_MODE && rosterInfo.isLargeRoster && (
        <DeskBanner variant="info" title="Paper desk entry mode">
          {rosterInfo.ids.length} strategies · threshold {threshold} · relax-confirm ON · bootstrap probe after 5m
          with zero trades. Check Entry debug below for dominantBlocker. Build{" "}
          {BTC_FT_DESK_BUILD} is the Vercel client — AWS engine is separate.
        </DeskBanner>
      )}

      {shouldRenderEngine && (
        <BTCFuturesScalper
          title={EFFECTIVE_RESEARCH_MODE ? "BTC Future Trading - Research" : "BTC Future Trading"}
          moduleTagline={tagline}
          strategyIds={rosterInfo.ids}
          symbols={BTC_ONLY_SYMBOLS}
          signalThreshold={threshold}
          relaxEntryConfirmation={relaxConfirm}
          cooldownMultiplier={EFFECTIVE_RESEARCH_MODE ? researchCooldownMul() : 1}
          minMoveKMultiplier={minMoveKMul}
          slippageBpsOverride={EFFECTIVE_RESEARCH_MODE ? researchSlippageBps() : undefined}
          disableAutoKill={EFFECTIVE_RESEARCH_MODE ? researchDisableAutoKill() : false}
          researchEnsureTrades={EFFECTIVE_RESEARCH_MODE ? researchEnsureTradesEnabled() : false}
          paperEnsureTrades={paperEnsureTrades}
          paperBootstrapProbe={paperEnsureTrades || EFFECTIVE_RESEARCH_MODE}
          entryDebugEnabled={entryDebugEnabled}
          entryUtcSessionOverride={EFFECTIVE_RESEARCH_MODE ? researchEntryUtcSession() : undefined}
          researchMode={EFFECTIVE_RESEARCH_MODE}
          researchPoolIds={EFFECTIVE_RESEARCH_MODE ? resolveResearchPool() : undefined}
          researchWinners={winners}
          researchRetiredIds={retiredIds}
          onPromoteResearchWinner={handlePromote}
          onRetireResearchStrategy={handleRetire}
          onUnretireResearchStrategy={handleUnretire}
          onAutoRetireResearchLosers={handleAutoRetireLosers}
          strategyProfile={strategyProfile}
          promotedStrategyIds={winners}
          watchlist={BTC_ONLY_WATCHLIST}
          storageNamespace={STORAGE_NS}
          baseBalance={1000}
          moduleKey="btc_future_trading"
        />
      )}
    </>
  );
}

// ── Sub-component: Research banners ──────────────────────────────────────────
function ResearchBanners({
  researchMode,
  rosterInfo,
  threshold,
  relaxConfirm,
  winnersCount,
  generatedCount,
}: {
  researchMode: boolean;
  rosterInfo: RosterInfo;
  threshold: number;
  relaxConfirm: boolean;
  winnersCount: number;
  generatedCount: number;
}) {
  if (!researchMode) return null;
  const batchLabel = `batch ${(rosterInfo.batchIndex ?? 0) + 1}/${rosterInfo.totalBatches ?? 1}`;
  const poolSize = rosterInfo.poolSize ?? rosterInfo.ids.length;
  return (
    <>
      <DeskBanner variant="info" title={`Research / Tournament mode - ${batchLabel}`}>
        Running {rosterInfo.ids.length} active strategies of {poolSize} pool
        {generatedCount > 0 ? ` (${generatedCount} generated + core)` : ""}.
        Threshold {threshold}, relax-confirm {relaxConfirm ? "ON" : "OFF"}, cooldown 0.5x.
        Auto-kill disabled. Winners: {winnersCount} promoted.{" "}
        <a href="#btc-ft-research-mode" style={{ textDecoration: "underline", fontWeight: 600 }}>
          How it works
        </a>
      </DeskBanner>
      {(rosterInfo.poolSize ?? 0) > 80 && (
        <DeskBanner variant="warning" title={`Large research pool — ${poolSize} strategies`}>
          Single symbol BTC — pool is for research rotation, not simultaneous edge.
          Only ~{rosterInfo.ids.length} strategies trade per 24h batch; the rest cycle in over time.
        </DeskBanner>
      )}
    </>
  );
}
