/**
 * BTC Future Trading — Research / Tournament mode (Part 1).
 *
 * Goal: test the full strategy pool (up to 120+ IDs), rotate batches of 30,
 * collect enough paper trades per strategy to compute real net PnL verdicts,
 * then promote WINNERS to a live-prep CORE roster.
 *
 * Research mode is ALWAYS env-gated (NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=1).
 * Production default: CORE roster, conservative gates — unaffected by this file.
 *
 * HARD RULES applied here:
 *   - Paper only; no Delta mainnet orders
 *   - Pool uses only IDs with real evalMinuteSignal wiring (no fake-div-only stubs)
 *   - All safety features remain (slippage 5bps, funding, liq cross-only, spread cap, category cap)
 *
 * See: README.md#btc-ft-research-mode
 */

import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import {
  BTC_FUTURE_TRADING_STRATEGY_IDS,
  CORE_BTC_FT_STRATEGY_IDS,
  resolveBtcFtActiveStrategyIds as resolveBaseBtcFtActiveStrategyIds,
  type BtcFtRosterSource,
} from "@/lib/btcFtRoster";
import type { DeskEntryUtcSession } from "@/lib/futuresDeskPolicy";

// ---------------------------------------------------------------------------
// Research mode gate
// ---------------------------------------------------------------------------

export function isResearchModeEnabled(): boolean {
  return process.env.NEXT_PUBLIC_BTC_FT_RESEARCH_MODE === "1";
}

export function isWinnersOnlyModeEnabled(): boolean {
  return process.env.NEXT_PUBLIC_BTC_FT_WINNERS_ONLY === "1";
}

// ---------------------------------------------------------------------------
// Strategy pool
// ---------------------------------------------------------------------------

/**
 * All valid candidate IDs for research tournament.
 * Uses NEXT_PUBLIC_BTC_FT_POOL_IDS (comma list) OR the verified BTC Future
 * Trading roster (CORE + BTC FT templates). This intentionally does not
 * default to every global futures ID, avoiding fake-diversity/unwired entries.
 */
export function resolveResearchPool(): number[] {
  const verifiedIds = new Set(BTC_FUTURE_TRADING_STRATEGY_IDS);
  const envPool = process.env.NEXT_PUBLIC_BTC_FT_POOL_IDS;
  if (envPool && envPool.trim() !== "") {
    const parsed = envPool
      .split(",")
      .map((s) => Number(s.trim()))
      .filter((n) => Number.isFinite(n) && n > 0);
    const validIds = new Set(FUTURES_STRAT_DEFS.map((s) => s.id));
    return [...new Set(parsed.filter((id) => validIds.has(id) && verifiedIds.has(id)))].slice(0, 200);
  }
  return [...BTC_FUTURE_TRADING_STRATEGY_IDS].slice(0, 200);
}

// ---------------------------------------------------------------------------
// Verdict types
// ---------------------------------------------------------------------------

export type ResearchVerdict =
  | "INSUFFICIENT_DATA" // < 10 trades
  | "CANDIDATE" // >= 10 trades, not yet WINNER or LOSER threshold
  | "WINNER" // expectancy > 0, trades >= 20, sumNet > 0
  | "LOSER"; // trades >= 15 AND (sumNet < -2 OR expectancy < -0.10)

export type ResearchStratStats = {
  strategyId: number;
  tradeCount: number;
  sumNet: number;
  expectancy: number;
  winRate: number;
  feePctOfGross: number | null;
  avgHoldMin: number | null;
  lastTradeAt: string | null;
  verdict: ResearchVerdict;
};

export function computeVerdict(stats: {
  tradeCount: number;
  sumNet: number;
  expectancy: number;
}): ResearchVerdict {
  const { tradeCount, sumNet, expectancy } = stats;
  if (tradeCount < 10) return "INSUFFICIENT_DATA";
  if (tradeCount >= 15 && (sumNet < -2 || expectancy < -0.1)) return "LOSER";
  if (tradeCount >= 20 && expectancy > 0 && sumNet > 0) return "WINNER";
  return "CANDIDATE";
}

