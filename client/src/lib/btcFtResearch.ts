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
  BTC_FT_GENERATED_STRATEGY_IDS,
  BTC_FT_RESEARCH_FULL_POOL,
  CORE_BTC_FT_STRATEGY_IDS,
  isGeneratedPoolEnabled,
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
 *
 * Resolution order:
 *  1. `NEXT_PUBLIC_BTC_FT_POOL_IDS` (explicit comma list) — always honored if set.
 *     Each ID must exist in FUTURES_STRAT_DEFS AND be in the verified BTC FT
 *     roster (CORE + extended + generated). Caps at 200.
 *  2. `NEXT_PUBLIC_BTC_FT_GENERATED_POOL=1` (default when research mode is on) →
 *     CORE + extended (200–299) + generated (300–399). Cap 200.
 *  3. Default → CORE + extended (200–299) only. Cap 200.
 *
 * Generated 300–399 IDs are NEVER in the production CORE roster — they only join
 * the research pool when explicitly enabled. See `btcFtStrategyGenerator.ts`.
 */
/** Hard cap on pool size to prevent runaway resource use. 300 = 120 CORE/extended + 100 generated + headroom. */
const RESEARCH_POOL_CAP = 300;

export function resolveResearchPool(): number[] {
  const includeGenerated = isGeneratedPoolEnabled();
  const verifiedIds = new Set<number>(
    includeGenerated ? BTC_FT_RESEARCH_FULL_POOL : BTC_FUTURE_TRADING_STRATEGY_IDS,
  );
  const envPool = process.env.NEXT_PUBLIC_BTC_FT_POOL_IDS;
  if (envPool && envPool.trim() !== "") {
    const parsed = envPool
      .split(",")
      .map((s) => Number(s.trim()))
      .filter((n) => Number.isFinite(n) && n > 0);
    const validIds = new Set(FUTURES_STRAT_DEFS.map((s) => s.id));
    return [...new Set(parsed.filter((id) => validIds.has(id) && verifiedIds.has(id)))].slice(0, RESEARCH_POOL_CAP);
  }
  const base = includeGenerated ? BTC_FT_RESEARCH_FULL_POOL : BTC_FUTURE_TRADING_STRATEGY_IDS;
  return [...base].slice(0, RESEARCH_POOL_CAP);
}

