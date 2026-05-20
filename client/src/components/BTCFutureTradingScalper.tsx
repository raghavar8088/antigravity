"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { BTCFuturesScalper } from "@/components/BTCFuturesScalper";
import type { BTCFuturesEngineOptions } from "@/hooks/useBTCFuturesScalperEngine";
import { usePaperDeskAuth } from "@/hooks/usePaperDeskAuth";
import {
  btcFtEntryDebugEnabledFromEnv,
  btcFtMinMoveKMulFromEnv,
  btcFtPaperEnsureTradesFromEnv,
  btcFtRelaxConfirmEnabledFromEnv,
  btcFtSignalThresholdFromEnv,
} from "@/lib/futuresDeskPolicy";
import { FUTURES_WATCHLIST } from "@/lib/futuresMarketData";
import { DeskBanner } from "@/components/desk/ui";
import {
  isResearchModeEnabled,
  isWinnersOnlyModeEnabled,
  loadPromotedWinnerIds,
  loadRetiredFromStorage,
  loadWinnersFromStorage,
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

const BTC_ONLY_SYMBOLS = ["BTCUSD"] as const;
const BTC_ONLY_WATCHLIST = FUTURES_WATCHLIST.filter((item) => item.symbol === "BTCUSD");
const STORAGE_NS = "btc_future_trading_v4";

// Resolve once at module load; env flags are stable for the browser bundle.
const RESEARCH_MODE = isResearchModeEnabled();
const WINNERS_ONLY_MODE = isWinnersOnlyModeEnabled();
const EFFECTIVE_RESEARCH_MODE = RESEARCH_MODE && !WINNERS_ONLY_MODE;
const ACTIVE_ROSTER_STATIC = EFFECTIVE_RESEARCH_MODE
  ? null
  : resolveBtcFtActiveStrategyIds({ storageNamespace: STORAGE_NS });

export function BTCFutureTradingScalper({
  strategyProfile,
}: {
  /**
   * Optional A/B profile: `"scalp_aggro_v1"` | `"fee_aware_v1"` (default baseline).
   * Pair with a distinct `storageNamespace` in a forked route for clean localStorage comparison.
   */
  strategyProfile?: BTCFuturesEngineOptions["strategyProfile"];
} = {}) {
  const auth = usePaperDeskAuth();
  const [retiredIds, setRetiredIds] = useState<Set<number>>(() =>
    EFFECTIVE_RESEARCH_MODE ? loadRetiredFromStorage(STORAGE_NS) : new Set(),
  );
  const [winners, setWinners] = useState<number[]>(() =>
    RESEARCH_MODE || WINNERS_ONLY_MODE ? loadWinnersFromStorage(STORAGE_NS) : resolveCoreWinners(STORAGE_NS),
  );
  const poolRef = useRef<number[]>([]);

  useEffect(() => {
    if (!WINNERS_ONLY_MODE) return;
    let cancelled = false;
    void loadPromotedWinnerIds({
      storageNamespace: STORAGE_NS,
      cloudAccountKey: auth.user?.id ?? null,
    }).then((ids) => {
      if (!cancelled) setWinners(ids);
    });
    return () => { cancelled = true; };
  }, [auth.user?.id]);

  const rosterInfo = useMemo(() => {
    if (WINNERS_ONLY_MODE) {
      const r = resolveBtcFtActiveStrategyIds({ storageNamespace: STORAGE_NS, winnerIds: winners });
      return {
        ids: r.ids,
        source: r.source,
        isLargeRoster: r.isLargeRoster,
        batchIndex: null,
        totalBatches: null,
        poolSize: null,
      };
    }

    if (!EFFECTIVE_RESEARCH_MODE) {
      const r = ACTIVE_ROSTER_STATIC!;
      return {
        ids: r.ids,
        source: r.source,
        isLargeRoster: r.isLargeRoster,
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
  }, [retiredIds, winners]);

  useEffect(() => {
    if (EFFECTIVE_RESEARCH_MODE) poolRef.current = resolveResearchPool();
  }, [retiredIds]);

  useEffect(() => {
    if (EFFECTIVE_RESEARCH_MODE) saveRetiredToStorage(STORAGE_NS, retiredIds);
  }, [retiredIds]);

  useEffect(() => {
    if (RESEARCH_MODE || WINNERS_ONLY_MODE) saveWinnersToStorage(STORAGE_NS, winners);
  }, [winners]);

  const threshold = WINNERS_ONLY_MODE
    ? 26
    : EFFECTIVE_RESEARCH_MODE
    ? researchSignalThreshold()
    : btcFtSignalThresholdFromEnv(18);
  const relaxConfirm = WINNERS_ONLY_MODE
    ? false
    : EFFECTIVE_RESEARCH_MODE
    ? researchRelaxConfirm()
    : true;
  const minMoveKMul = WINNERS_ONLY_MODE
    ? 1
    : EFFECTIVE_RESEARCH_MODE
    ? researchMinMoveKMul()
    : btcFtMinMoveKMulFromEnv(0.45);
  const paperEnsureTrades =
    !WINNERS_ONLY_MODE && !EFFECTIVE_RESEARCH_MODE && btcFtPaperEnsureTradesFromEnv();
  const entryDebugEnabled = btcFtEntryDebugEnabledFromEnv() || rosterInfo.isLargeRoster;

  const sourceLabel =
    rosterInfo.source === "research"
      ? `research batch ${(rosterInfo.batchIndex ?? 0) + 1}/${rosterInfo.totalBatches ?? 1}`
      : rosterInfo.source === "winners-empty"
      ? "no winners"
      : rosterInfo.source === "winners"
      ? "winners"
      : rosterInfo.source === "env"
      ? "env"
      : rosterInfo.source === "core+ranked"
      ? "core + ranked"
      : rosterInfo.source === "full"
      ? "full roster"
      : "core";

  const tagline = WINNERS_ONLY_MODE
    ? `BTC PERPETUAL FUTURES - 25x - ${rosterInfo.ids.length} WINNER STRATEGIES - THRESHOLD ${threshold} - PRODUCTION`
    : EFFECTIVE_RESEARCH_MODE
    ? `BTC PERPETUAL FUTURES - 25x - ${rosterInfo.ids.length} ACTIVE OF ${rosterInfo.poolSize ?? rosterInfo.ids.length} POOL - THRESHOLD ${threshold} - RESEARCH`
    : `BTC PERPETUAL FUTURES - 25x - ${rosterInfo.ids.length} STRATEGIES (${sourceLabel.toUpperCase()}) - THRESHOLD ${threshold} - BUILD ${BTC_FT_DESK_BUILD}`;

  const shouldRenderEngine = !WINNERS_ONLY_MODE || rosterInfo.ids.length > 0;

  return (
    <>
      {EFFECTIVE_RESEARCH_MODE && (
        <DeskBanner
          variant="info"
          title={`Research / Tournament mode - batch ${(rosterInfo.batchIndex ?? 0) + 1}/${rosterInfo.totalBatches ?? 1}`}
        >
          Running {rosterInfo.ids.length} active strategies of {rosterInfo.poolSize ?? rosterInfo.ids.length} pool
          {poolGeneratedCount() > 0 ? ` (${poolGeneratedCount()} generated + core)` : ""}.
          Threshold {threshold}, relax-confirm {relaxConfirm ? "ON" : "OFF"}, cooldown 0.5x.
          Auto-kill disabled. Winners: {winners.length} promoted.{" "}
          <a href="#btc-ft-research-mode" style={{ textDecoration: "underline", fontWeight: 600 }}>
            How it works
          </a>
        </DeskBanner>
      )}

      {EFFECTIVE_RESEARCH_MODE && (rosterInfo.poolSize ?? 0) > 80 && (
        <DeskBanner variant="warning" title={`Large research pool — ${rosterInfo.poolSize} strategies`}>
          Single symbol BTC — pool is for research rotation, not simultaneous edge.
          Only ~{rosterInfo.ids.length} strategies trade per 24h batch; the rest cycle in over time.
          Expect long runway before per-strategy verdicts converge.
        </DeskBanner>
      )}

      {process.env.NEXT_PUBLIC_BTC_FT_FIREHOSE === "1" && (
        <DeskBanner variant="warning" title="⚠ Firehose mode active — research only, NOT for live trading">
          MAX_OPEN_POSITIONS raised to 60. Per-side cap loosened to 30. Per-template
          cap loosened to 10. Daily strat cap disabled. Expect 10–20+ concurrent
          trades and heavy fee bleed — this mode is for fast verdict discovery,
          not profitability. Disable for production by removing
          NEXT_PUBLIC_BTC_FT_FIREHOSE from env.
        </DeskBanner>
      )}

      {WINNERS_ONLY_MODE && rosterInfo.ids.length > 0 && (
        <DeskBanner variant="info" title={`Production winners mode - ${rosterInfo.ids.length} strategies`}>
          Running only promoted BTC FT winners with production gates: threshold 26, relax-confirm OFF,
          cooldown 1x, min-move K 1x, and auto-kill ON. No Delta mainnet orders are enabled by this mode.
        </DeskBanner>
      )}

      {WINNERS_ONLY_MODE && rosterInfo.ids.length === 0 && (
        <DeskBanner variant="warning" title="No winners - run research or set BTC_FT_STRATEGY_IDS">
          Winners-only mode is enabled, but no promoted winner IDs were found in localStorage or Supabase.
          Run research and promote winners, or set NEXT_PUBLIC_BTC_FT_STRATEGY_IDS to a manual winners roster.
        </DeskBanner>
      )}

      {!EFFECTIVE_RESEARCH_MODE && !WINNERS_ONLY_MODE && rosterInfo.isLargeRoster && (
        <DeskBanner variant="info" title="Paper desk entry mode">
          {rosterInfo.ids.length} strategies · threshold {threshold} · relax-confirm ON · bootstrap probe after 5m
          with zero trades. Check Entry debug below for dominantBlocker (DATA / CONFIRM / MIN_MOVE). Build{" "}
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
          paperBootstrapProbe={paperEnsureTrades}
          entryDebugEnabled={entryDebugEnabled}
          entryUtcSessionOverride={EFFECTIVE_RESEARCH_MODE ? researchEntryUtcSession() : undefined}
          researchMode={EFFECTIVE_RESEARCH_MODE}
          researchPoolIds={EFFECTIVE_RESEARCH_MODE ? poolRef.current : undefined}
          researchWinners={winners}
          researchRetiredIds={retiredIds}
          onPromoteResearchWinner={(id) => {
            setWinners((prev) => [...new Set([...prev, id])].slice(0, 20));
          }}
          onRetireResearchStrategy={(id) => {
            setRetiredIds((prev) => new Set([...prev, id]));
          }}
          onUnretireResearchStrategy={(id) => {
            setRetiredIds((prev) => {
              const next = new Set(prev);
              next.delete(id);
              return next;
            });
          }}
          onAutoRetireResearchLosers={(ids) => {
            setRetiredIds((prev) => new Set([...prev, ...ids]));
          }}
          strategyProfile={strategyProfile}
          watchlist={BTC_ONLY_WATCHLIST}
          storageNamespace={STORAGE_NS}
          baseBalance={1000}
          moduleKey="btc_future_trading"
        />
      )}
    </>
  );
}