// ---------------------------------------------------------------------------
// Batch rotation
// ---------------------------------------------------------------------------

export type ResearchBatch = {
  batchIndex: number; // 0-based
  totalBatches: number;
  activeIds: number[];
  poolSize: number;
};

/**
 * Return the current active batch of strategies from the pool.
 *
 * Rotation is time-based (rotateEveryHours) using a stable epoch slot.
 * Retired IDs (LOSERs from local state) are removed from the pool before slicing.
 *
 * @param pool - All candidate IDs (from resolveResearchPool)
 * @param retiredIds - IDs marked LOSER / retired (excluded from rotation)
 * @param batchSize - How many IDs to run concurrently (default 30)
 * @param rotateEveryHours - How often to advance batch (default 24)
 * @param nowMs - Timestamp override (for testing)
 */
export function resolveResearchActiveIds(opts: {
  pool: number[];
  retiredIds?: ReadonlySet<number>;
  batchSize?: number;
  rotateEveryHours?: number;
  nowMs?: number;
}): ResearchBatch {
  const { pool, retiredIds = new Set(), batchSize = 30, rotateEveryHours = 24 } = opts;
  const nowMs = opts.nowMs ?? Date.now();

  const eligiblePool = pool.filter((id) => !retiredIds.has(id));
  if (eligiblePool.length === 0) {
    return { batchIndex: 0, totalBatches: 1, activeIds: [...CORE_BTC_FT_STRATEGY_IDS], poolSize: 0 };
  }

  const safeSize = Math.max(1, Math.min(batchSize, eligiblePool.length));
  const totalBatches = Math.ceil(eligiblePool.length / safeSize);

  const slotMs = rotateEveryHours * 60 * 60 * 1000;
  const slotIndex = Math.floor(nowMs / slotMs);
  const batchIndex = slotIndex % totalBatches;

  const start = batchIndex * safeSize;
  const activeIds = eligiblePool.slice(start, start + safeSize);

  return { batchIndex, totalBatches, activeIds, poolSize: eligiblePool.length };
}

// ---------------------------------------------------------------------------
// Winners / Promotions (localStorage-backed)
// ---------------------------------------------------------------------------

export const RESEARCH_WINNERS_LS_KEY_SUFFIX = "_btc_ft_winners";
export const RESEARCH_RETIRED_LS_KEY_SUFFIX = "_btc_ft_retired";

export function loadWinnersFromStorage(ns: string): number[] {
  if (typeof localStorage === "undefined") return [];
  try {
    const raw = localStorage.getItem(`${ns}${RESEARCH_WINNERS_LS_KEY_SUFFIX}`);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return (parsed as unknown[]).filter((x): x is number => typeof x === "number" && Number.isFinite(x));
  } catch {
    return [];
  }
}

export function loadWinnersFromEnv(): number[] {
  const raw = process.env.NEXT_PUBLIC_BTC_FT_WINNER_IDS;
  if (!raw || raw.trim() === "") return [];
  const valid = new Set(BTC_FUTURE_TRADING_STRATEGY_IDS);
  return [
    ...new Set(
      raw
        .split(",")
        .map((s) => Number(s.trim()))
        .filter((id) => Number.isFinite(id) && valid.has(id)),
    ),
  ].slice(0, 15);
}

export function resolveCoreWinners(ns: string): number[] {
  const envWinners = loadWinnersFromEnv();
  if (envWinners.length > 0) return envWinners;
  return loadWinnersFromStorage(ns).filter((id) => BTC_FUTURE_TRADING_STRATEGY_IDS.includes(id)).slice(0, 15);
}

function parseExplicitBtcFtStrategyIds(cap: number): number[] {
  const raw = process.env.NEXT_PUBLIC_BTC_FT_STRATEGY_IDS;
  if (!raw || raw.trim() === "") return [];
  const valid = new Set(BTC_FUTURE_TRADING_STRATEGY_IDS);
  return [
    ...new Set(
      raw
        .split(",")
        .map((s) => Number(s.trim()))
        .filter((id) => Number.isFinite(id) && valid.has(id)),
    ),
  ].slice(0, cap);
}