/** Diagnostic: count of generated IDs currently in the pool. UI banner uses this. */
export function poolGeneratedCount(): number {
  if (!isGeneratedPoolEnabled()) return 0;
  const envPool = process.env.NEXT_PUBLIC_BTC_FT_POOL_IDS;
  const generated = new Set<number>(BTC_FT_GENERATED_STRATEGY_IDS);
  if (envPool && envPool.trim() !== "") {
    const parsed = envPool
      .split(",")
      .map((s) => Number(s.trim()))
      .filter((n) => Number.isFinite(n) && generated.has(n));
    return new Set(parsed).size;
  }
  return BTC_FT_GENERATED_STRATEGY_IDS.length;
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
  /**
   * Total fees paid as % of total positive gross PnL. WINNER requires < 80% —
   * a strategy that gives up 80%+ of its gross to fees is fee-bleeding even
   * if net is barely positive (often via the $2 win-floor).
   */
  feePctOfGross?: number | null;
}): ResearchVerdict {
  const { tradeCount, sumNet, expectancy, feePctOfGross } = stats;
  if (tradeCount < 10) return "INSUFFICIENT_DATA";
  // Tighter LOSER: retire after 12 trades when sumNet < -$1 OR expectancy < -$0.05.
  // Previous threshold (15 trades, sumNet < -$2) allowed strategies to fee-bleed for days.
  if (tradeCount >= 12 && (sumNet < -1 || expectancy < -0.05)) return "LOSER";
  if (tradeCount >= 20 && expectancy > 0 && sumNet > 0) {
    // Reject strategies whose positive net is dominated by fees — usually
    // means the gross edge is too thin and one bad batch wipes it out.
    if (feePctOfGross !== null && feePctOfGross !== undefined && feePctOfGross >= 80) {
      return "CANDIDATE";
    }
    return "WINNER";
  }
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

/** @deprecated localStorage removed — returns [] synchronously; use loadResearchStateFromMongo for async load. */
export function loadWinnersFromStorage(_ns: string): number[] {
  return [];
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
 * When NEXT_PUBLIC_BTC_FT_WINNERS_ONLY=1, research rotation is bypassed.
 * Priority: explicit env IDs → promoted winners → CORE 20 basket (never empty).
 */
export function resolveBtcFtActiveStrategyIds(opts: {
  storageNamespace?: string;
  winnerIds?: readonly number[];
} = {}): BtcFtResearchRosterResolution {
  if (isWinnersOnlyModeEnabled()) {
    const ns = opts.storageNamespace ?? "btc_future_trading_20";
    const valid = new Set(BTC_FUTURE_TRADING_STRATEGY_IDS);

    const explicit = parseExplicitBtcFtStrategyIds(24);
    if (explicit.length > 0) {
      return { ids: explicit, source: "env", isLargeRoster: false, winnersOnly: true };
    }

    const promoted = [
      ...new Set(
        (opts.winnerIds && opts.winnerIds.length > 0 ? opts.winnerIds : resolveCoreWinners(ns))
          .map((id) => Math.floor(id))
          .filter((id) => valid.has(id)),
      ),
    ].slice(0, 20);
    if (promoted.length > 0) {
      return { ids: promoted, source: "winners", isLargeRoster: false, winnersOnly: true };
    }

    return {
      ids: [...CORE_BTC_FT_STRATEGY_IDS],
      source: "core",
      isLargeRoster: false,
      winnersOnly: true,
    };
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

/** Save winners to MongoDB paper_research collection. */
export function saveWinnersToStorage(ns: string, ids: number[], accountKey?: string): void {
  if (!accountKey) return;
  void fetch("/api/paper-research", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ accountKey, namespace: ns, winners: ids }),
  }).catch(() => {});
}

/** @deprecated localStorage removed — returns empty Set; use loadResearchStateFromMongo for async load. */
export function loadRetiredFromStorage(_ns: string): Set<number> {
  return new Set();
}

/** Save retired IDs to MongoDB paper_research collection. */
export function saveRetiredToStorage(ns: string, ids: ReadonlySet<number>, accountKey?: string): void {
  if (!accountKey) return;
  void fetch("/api/paper-research", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ accountKey, namespace: ns, retiredIds: [...ids] }),
  }).catch(() => {});
}

/** Load winners and retired IDs from MongoDB for a given namespace. */
export async function loadResearchStateFromMongo(
  ns: string,
  accountKey: string,
): Promise<{ winners: number[]; retiredIds: Set<number> }> {
  try {
    const params = new URLSearchParams({ account_key: accountKey, namespace: ns });
    const res = await fetch(`/api/paper-research?${params.toString()}`, {
      cache: "no-store",
      credentials: "include",
    });
    if (!res.ok) return { winners: [], retiredIds: new Set() };
    const data = (await res.json()) as { ok?: boolean; winners?: number[]; retiredIds?: number[] };
    return {
      winners: Array.isArray(data.winners) ? data.winners : [],
      retiredIds: new Set(Array.isArray(data.retiredIds) ? data.retiredIds : []),
    };
  } catch {
    return { winners: [], retiredIds: new Set() };
  }
}

// ---------------------------------------------------------------------------
// Research mode env policy helpers
// ---------------------------------------------------------------------------

/**
 * Signal threshold for research mode (default 20 — matches production floor).
 * Only active when RESEARCH_MODE=1; never affects other modules.
 */
export function researchSignalThreshold(): number {
  if (!isResearchModeEnabled()) return 26;
  const env = process.env.NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD;
  if (env && env.trim() !== "") {
    const n = Number(env);
    if (Number.isFinite(n)) return Math.min(28, Math.max(18, Math.round(n)));
  }
  return 20; // research default (was 22 — too high for generated templates with limited HTF data)
}

