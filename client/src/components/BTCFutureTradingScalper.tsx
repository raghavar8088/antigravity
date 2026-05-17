"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { BTCFuturesScalper } from "@/components/BTCFuturesScalper";
import type { BTCFuturesEngineOptions } from "@/hooks/useBTCFuturesScalperEngine";
import { btcFtSignalThresholdFromEnv, btcFtRelaxConfirmEnabledFromEnv } from "@/lib/futuresDeskPolicy";
import { FUTURES_WATCHLIST } from "@/lib/futuresMarketData";
import { resolveBtcFtActiveStrategyIds } from "@/lib/btcFtRoster";
import { DeskBanner } from "@/components/desk/ui";
import {
  isResearchModeEnabled,
  resolveResearchPool,
  resolveResearchActiveIds,
  researchSignalThreshold,
  researchRelaxConfirm,
  researchCooldownMul,
  researchMinMoveKMul,
  researchDisableAutoKill,
  researchEnsureTradesEnabled,
  researchSlippageBps,
  researchEntryUtcSession,
  resolveCoreWinners,
  loadRetiredFromStorage,
  saveRetiredToStorage,
  loadWinnersFromStorage,
  saveWinnersToStorage,
} from "@/lib/btcFtResearch";

const BTC_ONLY_SYMBOLS = ["BTCUSD"] as const;
const BTC_ONLY_WATCHLIST = FUTURES_WATCHLIST.filter((item) => item.symbol === "BTCUSD");

// Resolve once at module load (server / SSR) — same env for entire session.
const RESEARCH_MODE = isResearchModeEnabled();
const ACTIVE_ROSTER_STATIC = RESEARCH_MODE ? null : resolveBtcFtActiveStrategyIds();

const STORAGE_NS = "btc_future_trading_20";