export type BtcFtResearchRosterSource = BtcFtRosterSource | "winners" | "winners-empty";

export type BtcFtResearchRosterResolution = {
  ids: number[];
  source: BtcFtResearchRosterSource;
  isLargeRoster: boolean;
  winnersOnly: boolean;
};

/**
 * BTC Future Trading roster resolver with the production winners-only override.
 *
 * When NEXT_PUBLIC_BTC_FT_WINNERS_ONLY=1, research rotation and ranked/core
 * fallback are bypassed. The active roster is promoted winners only, capped at
 * 20. If no promoted winners exist, an explicit NEXT_PUBLIC_BTC_FT_STRATEGY_IDS
 * line may be used as the manual production roster; otherwise ids=[] so the UI
 * can stop the engine instead of accidentally running the full library.
 */
export function resolveBtcFtActiveStrategyIds(opts: {
  storageNamespace?: string;
  winnerIds?: readonly number[];
} = {}): BtcFtResearchRosterResolution {
  if (isWinnersOnlyModeEnabled()) {
    const ns = opts.storageNamespace ?? "btc_future_trading_20";
    const valid = new Set(BTC_FUTURE_TRADING_STRATEGY_IDS);
    const promoted = [
      ...new Set(
        (opts.winnerIds ?? resolveCoreWinners(ns))
          .map((id) => Math.floor(id))
          .filter((id) => valid.has(id)),
      ),
    ].slice(0, 20);
    if (promoted.length > 0) {
      return { ids: promoted, source: "winners", isLargeRoster: false, winnersOnly: true };
    }

    const explicit = parseExplicitBtcFtStrategyIds(20);
    if (explicit.length > 0) {
      return { ids: explicit, source: "env", isLargeRoster: false, winnersOnly: true };
    }

    return { ids: [], source: "winners-empty", isLargeRoster: false, winnersOnly: true };
  }

  const base = resolveBaseBtcFtActiveStrategyIds();
  return { ...base, winnersOnly: false };
}

export async function loadPromotedWinnerIds(opts: {
  storageNamespace: string;
  cloudAccountKey?: string | null;
  fetcher?: typeof fetch;
}): Promise<number[]> {
  const valid = new Set(BTC_FUTURE_TRADING_STRATEGY_IDS);
  const local = loadWinnersFromStorage(opts.storageNamespace).filter((id) => valid.has(id));
  const fetcher = opts.fetcher ?? (typeof fetch !== "undefined" ? fetch : null);
  if (!fetcher || !opts.cloudAccountKey) {
    return [...new Set(local)].slice(0, 20);
  }

  try {
    const params = new URLSearchParams({
      promotions: "1",
      account_key: opts.cloudAccountKey,
    });
    const res = await fetcher(`/api/paper-trades/strategy-research?${params.toString()}`, {
      credentials: "include",
      cache: "no-store",
    });
    if (!res.ok) return [...new Set(local)].slice(0, 20);
    const body = (await res.json()) as {
      ok?: boolean;
      promotions?: Array<{ strategyId?: unknown; status?: unknown }>;
    };
    const cloud = (body.ok && Array.isArray(body.promotions) ? body.promotions : [])
      .filter((row) => row.status === "winner")
      .map((row) => Number(row.strategyId))
      .filter((id) => Number.isFinite(id) && valid.has(id));
    return [...new Set([...local, ...cloud])].slice(0, 20);
  } catch {
    return [...new Set(local)].slice(0, 20);
  }
}

export function saveWinnersToStorage(ns: string, ids: number[]): void {
  if (typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(`${ns}${RESEARCH_WINNERS_LS_KEY_SUFFIX}`, JSON.stringify(ids));
  } catch { /* quota */ }
}

export function loadRetiredFromStorage(ns: string): Set<number> {
  if (typeof localStorage === "undefined") return new Set();
  try {
    const raw = localStorage.getItem(`${ns}${RESEARCH_RETIRED_LS_KEY_SUFFIX}`);
    if (!raw) return new Set();
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return new Set();
    return new Set((parsed as unknown[]).filter((x): x is number => typeof x === "number" && Number.isFinite(x)));
  } catch {
    return new Set();
  }
}