/**
 * Cooldown multiplier for research mode. Default 0.75 (was 0.5) — slows the re-entry
 * cadence so we don't fee-bleed the same strategy in successive ticks.
 * Only active when RESEARCH_MODE=1.
 */
export function researchCooldownMul(): number {
  if (!isResearchModeEnabled()) return 1;
  const env = process.env.NEXT_PUBLIC_BTC_FT_COOLDOWN_MUL;
  if (env && env.trim() !== "") {
    const n = Number(env);
    if (Number.isFinite(n) && n > 0) return Math.min(2, Math.max(0.1, n));
  }
  return 0.75;
}

/**
 * Min-move safety K multiplier for research. Default 0.7 — keeps a meaningful
 * fee hurdle while allowing entries during moderate-volatility periods.
 * (Previous default 1.0 blocked entries when 1m ATR < ~$90 at $95k BTC.)
 * Only active when RESEARCH_MODE=1.
 */
export function researchMinMoveKMul(): number {
  if (!isResearchModeEnabled()) return 1;
  const env = process.env.NEXT_PUBLIC_BTC_FT_MIN_MOVE_K_MUL;
  if (env && env.trim() !== "") {
    const n = Number(env);
    if (Number.isFinite(n) && n > 0) return Math.min(2, Math.max(0.5, n));
  }
  return 0.7;
}

export function researchEnsureTradesEnabled(): boolean {
  if (!isResearchModeEnabled()) return false;
  // Default ON in research mode — after 2h with zero trades, threshold drops by 4
  // (floor 18) so zero-data strategies get a chance to accumulate verdicts.
  // Set NEXT_PUBLIC_BTC_FT_RESEARCH_ENSURE_TRADES=0 to disable.
  return process.env.NEXT_PUBLIC_BTC_FT_RESEARCH_ENSURE_TRADES !== "0";
}

/**
 * Maximum trade closes per strategy per UTC day in research mode. Default 8.
 * Once a strategy hits the cap, new entries are skipped (with
 * `deskSkippedDailyStratCap` increment) until the next UTC day.
 * Env: `NEXT_PUBLIC_BTC_FT_DAILY_STRAT_CAP`. Clamped to [1, 100].
 * Returns 0 (disabled) outside research mode.
 */
export function researchDailyStratCap(): number {
  if (!isResearchModeEnabled()) return 0;
  const env = process.env.NEXT_PUBLIC_BTC_FT_DAILY_STRAT_CAP;
  if (env && env.trim() !== "") {
    const n = Math.floor(Number(env));
    if (Number.isFinite(n) && n > 0) return Math.min(100, Math.max(1, n));
  }
  return 8;
}

/**
 * Extract the template-family key from a strategy name. Same family means
 * "same evalMinuteSignal branch" — VWAP_V0_LONG_203, VWAP_V0_LONG_243,
 * VWAP_V0_LONG_283 are all family `BTCFT_VWAP_V0_LONG`.
 *
 * Research-mode dedupe rule: at most ONE open position per template family per side,
 * so we don't pay 3× round-trip fees for one signal.
 */
export function btcFtTemplateFamilyKey(name: string): string {
  // Strip trailing _<digits> ID suffix.
  const m = name.match(/^(.*?)(?:_\d+)?$/);
  return m ? m[1]! : name;
}

export function researchSlippageBps(): number {
  return isResearchModeEnabled() ? 5 : 0;
}

/**
 * Min absolute net win USD floor used in `paperNetPnlOnClose`. The legacy floor
 * (`$2.00`) inflates tiny gross wins to a fake "$2 outlier" — useful for display
 * polish, harmful for honest expectancy measurement.
 *
 * Default returns 0 in research mode (raw, untruncated net) and 2.0 outside.
 * Env `NEXT_PUBLIC_DESK_MIN_ABS_NET_WIN_USD` overrides; clamped to [0, 5].
 */
export function deskMinAbsNetWinUsd(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MIN_ABS_NET_WIN_USD;
  if (raw !== undefined && raw.trim() !== "") {
    const n = Number(raw);
    if (Number.isFinite(n) && n >= 0) return Math.min(5, n);
  }
  return isResearchModeEnabled() ? 0 : 2;
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