export function BTCFutureTradingScalper({
  strategyProfile,
}: {
  /**
   * Optional A/B profile: `"scalp_aggro_v1"` | `"fee_aware_v1"` (default baseline).
   * Pair with a distinct `storageNamespace` in a forked route for clean localStorage comparison.
   */
  strategyProfile?: BTCFuturesEngineOptions["strategyProfile"];
} = {}) {
  // ── Research mode state ────────────────────────────────────────────────────
  const [retiredIds, setRetiredIds] = useState<Set<number>>(() =>
    RESEARCH_MODE ? loadRetiredFromStorage(STORAGE_NS) : new Set(),
  );
  const [winners, setWinners] = useState<number[]>(() =>
    RESEARCH_MODE ? loadWinnersFromStorage(STORAGE_NS) : resolveCoreWinners(STORAGE_NS),
  );
  const poolRef = useRef<number[]>([]);

  // Resolve pool + active batch (research) or static core roster (production)
  const rosterInfo = useMemo(() => {
    if (!RESEARCH_MODE) {
      const winnerIds = resolveCoreWinners(STORAGE_NS);
      if (winnerIds.length > 0) {
        return {
          ids: winnerIds,
          source: "winners" as const,
          isLargeRoster: false,
          batchIndex: null,
          totalBatches: null,
          poolSize: null,
        };
      }
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
    poolRef.current = pool;
    const batch = resolveResearchActiveIds({ pool, retiredIds, batchSize: 30, rotateEveryHours: 24 });
    return {
      ids: batch.activeIds,
      source: "research" as const,
      isLargeRoster: false,
      batchIndex: batch.batchIndex,
      totalBatches: batch.totalBatches,
      poolSize: batch.poolSize,
    };
  }, [retiredIds]);

  // Persist retired changes
  useEffect(() => {
    if (RESEARCH_MODE) saveRetiredToStorage(STORAGE_NS, retiredIds);
  }, [retiredIds]);

  // Persist winner changes
  useEffect(() => {
    if (RESEARCH_MODE) saveWinnersToStorage(STORAGE_NS, winners);
  }, [winners]);

  // ── Threshold ──────────────────────────────────────────────────────────────
  const threshold = RESEARCH_MODE ? researchSignalThreshold() : btcFtSignalThresholdFromEnv(26);
  const relaxConfirm = RESEARCH_MODE ? researchRelaxConfirm() : btcFtRelaxConfirmEnabledFromEnv();

  // ── UI labels ──────────────────────────────────────────────────────────────
  const sourceLabel =
    rosterInfo.source === "research"
      ? `research batch ${(rosterInfo.batchIndex ?? 0) + 1}/${rosterInfo.totalBatches ?? 1}`
      : rosterInfo.source === "winners"
      ? "winners"
      : rosterInfo.source === "env"
      ? "env"
      : rosterInfo.source === "core+ranked"
      ? "core + ranked"
      : "core";

  const tagline = RESEARCH_MODE
    ? `BTC PERPETUAL FUTURES · 25x · ${rosterInfo.ids.length} ACTIVE OF ${rosterInfo.poolSize ?? rosterInfo.ids.length} POOL · THRESHOLD ${threshold} · RESEARCH`
    : `BTC PERPETUAL FUTURES · 25x · ${rosterInfo.ids.length} STRATEGIES (${sourceLabel.toUpperCase()}) · THRESHOLD ${threshold}`;

  return (
    <>
      {/* ── Research mode banner ── */}
      {RESEARCH_MODE && (
        <DeskBanner variant="info" title={`Research / Tournament mode — batch ${(rosterInfo.batchIndex ?? 0) + 1}/${rosterInfo.totalBatches ?? 1}`}>
          Running {rosterInfo.ids.length} active strategies of {rosterInfo.poolSize ?? rosterInfo.ids.length} pool.
          Threshold {threshold}, relax-confirm {relaxConfirm ? "ON" : "OFF"}, cooldown 0.5×.
          Auto-kill disabled. Winners: {winners.length} promoted.{" "}
          <a href="#btc-ft-research-mode" style={{ textDecoration: "underline", fontWeight: 600 }}>
            How it works ↗
          </a>
        </DeskBanner>
      )}

      {/* ── Large roster warning (production mode with >30 ids via env) ── */}
      {!RESEARCH_MODE && rosterInfo.isLargeRoster && (
        <DeskBanner variant="warning" title="Large roster on single symbol">
          You have {rosterInfo.ids.length} strategies active via NEXT_PUBLIC_BTC_FT_STRATEGY_IDS.
          On choppy 1m bars with threshold {threshold}, candidatesBuilt ≈ 0 is expected.
          See{" "}
          <a href="#btc-ft-no-trades" style={{ textDecoration: "underline", fontWeight: 600 }}>
            README#btc-ft-no-trades
          </a>{" "}
          for troubleshooting.
        </DeskBanner>
      )}

      <BTCFuturesScalper
        title={RESEARCH_MODE ? "BTC Future Trading — Research" : "BTC Future Trading"}
        moduleTagline={tagline}
        strategyIds={rosterInfo.ids}
        symbols={BTC_ONLY_SYMBOLS}
        signalThreshold={threshold}
        relaxEntryConfirmation={relaxConfirm}
        cooldownMultiplier={RESEARCH_MODE ? researchCooldownMul() : 1}
        minMoveKMultiplier={RESEARCH_MODE ? researchMinMoveKMul() : 1}
        slippageBpsOverride={RESEARCH_MODE ? researchSlippageBps() : undefined}
        disableAutoKill={RESEARCH_MODE ? researchDisableAutoKill() : false}
        researchEnsureTrades={RESEARCH_MODE ? researchEnsureTradesEnabled() : false}
        entryUtcSessionOverride={RESEARCH_MODE ? researchEntryUtcSession() : undefined}
        researchMode={RESEARCH_MODE}
        researchPoolIds={RESEARCH_MODE ? poolRef.current : undefined}
        researchWinners={winners}
        researchRetiredIds={retiredIds}
        onPromoteResearchWinner={(id) => {
          setWinners((prev) => [...new Set([...prev, id])].slice(0, 15));
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
      />
    </>
  );
}