export function saveRetiredToStorage(ns: string, ids: ReadonlySet<number>): void {
  if (typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(`${ns}${RESEARCH_RETIRED_LS_KEY_SUFFIX}`, JSON.stringify([...ids]));
  } catch { /* quota */ }
}

// ---------------------------------------------------------------------------
// Research mode env policy helpers
// ---------------------------------------------------------------------------

/**
 * Signal threshold for research mode (default 22 — lower than production 26).
 * Only active when RESEARCH_MODE=1; never affects other modules.
 */
export function researchSignalThreshold(): number {
  if (!isResearchModeEnabled()) return 26;
  const env = process.env.NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD;
  if (env && env.trim() !== "") {
    const n = Number(env);
    if (Number.isFinite(n)) return Math.min(28, Math.max(22, Math.round(n)));
  }
  return 22; // research default
}

/**
 * Cooldown multiplier for research mode (default 0.5 = half cooldown = more trade frequency).
 * Only active when RESEARCH_MODE=1.
 */
export function researchCooldownMul(): number {
  if (!isResearchModeEnabled()) return 1;
  const env = process.env.NEXT_PUBLIC_BTC_FT_COOLDOWN_MUL;
  if (env && env.trim() !== "") {
    const n = Number(env);
    if (Number.isFinite(n) && n > 0) return Math.min(2, Math.max(0.1, n));
  }
  return 0.5;
}

/**
 * Min-move safety K multiplier for research (default 0.85 — slightly relaxed fee hurdle).
 * Only active when RESEARCH_MODE=1.
 */
export function researchMinMoveKMul(): number {
  if (!isResearchModeEnabled()) return 1;
  const env = process.env.NEXT_PUBLIC_BTC_FT_MIN_MOVE_K_MUL;
  if (env && env.trim() !== "") {
    const n = Number(env);
    if (Number.isFinite(n) && n > 0) return Math.min(2, Math.max(0.5, n));
  }
  return 0.85;
}

export function researchEnsureTradesEnabled(): boolean {
  return isResearchModeEnabled() && process.env.NEXT_PUBLIC_BTC_FT_RESEARCH_ENSURE_TRADES === "1";
}

export function researchSlippageBps(): number {
  return isResearchModeEnabled() ? 5 : 0;
}

export function researchEntryUtcSession(): DeskEntryUtcSession {
  if (!isResearchModeEnabled()) return { startHour: 0, endHour: 24 };
  const raw = process.env.NEXT_PUBLIC_BTC_FT_SESSION;
  if (!raw || raw.trim() === "") return { startHour: 0, endHour: 24 };
  const [startRaw, endRaw] = raw.split("-");
  const start = Number(startRaw);
  const end = Number(endRaw);
  if (!Number.isFinite(start) || !Number.isFinite(end)) return { startHour: 0, endHour: 24 };
  return {
    startHour: Math.min(23, Math.max(0, Math.floor(start))),
    endHour: Math.min(24, Math.max(0, Math.floor(end))),
  };
}

/**
 * Whether to skip auto-kill in research (default true in research — let LOSERs accumulate trades).
 * Only active when RESEARCH_MODE=1.
 */
export function researchDisableAutoKill(): boolean {
  if (!isResearchModeEnabled()) return false;
  // NEXT_PUBLIC_BTC_FT_DISABLE_AUTO_KILL=0 lets user opt back in to kill-switch even in research
  const env = process.env.NEXT_PUBLIC_BTC_FT_DISABLE_AUTO_KILL;
  if (env === "0") return false;
  return true; // default: disable auto-kill in research
}

/**
 * Whether relaxed confirmation is active for research mode.
 * Production: only in NODE_ENV=development. Research: any env.
 */
export function researchRelaxConfirm(): boolean {
  if (!isResearchModeEnabled()) {
    return process.env.NODE_ENV === "development" && process.env.NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM === "1";
  }
  const env = process.env.NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM;
  if (env === "0") return false;
  return true; // research default per spec
}
