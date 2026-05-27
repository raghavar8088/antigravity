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
  btcFtPaperCooldownMultiplierFromEnv,
  btcFtPaperEnsureTradesFromEnv,
  btcFtSignalThresholdFromEnv,
} from "@/lib/futuresDeskPolicy";
import { FUTURES_WATCHLIST } from "@/lib/futuresMarketData";
import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import { DeskBanner } from "@/components/desk/ui";
import { LiveDeskStatusBar } from "@/components/LiveDeskStatusBar";
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
import {
  BTC_FT_RESEARCH_CATEGORY_IDS,
  CATEGORY_STRATEGY_IDS,
  btcFtActiveCategoryFromEnv,
} from "@/lib/btcFtRoster";
import { CATEGORY_REGISTRY } from "@/lib/futuresCategoryRegistry";
import type { TradingCategoryId } from "@/lib/futuresStratTypes";
import { BTC_FT_DESK_BUILD } from "@/lib/btcFtDeskBuild";
import { profitModeFromEnv } from "@/lib/futuresProfitMode";

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
  const profitMode = useMemo(() => profitModeFromEnv(), []);
  const strategyProfile = useMemo(() => {
    if (strategyProfileProp === "scalp_aggro_v1") return "baseline" as const;
    if (strategyProfileProp) return strategyProfileProp;
    if (profitMode.enabled) return "fee_aware_v1" as const;
    return undefined;
  }, [strategyProfileProp, profitMode.enabled]);
  const auth = usePaperDeskAuth();
  const { RESEARCH_MODE, WINNERS_ONLY_MODE, EFFECTIVE_RESEARCH_MODE } = useResolvedModes();

  // PR 10 — runtime category filter. Initialized from NEXT_PUBLIC_BTC_FT_CATEGORY
  // env (one-shot read; users can override via the in-page dropdown afterwards).
  // "all" = no filter (CORE+premium + full research pool).
  const [selectedCategory, setSelectedCategory] = useState<TradingCategoryId | "all">(
    () => btcFtActiveCategoryFromEnv(),
  );

  /** Per-category storageNamespace so a swing trade's PnL doesn't pollute the
   *  scalping ledger and vice versa. "all" keeps the legacy namespace.        */
  const effectiveStorageNs = useMemo(
    () => (selectedCategory === "all" ? STORAGE_NS : `${STORAGE_NS}_${selectedCategory}`),
    [selectedCategory],
  );

  // Resolve the same account key the engine will use so research state is co-located
  const accountKey = useMemo(
    () =>
      resolveCloudPaperTradesAccountKey({
        supabaseUserId: auth.user?.id,
        storageNamespace: effectiveStorageNs,
      }) ?? getOrCreateAnonAccountKey() ?? "",
    [auth.user?.id, effectiveStorageNs],
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
      const r = resolveBtcFtActiveStrategyIds({ storageNamespace: effectiveStorageNs, winnerIds: winners });
      return { ids: r.ids, source: r.source, isLargeRoster: r.isLargeRoster, batchIndex: null, totalBatches: null, poolSize: null };
    }
    if (!EFFECTIVE_RESEARCH_MODE) {
      // "all" uses every defined strategy so the leaderboard shows AVAILABLE for each.
      const merged =
        selectedCategory === "all"
          ? FUTURES_STRAT_DEFS.map((d) => d.id)
          : [...CATEGORY_STRATEGY_IDS[selectedCategory]];
      return {
        ids: merged,
        source: selectedCategory === "all" ? ("full" as const) : selectedCategory,
        isLargeRoster: merged.length > 30,
        batchIndex: null,
        totalBatches: null,
        poolSize: null,
      };
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
  }, [retiredIds, winners, WINNERS_ONLY_MODE, EFFECTIVE_RESEARCH_MODE, selectedCategory, effectiveStorageNs]);

  // ── Derived engine options ─────────────────────────────────────────────────
  const threshold = useMemo(() => {
    if (profitMode.enabled && !EFFECTIVE_RESEARCH_MODE) {
      return btcFtSignalThresholdFromEnv(28);
    }
    if (WINNERS_ONLY_MODE) return btcFtSignalThresholdFromEnv(26);
    if (EFFECTIVE_RESEARCH_MODE) return researchSignalThreshold();
    return btcFtSignalThresholdFromEnv(20);
  }, [WINNERS_ONLY_MODE, EFFECTIVE_RESEARCH_MODE, profitMode.enabled]);

  const relaxConfirm = useMemo(() => {
    if (profitMode.enabled && !EFFECTIVE_RESEARCH_MODE) return false;
    if (EFFECTIVE_RESEARCH_MODE) return researchRelaxConfirm();
    return true;
  }, [EFFECTIVE_RESEARCH_MODE, profitMode.enabled]);

  const minMoveKMul = useMemo(() => {
    if (profitMode.enabled && !EFFECTIVE_RESEARCH_MODE) return 1;
    if (WINNERS_ONLY_MODE) return btcFtMinMoveKMulFromEnv(1.0);
    if (EFFECTIVE_RESEARCH_MODE) return researchMinMoveKMul();
    return btcFtMinMoveKMulFromEnv(0.45);
  }, [WINNERS_ONLY_MODE, EFFECTIVE_RESEARCH_MODE, profitMode.enabled]);

  /** Paper discovery: always on for BTC FT desk unless explicitly disabled via env or WINNERS_ONLY (which must only trade on merit). */
  const paperEnsureTrades =
    !EFFECTIVE_RESEARCH_MODE && !WINNERS_ONLY_MODE && btcFtPaperEnsureTradesFromEnv();
  const paperDeskActive = paperEnsureTrades;
  const paperBootstrapProbe =
    (!EFFECTIVE_RESEARCH_MODE && !WINNERS_ONLY_MODE && btcFtPaperEnsureTradesFromEnv()) ||
    (EFFECTIVE_RESEARCH_MODE && researchEnsureTradesEnabled());

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

  const statusChips = useMemo(
    () => [
      WINNERS_ONLY_MODE ? "Winners" : EFFECTIVE_RESEARCH_MODE ? "Research" : "Paper",
      "25×",
      `${rosterInfo.ids.length} active`,
      `TH ${threshold}`,
      BTC_FT_DESK_BUILD !== "dev" ? `BUILD ${BTC_FT_DESK_BUILD}` : "",
    ].filter(Boolean),
    [WINNERS_ONLY_MODE, EFFECTIVE_RESEARCH_MODE, rosterInfo.ids.length, threshold],
  );

  return (
    <>
      <CategoryFilterBar
        selected={selectedCategory}
        onChange={setSelectedCategory}
        activeRosterCount={rosterInfo.ids.length}
      />

      <ResearchBanners
        researchMode={EFFECTIVE_RESEARCH_MODE}
        rosterInfo={rosterInfo}
        threshold={threshold}
        relaxConfirm={relaxConfirm}
        winnersCount={winners.length}
        generatedCount={poolGeneratedCount()}
      />

      <LiveDeskStatusBar
        profitModeEnabled={profitMode.enabled}
        isSignedIn={!!auth.user}
      />

      {/* Priority 1: Firehose warning (always shown when active) */}
      {process.env.NEXT_PUBLIC_BTC_FT_FIREHOSE === "1" && (
        <DeskBanner variant="warning" title="⚠ Firehose mode active — research only, NOT for live trading">
          MAX_OPEN_POSITIONS raised to 60. Per-side cap loosened to 30. Per-template cap loosened to 10.
          Daily strat cap disabled. Expect 10–20+ concurrent trades and heavy fee bleed.
          Disable by removing NEXT_PUBLIC_BTC_FT_FIREHOSE from env.
        </DeskBanner>
      )}

      {/* Priority 2: Profit mode + Winners merged into one banner when both apply */}
      {profitMode.enabled && WINNERS_ONLY_MODE && !EFFECTIVE_RESEARCH_MODE && rosterInfo.ids.length > 0 && (
        <DeskBanner variant="success" title={`Profit mode ON · Winners desk — ${rosterInfo.ids.length} strategies · threshold ${threshold}`}>
          CORE winner roster · quality ≥{profitMode.minQualityScore} · MTF ≥{profitMode.minMtfConfluence} ·
          max {profitMode.maxOpenPositions} open · fee_aware profile · relaxed confirm OFF. Paper only.
        </DeskBanner>
      )}
      {profitMode.enabled && !WINNERS_ONLY_MODE && !EFFECTIVE_RESEARCH_MODE && (
        <DeskBanner variant="success" title="Profit mode ON — trade less, trade better">
          Quality ≥{profitMode.minQualityScore} · MTF ≥{profitMode.minMtfConfluence} · chop same-side max{" "}
          {profitMode.maxSameSideChop} · max {profitMode.maxOpenPositions} open · {profitMode.dailyStratCap}
          /day/strat · fee_aware profile · threshold {threshold} · relaxed confirm OFF. Paper only.
        </DeskBanner>
      )}
      {!profitMode.enabled && WINNERS_ONLY_MODE && rosterInfo.ids.length > 0 && (
        <DeskBanner variant="info" title={`Winners paper desk - ${rosterInfo.ids.length} strategies`}>
          CORE winner roster · threshold {threshold} · scalp TP ~0.45% · wide SL · 8m SL grace ·
          profit-bias exits ON. Keep tab open. Paper only — not guaranteed wins on live BTC.
        </DeskBanner>
      )}

      {/* Large-roster info banner — hidden when profit mode is already explaining the setup */}
      {!profitMode.enabled && !EFFECTIVE_RESEARCH_MODE && !WINNERS_ONLY_MODE && rosterInfo.isLargeRoster && (
        <DeskBanner variant="info" title="Paper desk entry mode">
          {rosterInfo.ids.length} strategies · threshold {threshold} · relax-confirm ON · bootstrap probe after 90s
          with zero trades. Keep this browser tab open — polling runs every ~4s only while the page is active.
          Check Entry debug for dominantBlocker. Build {BTC_FT_DESK_BUILD}.
        </DeskBanner>
      )}

      {WINNERS_ONLY_MODE && rosterInfo.ids.length === 0 && (
        <DeskBanner variant="warning" title="No winner strategies loaded">
          Winners desk is active but the roster is empty. Set{" "}
          <code>NEXT_PUBLIC_BTC_FT_STRATEGY_IDS</code> to a comma-separated list of strategy IDs,
          or promote winners via the leaderboard in research mode. Engine will not open positions
          until at least one strategy ID is available.
        </DeskBanner>
      )}

      {shouldRenderEngine && (
        <BTCFuturesScalper
          title={EFFECTIVE_RESEARCH_MODE ? "BTC Future Trading - Research" : "BTC Future Trading"}
          moduleTagline={tagline}
          statusChips={statusChips}
          strategyIds={rosterInfo.ids}
          symbols={BTC_ONLY_SYMBOLS}
          signalThreshold={threshold}
          relaxEntryConfirmation={relaxConfirm}
          cooldownMultiplier={
            EFFECTIVE_RESEARCH_MODE
              ? researchCooldownMul()
              : paperDeskActive
                ? btcFtPaperCooldownMultiplierFromEnv()
                : 1
          }
          minMoveKMultiplier={minMoveKMul}
          slippageBpsOverride={EFFECTIVE_RESEARCH_MODE ? researchSlippageBps() : undefined}
          disableAutoKill={EFFECTIVE_RESEARCH_MODE ? researchDisableAutoKill() : false}
          researchEnsureTrades={EFFECTIVE_RESEARCH_MODE ? researchEnsureTradesEnabled() : false}
          paperEnsureTrades={paperEnsureTrades}
          paperBootstrapProbe={paperBootstrapProbe}
          entryDebugEnabled={entryDebugEnabled}
          entryUtcSessionOverride={EFFECTIVE_RESEARCH_MODE ? researchEntryUtcSession() : undefined}
          researchMode={EFFECTIVE_RESEARCH_MODE}
          researchPoolIds={
            EFFECTIVE_RESEARCH_MODE
              ? resolveResearchPool()
              : [...BTC_FT_RESEARCH_CATEGORY_IDS]
          }
          researchWinners={winners}
          researchRetiredIds={retiredIds}
          onPromoteResearchWinner={handlePromote}
          onRetireResearchStrategy={handleRetire}
          onUnretireResearchStrategy={handleUnretire}
          onAutoRetireResearchLosers={handleAutoRetireLosers}
          strategyProfile={strategyProfile}
          promotedStrategyIds={winners}
          watchlist={BTC_ONLY_WATCHLIST}
          storageNamespace={effectiveStorageNs}
          baseBalance={1000}
          moduleKey="btc_future_trading"
          allStrategiesAvailable
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

// ── Sub-component: Category filter dropdown ─────────────────────────────────
// PR 10 — runtime category filter. Switching narrows the active roster to one
// category's strategies and uses a category-specific storageNamespace so PnL
// stays isolated from the other categories.
const CATEGORY_OPTIONS: Array<{ value: TradingCategoryId | "all"; label: string }> = [
  { value: "all", label: "All categories" },
  { value: "scalping", label: CATEGORY_REGISTRY.scalping.label },
  { value: "day_trading", label: CATEGORY_REGISTRY.day_trading.label },
  { value: "swing_trading", label: CATEGORY_REGISTRY.swing_trading.label },
  { value: "position_trading", label: CATEGORY_REGISTRY.position_trading.label },
  { value: "trend_trading", label: CATEGORY_REGISTRY.trend_trading.label },
  { value: "range_trading", label: CATEGORY_REGISTRY.range_trading.label },
  { value: "breakout_trading", label: CATEGORY_REGISTRY.breakout_trading.label },
  { value: "momentum_trading", label: CATEGORY_REGISTRY.momentum_trading.label },
];

function CategoryFilterBar({
  selected,
  onChange,
  activeRosterCount,
}: {
  selected: TradingCategoryId | "all";
  onChange: (next: TradingCategoryId | "all") => void;
  activeRosterCount: number;
}) {
  const profile = selected === "all" ? null : CATEGORY_REGISTRY[selected];
  const detail = profile
    ? `${profile.primaryBarInterval} bars · ${profile.defaultLeverage}× lev · hold ${profile.holdMinutesMin}–${profile.holdMinutesMax}m · SL ${profile.slPctMin}-${profile.slPctMax}%`
    : "CORE 20 + premium + full 60-strategy research pool";
  return (
    <DeskBanner variant="info" title={`Category filter — ${activeRosterCount} strategies`}>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, alignItems: "center", marginTop: 4 }}>
        <label htmlFor="btc-ft-category-select" className="desk-label-md">
          Category:
        </label>
        <select
          id="btc-ft-category-select"
          value={selected}
          onChange={(e) => onChange(e.target.value as TradingCategoryId | "all")}
          style={{
            padding: "6px 10px",
            borderRadius: 6,
            border: "1px solid var(--desk-outline)",
            background: "var(--desk-surface)",
            color: "var(--desk-on-surface)",
            fontSize: "0.875rem",
            minWidth: 180,
          }}
        >
          {CATEGORY_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
        <span className="desk-label-md" style={{ fontWeight: 400 }}>{detail}</span>
      </div>
    </DeskBanner>
  );
}
