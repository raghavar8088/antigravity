/**
 * Orchestration: breadth, the eleven boards, and per-feed honesty.
 *
 * BREADTH IS COMPUTED, NOT SCRAPED. Every number in the header — advances
 * versus declines, the share of the universe above its 20/50/200-day average,
 * new highs against new lows, aggregate open interest, the market-wide funding
 * tilt — is a count over the snapshot this module already builds. Live rather
 * than delayed, and with no dependency on a third party staying reachable.
 *
 * THE SETUPS TAB IS THE POINT OF THE MODULE. Everything else describes the
 * market; that tab answers "what would I actually trade", and it is the screen
 * most likely to be trusted without reading the small print. So it is the
 * strictest: a contract reaches a shortlist only by passing that mode's gate,
 * and every row carries reward-to-risk NET of real taker fees AND the funding
 * the hold would pay. A setup whose net R:R falls below 1 is still shown, with
 * `worthTaking: false`, because silently hiding it would leave the reader
 * unable to tell "no setups today" from "today's setups do not clear their
 * costs" — and those are very different market states.
 *
 * WHAT THIS MODULE WILL NOT DO is rank the whole venue by return and call the
 * top of it a shortlist. On a universe where 24h turnover spans from $1.8bn to
 * $4,400, the raw top of a return board is reliably a list of contracts nobody
 * can trade. Liquidity and the tick grid are gates, not columns.
 */

import * as H from "./horizons";
import type { HorizonKey } from "./horizons";
import * as P from "./patterns";
import type { PatternHit } from "./patterns";
import * as PL from "./plans";
import * as R from "./reasons";
import * as S from "./sectors";
import type { Bar } from "./delta";
import {
  BENCHMARK_SYMBOL,
  cacheAgeMs,
  getSnapshot,
  lastBuildBars,
  publicRow,
  sectorLabel,
  SECTOR_LABELS,
  type ScreenerRow,
  type Snapshot,
} from "./universe";

export class ScreenerRequestError extends Error {}

/** Contracts under this 24h turnover are excluded from ranked boards by default. */
export const DEFAULT_MIN_TURNOVER = 250_000;

// ── pattern scan cache ──────────────────────────────────────────────────────

const PATTERN_TTL_MS = 15 * 60 * 1000;
let patternCache: { at: number; snapshotAt: number; hits: PatternHit[]; elapsedMs: number; weeklyReady: number } | null =
  null;

/**
 * Scan every contract on daily and weekly bars.
 *
 * Keyed on the SNAPSHOT's build time as well as the clock: a fresh snapshot
 * means new bars, and serving pattern hits computed against the previous bar
 * set beside prices from the current one would put two different moments in one
 * row. Pure CPU over bars already in memory — no network at all.
 */
async function patternScan(snap: Snapshot, fresh = false): Promise<{
  hits: PatternHit[];
  scanned: number;
  elapsedMs: number;
  weeklyReady: number;
}> {
  if (
    !fresh &&
    patternCache &&
    patternCache.snapshotAt === snap.builtAt &&
    Date.now() - patternCache.at < PATTERN_TTL_MS
  ) {
    return {
      hits: patternCache.hits,
      scanned: snap.rows.length,
      elapsedMs: patternCache.elapsedMs,
      weeklyReady: patternCache.weeklyReady,
    };
  }

  const started = Date.now();
  const hits: PatternHit[] = [];
  let weeklyReady = 0;

  const barsBySymbol = await barsForScan(snap);

  for (const row of snap.rows) {
    const daily = barsBySymbol.get(row.symbol);
    if (!daily || daily.length < 30) continue;
    hits.push(...P.scanSymbol(row.symbol, row.sector, daily, "1d"));
    const weekly = P.toWeekly(daily);
    // Below 45 weekly bars the longer shapes — cup & handle, rounding bottom —
    // cannot form at all, and running them anyway produces hits drawn on a
    // handful of points.
    if (weekly.length >= 45) {
      weeklyReady++;
      hits.push(...P.scanSymbol(row.symbol, row.sector, weekly, "1w"));
    }
  }

  const elapsedMs = Date.now() - started;
  patternCache = { at: Date.now(), snapshotAt: snap.builtAt, hits, elapsedMs, weeklyReady };
  return { hits, scanned: snap.rows.length, elapsedMs, weeklyReady };
}

/**
 * Full OHLC series for the pattern scan and the target maths.
 *
 * REUSES THE BARS THE SNAPSHOT BUILD ALREADY FETCHED. The snapshot rows carry
 * only a tail of closes — enough for consistency, not enough to draw a shape —
 * so this needs the real series. Re-requesting them from the venue would be a
 * THIRD full pass over 220 contracts on top of the two the build already makes,
 * for bars that were in memory a second earlier; the first version of this file
 * did exactly that and turned a 440-request build into a 660-request one.
 *
 * The fallback fetch stays for the case the build ran in a different container
 * than the one serving this request, which on a serverless platform is normal
 * rather than exceptional.
 */
async function barsForScan(snap: Snapshot) {
  const shared = lastBuildBars();
  if (shared && shared.size >= snap.rows.length) return shared;

  const { fetchDailyBarsMany } = await import("./delta");
  const { bars } = await fetchDailyBarsMany(
    snap.rows.map((r) => r.symbol),
    H.LOOKBACK_DAYS,
    16,
  );
  return bars;
}

function hitsBySymbol(hits: PatternHit[]): Map<string, PatternHit[]> {
  const m = new Map<string, PatternHit[]>();
  for (const h of hits) {
    const l = m.get(h.symbol);
    if (l) l.push(h);
    else m.set(h.symbol, [h]);
  }
  return m;
}

// ── config ──────────────────────────────────────────────────────────────────

export function config() {
  return {
    horizons: H.HORIZON_ORDER.map((h) => ({
      key: h,
      label: H.HORIZON_LABELS[h],
      days: H.HORIZONS[h],
    })),
    sectors: Object.entries(SECTOR_LABELS).map(([key, label]) => ({ key, label })),
    timeframes: [
      { key: "1d", label: "Daily" },
      { key: "1w", label: "Weekly" },
    ],
    patternCatalog: P.catalog(),
    planKinds: PL.PLAN_KINDS.map((k) => ({
      key: k,
      label: PL.PLAN_LABELS[k],
      holdHours: PL.PLAN_HOLD_HOURS[k],
    })),
    volumeStates: Object.entries(VOLUME_STATES).map(([key, text]) => ({
      key,
      label: key.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase()),
      text,
    })),
    buildupKinds: [
      { key: "long_buildup", label: "Long buildup" },
      { key: "short_buildup", label: "Short buildup" },
      { key: "long_unwinding", label: "Long unwinding" },
      { key: "short_covering", label: "Short covering" },
      { key: "flat", label: "No change" },
      { key: "unclassified", label: "Not classified" },
    ],
    benchmark: BENCHMARK_SYMBOL,
    perTradeNotionalUsd: PL.PER_TRADE_NOTIONAL,
    defaultMinTurnover: DEFAULT_MIN_TURNOVER,
  };
}

// ── summary / breadth ───────────────────────────────────────────────────────

export async function summary(fresh = false) {
  const snap = await getSnapshot(fresh);
  const rows = snap.rows;

  const share = (pred: (r: ScreenerRow) => boolean | null) => {
    const eligible = rows.filter((r) => pred(r) !== null);
    if (eligible.length === 0) return { pct: null, n: 0, of: 0 };
    const hit = eligible.filter((r) => pred(r) === true).length;
    return { pct: H.round((hit / eligible.length) * 100, 1), n: hit, of: eligible.length };
  };
  const above = (key: "sma20" | "sma50" | "sma200") => (r: ScreenerRow) =>
    r[key] !== null && r.price !== null ? r.price > r[key]! : null;

  const day = rows.map((r) => r.returns["1d"]).filter((v): v is number => v !== null);
  const advances = day.filter((v) => v > 0).length;
  const declines = day.filter((v) => v < 0).length;

  const newHighs = rows.filter((r) => r.pctFrom1yHigh !== null && r.pctFrom1yHigh >= -0.5).length;
  const newLows = rows.filter((r) => r.pctFrom1yLow !== null && r.pctFrom1yLow <= 0.5).length;

  const funding = rows.map((r) => r.funding.ratePct8h).filter((v): v is number => v !== null);
  const longsPaying = funding.filter((v) => v > 0.0005).length;
  const shortsPaying = funding.filter((v) => v < -0.0005).length;
  const medianFunding = median(funding);

  const buildupCounts: Record<string, number> = {};
  for (const r of rows) buildupCounts[r.oi.buildup] = (buildupCounts[r.oi.buildup] ?? 0) + 1;

  const btcTurnover = rows.find((r) => r.symbol === BENCHMARK_SYMBOL)?.turnoverUsd24h ?? 0;
  const corr = rows
    .filter((r) => r.symbol !== BENCHMARK_SYMBOL)
    .map((r) => r.btcCorrelation30d)
    .filter((v): v is number => v !== null);

  const tradable = rows.filter((r) => r.micro.stopExpressible).length;

  return {
    universe: rows.length,
    listed: snap.universeListed,
    advances,
    declines,
    unchanged: day.length - advances - declines,
    advanceDeclineRatio: declines > 0 ? H.round(advances / declines) : null,
    aboveSma20: share(above("sma20")),
    aboveSma50: share(above("sma50")),
    aboveSma200: share(above("sma200")),
    new1yHighs: newHighs,
    new1yLows: newLows,

    // The crypto-only half of the header. None of these exist on the equity
    // screener, and together they say more about the state of the market than
    // the moving-average breadth above them does.
    totalTurnoverUsd24h: snap.totalTurnoverUsd24h,
    totalOiUsd: snap.totalOiUsd,
    btcTurnoverSharePct:
      snap.totalTurnoverUsd24h > 0
        ? H.round((btcTurnover / snap.totalTurnoverUsd24h) * 100, 1)
        : null,
    funding: {
      medianPct8h: H.round(medianFunding, 5),
      medianAnnualisedPct: medianFunding !== null ? H.round(medianFunding * 1095, 1) : null,
      longsPaying,
      shortsPaying,
      neutral: funding.length - longsPaying - shortsPaying,
      tilt:
        longsPaying > shortsPaying * 1.5
          ? "longs_crowded"
          : shortsPaying > longsPaying * 1.5
            ? "shorts_crowded"
            : "balanced",
    },
    openInterest: {
      totalUsd: snap.totalOiUsd,
      byBuildup: buildupCounts,
      note:
        "Buildup is classified over a 6-hour window on BOTH axes — the venue's 6h open-interest " +
        "change against a 6h price change from hourly bars. Contracts without seven hourly bars " +
        "report as unclassified rather than being judged against the 24h move.",
    },
    btcCorrelation: {
      median: H.round(median(corr), 3),
      above85: corr.filter((c) => c >= 0.85).length,
      of: corr.length,
      note:
        "When most of the venue is highly correlated to BTC, sector rotation and single-name " +
        "momentum are largely the same trade wearing different tickers. The number is shown so " +
        "that can be checked rather than assumed either way.",
    },
    tradableContracts: {
      n: tradable,
      of: rows.length,
      note:
        `${rows.length - tradable} contracts cannot express a ${0.35}% stop within ` +
        `20 ticks of their own price grid. They still appear on momentum boards — that is a real ` +
        `price move — but no plan on them is marked tradable.`,
    },

    benchmark: snap.benchmark,
    coverage: snap.coverage,
    barsMissing: snap.barsMissing.length,
    hourlyMissing: snap.hourlyMissing.length,
    builtAt: snap.builtAt,
    buildMs: snap.buildMs,
    cacheAgeMs: cacheAgeMs(),
  };
}

function median(v: number[]): number | null {
  if (v.length === 0) return null;
  const s = [...v].sort((a, b) => a - b);
  const m = Math.floor(s.length / 2);
  return s.length % 2 ? s[m]! : (s[m - 1]! + s[m]!) / 2;
}

// ── momentum ────────────────────────────────────────────────────────────────

export type MomentumOpts = {
  horizon?: HorizonKey;
  sector?: string | null;
  assetClass?: string | null;
  limit?: number;
  minTurnover?: number | null;
  fresh?: boolean;
};

export async function momentumBoard(opts: MomentumOpts = {}) {
  const horizon = opts.horizon ?? "1d";
  if (!H.HORIZONS[horizon]) {
    throw new ScreenerRequestError(`unknown horizon ${horizon}; expected one of ${H.HORIZON_ORDER.join(", ")}`);
  }
  const snap = await getSnapshot(opts.fresh);
  const sectorBoard = S.rollUp(snap, horizon);
  const sectorIndex = new Map(sectorBoard.sectors.map((s) => [s.sector, s]));
  const bench = snap.benchmark.returns[horizon];
  const floor = opts.minTurnover === null ? 0 : (opts.minTurnover ?? DEFAULT_MIN_TURNOVER);

  let pool = snap.rows.filter((r) => r.returns[horizon] !== null);
  if (floor > 0) {
    // A contract with no turnover figure at all is kept: filtering on a missing
    // number would drop it for a data gap rather than for being illiquid.
    pool = pool.filter((r) => r.turnoverUsd24h === null || r.turnoverUsd24h >= floor);
  }
  if (opts.sector) pool = pool.filter((r) => r.sector === opts.sector);
  if (opts.assetClass) pool = pool.filter((r) => r.assetClass === opts.assetClass);

  const population = pool.map((r) => r.returns[horizon]!);
  const days = H.HORIZONS[horizon];

  const out = pool.map((r) => {
    const ret = r.returns[horizon]!;
    const sec = sectorIndex.get(r.sector);
    const cons = days > 1 ? H.round(H.consistency(r._closes, days)) : null;
    const rsBench = H.relativeStrength(ret, bench);

    const stack = R.build({
      symbol: r.symbol,
      returnPct: ret,
      volumeX: r.volumeX,
      breakout: r.breakout,
      pctFromPeriodHigh: r.pctFrom1yHigh,
      emaHoldPct: r.emaHoldPct,
      consistency: cons,
      upStreak: r.upStreak,
      rsBenchmark: rsBench,
      benchmarkSymbol: snap.benchmark.symbol,
      funding: r.funding,
      oi: r.oi,
      basis: r.basis,
      micro: r.micro,
      sector: sec
        ? { sector: sec.label, returnPct: sec.returnPct, rank: sec.rank, of: sectorBoard.count }
        : null,
      correlation: { btc: r.btcCorrelation30d, beta: r.btcBeta30d },
    });

    return {
      ...publicRow(r),
      // A downsampled close series for the row's sparkline.
      //
      // Downsampled rather than sent whole: 181 closes x 220 rows is a payload
      // several times the size of everything else on the board, to draw a line
      // 74 pixels wide. Forty points is more than that line can resolve.
      spark: sparkFrom(r._closes, 40),
      returnPct: ret,
      rankPct: H.round(H.percentileRank(ret, population)),
      rsBenchmark: H.round(rsBench),
      rsSector: H.round(H.relativeStrength(ret, sec?.returnPct ?? null)),
      consistency: cons,
      sectorReturnPct: sec?.returnPct ?? null,
      why: R.chips(stack),
      whySummary: R.summarise(r.symbol, ret, stack),
      character: R.classify(stack),
      score: score(r, ret, rsBench),
      rank: 0,
    };
  });

  out.sort((a, b) => (b.score ?? 0) - (a.score ?? 0) || (b.returnPct ?? 0) - (a.returnPct ?? 0));
  out.forEach((r, i) => {
    r.rank = i + 1;
  });

  return {
    horizon,
    horizonLabel: H.HORIZON_LABELS[horizon],
    benchmark: snap.benchmark,
    coverage: snap.coverage[horizon],
    sectorFilter: opts.sector ?? null,
    assetClassFilter: opts.assetClass ?? null,
    minTurnover: floor,
    universe: snap.rows.length,
    count: out.length,
    rows: out.slice(0, opts.limit ?? 120),
  };
}

/**
 * A blunt 0-100 composite for default ordering, not a prediction.
 *
 * Deliberately simple and fully inspectable: the move, relative strength,
 * participation, trend quality, positioning, and a penalty for a contract
 * nobody can put a stop on. Each term is capped so none can dominate. It exists
 * so the board has a sensible default sort; the columns behind it are what a
 * decision should actually use.
 */
/**
 * Evenly sample a series down to at most `points`.
 *
 * Samples rather than truncates, so the sparkline still spans the whole window
 * — taking the last 40 closes would silently turn a 6-month trace into a
 * 40-day one while still being labelled 6 months.
 */
function sparkFrom(closes: number[], points: number): number[] {
  if (closes.length <= points) return closes;
  const step = (closes.length - 1) / (points - 1);
  const out: number[] = [];
  for (let i = 0; i < points; i++) out.push(closes[Math.round(i * step)]!);
  return out;
}

function score(r: ScreenerRow, ret: number | null, rs: number | null): number | null {
  if (ret === null) return null;
  let s = 30;
  s += Math.max(-20, Math.min(40, ret * 2));
  if (rs !== null) s += Math.max(-10, Math.min(20, rs * 1.5));
  if (r.volumeX !== null) s += Math.min(15, (r.volumeX - 1) * 7);
  if (r.emaHoldPct !== null) s += r.emaHoldPct * 0.15;
  if (r.breakout) s += 10;
  if (r.pctFrom1yHigh !== null && r.pctFrom1yHigh >= -2) s += 8;

  // Positioning: a rise on rising open interest is worth more than the same
  // rise on a short squeeze, and this is the only place the board can say so.
  if (r.oi.buildup === "long_buildup") s += 8;
  else if (r.oi.buildup === "short_covering") s -= 6;

  // A contract whose grid cannot hold a stop is not an opportunity however well
  // it ranks. The penalty is large enough to push it out of the visible top
  // without hiding it entirely.
  if (!r.micro.stopExpressible) s -= 20;

  return H.round(Math.max(0, Math.min(100, s)), 1);
}

// ── symbol detail ───────────────────────────────────────────────────────────

export async function symbolDetail(symbol: string, fresh = false) {
  const sym = symbol.trim().toUpperCase();
  const snap = await getSnapshot(fresh);
  const row = snap.rows.find((r) => r.symbol === sym);
  if (!row) {
    throw new ScreenerRequestError(
      `${sym} is not a listed Delta perpetual with stored bars. The universe is built from ` +
        `/v2/tickers, so a contract that was delisted or has fewer than two daily bars is absent.`,
    );
  }

  const scan = await patternScan(snap);
  const hits = scan.hits.filter((h) => h.symbol === sym);

  const perHorizon: Record<string, unknown> = {};
  for (const h of H.HORIZON_ORDER) {
    const board = S.rollUp(snap, h);
    const sec = board.sectors.find((s) => s.sector === row.sector);
    const ret = row.returns[h];
    const bench = snap.benchmark.returns[h];
    const rs = H.relativeStrength(ret, bench);
    const stack = R.build({
      symbol: sym,
      returnPct: ret,
      volumeX: row.volumeX,
      breakout: row.breakout,
      pctFromPeriodHigh: row.pctFrom1yHigh,
      emaHoldPct: row.emaHoldPct,
      consistency: H.round(H.consistency(row._closes, H.HORIZONS[h])),
      upStreak: row.upStreak,
      rsBenchmark: rs,
      benchmarkSymbol: snap.benchmark.symbol,
      funding: row.funding,
      oi: row.oi,
      basis: row.basis,
      micro: row.micro,
      sector: sec
        ? { sector: sec.label, returnPct: sec.returnPct, rank: sec.rank, of: board.count }
        : null,
      correlation: { btc: row.btcCorrelation30d, beta: row.btcBeta30d },
    });
    perHorizon[h] = {
      label: H.HORIZON_LABELS[h],
      returnPct: ret,
      benchmarkPct: bench,
      rsBenchmark: H.round(rs),
      sectorReturnPct: sec?.returnPct ?? null,
      sectorRank: sec?.rank ?? null,
      reasons: stack,
      summary: R.summarise(sym, ret, stack),
      character: R.classify(stack),
    };
  }

  return {
    ...publicRow(row),
    horizons: perHorizon,
    patterns: hits.sort(
      (a, b) =>
        (a.state === "TRIGGERED" ? 0 : 1) - (b.state === "TRIGGERED" ? 0 : 1) ||
        b.confidence - a.confidence,
    ),
    tradePlans: PL.plansFor(row, hits),
    gates: PL.PLAN_KINDS.map((k) => {
      const [ok, why] = PL.gate(row, k);
      return { kind: k, label: PL.PLAN_LABELS[k], passes: ok, whyNot: why || null };
    }),
    narrative: {
      available: false,
      reason:
        "No news or on-chain source is wired up for this module, so nothing is checked and no " +
        "narrative is invented in its place.",
    },
  };
}

// ── sectors ─────────────────────────────────────────────────────────────────

export async function sectorBoard(horizon: HorizonKey | null, fresh = false) {
  const snap = await getSnapshot(fresh);
  if (horizon) {
    if (!H.HORIZONS[horizon]) throw new ScreenerRequestError(`unknown horizon ${horizon}`);
    return S.rollUp(snap, horizon);
  }
  return S.allHorizons(snap);
}

export async function sectorDrilldown(sector: string, horizon: HorizonKey, fresh = false) {
  const snap = await getSnapshot(fresh);
  if (!H.HORIZONS[horizon]) throw new ScreenerRequestError(`unknown horizon ${horizon}`);
  try {
    return S.drillDown(snap, sector, horizon);
  } catch (e) {
    throw new ScreenerRequestError(e instanceof Error ? e.message : String(e));
  }
}

// ── volume ──────────────────────────────────────────────────────────────────

export const VOLUME_STATES: Record<string, string> = {
  accumulation: "Price up on rising volume — buyers paying up",
  distribution: "Price down on rising volume — sellers hitting bids",
  weak_rally: "Price up but volume FALLING — a drift with nothing behind it",
  selling_dried: "Price down on falling volume — sellers running out",
  churn: "Heavy volume, price going nowhere — contracts changing hands",
};

const VOLUME_WINDOWS: Record<string, number> = { "1d": 1, "1w": 7, "1m": 30 };
const VOLUME_WINDOW_HORIZON: Record<string, HorizonKey> = { "1d": "1d", "1w": "1w", "1m": "1m" };
const MIN_VOL_RATIO = 1.5;

function classifyVolume(ret: number | null, volRatio: number | null): [string, string] {
  if (ret === null || volRatio === null) {
    return ["unknown", "Not enough history to compare volume against its own average"];
  }
  const heavy = volRatio >= 1.2;
  const light = volRatio < 0.9;
  // "Flat" scales with the horizon: 1% is a quiet day for a perpetual and a
  // large move for a week. A fixed band would label most of the 30-day board as
  // churn.
  const flatBand = volRatio >= 1.2 ? 2 : 1;
  if (Math.abs(ret) < flatBand && heavy) return ["churn", VOLUME_STATES.churn!];
  if (ret > 0) {
    if (heavy) return ["accumulation", VOLUME_STATES.accumulation!];
    if (light) return ["weak_rally", VOLUME_STATES.weak_rally!];
    return ["accumulation", VOLUME_STATES.accumulation!];
  }
  if (heavy) return ["distribution", VOLUME_STATES.distribution!];
  if (light) return ["selling_dried", VOLUME_STATES.selling_dried!];
  return ["distribution", VOLUME_STATES.distribution!];
}

export async function volumeBoard(
  window = "1d",
  state: string | null = null,
  limit = 80,
  fresh = false,
) {
  const days = VOLUME_WINDOWS[window];
  if (!days) {
    throw new ScreenerRequestError(`unknown window ${window}; expected 1d, 1w or 1m`);
  }
  const snap = await getSnapshot(fresh);
  const scan = await patternScan(snap);
  const byS = hitsBySymbol(scan.hits);
  const bars = await barsForScan(snap);
  const horizon = VOLUME_WINDOW_HORIZON[window]!;
  const sectorIdx = new Map(S.rollUp(snap, horizon).sectors.map((s) => [s.sector, s]));

  const rows = [];
  for (const r of snap.rows) {
    const b = bars.get(r.symbol);
    if (!b) continue;
    const [cur, base] = H.windowVolume(b, days);
    if (!cur || !base || base <= 0) continue;
    const ratio = cur / base;
    if (ratio < MIN_VOL_RATIO) continue;

    const ret = r.returns[horizon];
    const [st, stText] = classifyVolume(ret, ratio);
    if (state && st !== state) continue;

    const hits = byS.get(r.symbol) ?? [];
    const sec = sectorIdx.get(r.sector);

    // OPEN INTEREST IS THE TIEBREAKER, and it is the one thing that can
    // contradict the price-volume label. Price up on heavy volume reads as
    // "accumulation" by definition — but if open interest fell over the same
    // window, that volume was shorts closing, which is very nearly the opposite
    // of accumulation. The state stays a pure price-volume fact, because that
    // is what it measures; the conflict gets its own field so the table shows
    // the disagreement instead of leaving the label to be read as a verdict it
    // cannot support. This is the crypto replacement for the equity module's
    // delivery-percentage tiebreaker, and it is strictly better: delivery is a
    // once-a-day file that often never arrives, and this is live for every
    // contract.
    let conflict: string | null = null;
    if (st === "accumulation" && r.oi.buildup === "short_covering") {
      conflict =
        "Open interest FELL as price rose — the volume is shorts closing, not buyers accumulating. " +
        "A squeeze exhausts when the shorts run out.";
    } else if (st === "distribution" && r.oi.buildup === "short_buildup") {
      conflict =
        "Open interest ROSE as price fell — this is not exit liquidity, it is new shorts being " +
        "positioned into the decline.";
    } else if (st === "distribution" && r.oi.buildup === "long_unwinding") {
      conflict =
        "Open interest fell with price — longs liquidating rather than sellers pressing. " +
        "Forced supply tends to end abruptly.";
    }

    rows.push({
      symbol: r.symbol,
      name: r.name,
      sector: r.sector,
      sectorLabel: r.sectorLabel,
      price: r.price,
      returnPct: ret,
      volumeRatio: H.round(ratio),
      volume: cur,
      volumeBaseline: base,
      turnoverUsd24h: r.turnoverUsd24h,
      state: st,
      stateLabel: st.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase()),
      stateText: stText,
      priceConfirms: st === "accumulation" || st === "distribution",
      oiBuildup: r.oi.buildup,
      oiBuildupLabel: r.oi.buildupLabel,
      oiChangePct6h: r.oi.oiChangePct6h,
      oiConflict: conflict,
      fundingPct8h: r.funding.ratePct8h,
      sectorReturnPct: sec?.returnPct ?? null,
      reasons: volumeReasons(r, st, ratio, sec?.returnPct ?? null, hits),
      target: nextTarget(b, r.price ?? 0, hits, r.atr14),
      patterns: hits.slice(0, 3).map((h) => ({
        pattern: h.pattern,
        state: h.state,
        timeframe: h.timeframeLabel,
      })),
    });
  }

  rows.sort((a, b) => (b.volumeRatio ?? 0) - (a.volumeRatio ?? 0));
  const byState: Record<string, number> = {};
  for (const r of rows) byState[r.state] = (byState[r.state] ?? 0) + 1;

  return {
    window,
    windowLabel: { "1d": "24 Hours", "1w": "7 Days", "1m": "30 Days" }[window],
    days,
    count: rows.length,
    minVolumeRatio: MIN_VOL_RATIO,
    byState,
    states: Object.entries(VOLUME_STATES).map(([k, v]) => ({
      key: k,
      label: k.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase()),
      text: v,
    })),
    note:
      "Volume is measured in CONTRACTS against this contract's own trailing average, never across " +
      "contracts — one BTCUSD contract is 0.001 BTC and one ADAUSD contract is 1 ADA, so raw " +
      "contract counts are not comparable between symbols. The USD turnover column is.",
    rows: rows.slice(0, limit),
  };
}

function volumeReasons(
  r: ScreenerRow,
  state: string,
  ratio: number,
  sectorRet: number | null,
  hits: PatternHit[],
): string[] {
  const out: string[] = [];
  if (ratio >= 3) {
    out.push(
      `Volume is ${ratio.toFixed(1)}x its own recent average — far outside this contract's normal ` +
        `participation`,
    );
  } else if (ratio >= MIN_VOL_RATIO) {
    out.push(`Volume is ${ratio.toFixed(1)}x its own recent average`);
  }

  if (r.oi.buildup !== "unclassified" && r.oi.buildup !== "flat") out.push(r.oi.text);
  if (r.funding.ratePct8h !== null && Math.abs(r.funding.ratePct8h) >= 0.03) out.push(r.funding.text);

  if (r.breakout) {
    out.push(
      `It broke its ${r.breakout.window}-day high on ${r.breakout.date} — volume arriving on a ` +
        `breakout is the confirmation the breakout needs`,
    );
  }

  const trig = hits.find((h) => h.state === "TRIGGERED");
  if (trig) {
    out.push(
      `A ${trig.pattern} completed on the ${trig.timeframeLabel.toLowerCase()} chart at the same time`,
    );
  }

  if (sectorRet !== null && Math.abs(sectorRet) >= 2.5) {
    out.push(
      `${r.sectorLabel} is ${sectorRet >= 0 ? "+" : ""}${sectorRet.toFixed(1)}% over the same ` +
        `window — the volume is sector-wide, not specific to this contract`,
    );
  }

  if (state === "churn") {
    out.push(
      "Price has not moved despite the volume — someone is absorbing supply at this level, and " +
        "the direction it resolves is not yet decided",
    );
  } else if (state === "weak_rally") {
    out.push("Note the volume is BELOW average — this rise is not being paid for");
  }
  return out;
}

/**
 * The next level up, with the method that produced it stated.
 *
 * Ordered by how much each method is actually worth: a chart pattern's own
 * measured move beats a prior swing high, which beats a base projection, which
 * beats an ATR multiple. Every branch names itself so a reader can weigh it; a
 * bare number here would imply a confidence none of these have.
 */
function nextTarget(
  bars: Bar[],
  price: number,
  hits: PatternHit[],
  atr: number | null,
) {
  const bullish = hits.filter((h) => h.direction === "bullish");
  const triggered = bullish.filter((h) => h.state === "TRIGGERED");
  for (const [pool, note] of [
    [triggered, "triggered"],
    [bullish, "forming"],
  ] as const) {
    if (pool.length === 0) continue;
    const best = pool.reduce((a, b) => (b.confidence > a.confidence ? b : a));
    if (best.target > price) {
      return {
        target: H.round(best.target, 8),
        upsidePct: H.round((best.target / price - 1) * 100),
        method: `${best.pattern} measured move (${note})`,
        strength: note === "triggered" ? "strong" : "moderate",
        note: best.rationale,
      };
    }
  }

  const resistance = H.nearestResistanceAbove(bars, price);
  if (resistance) {
    return {
      target: H.round(resistance, 8),
      upsidePct: H.round((resistance / price - 1) * 100),
      method: "nearest prior swing high",
      strength: "moderate",
      note: "The last level this contract turned back from — the first thing that has to give way",
    };
  }

  const hi = H.donchianHigh(bars, 20);
  const lo = H.donchianLow(bars, 20);
  if (hi && lo && hi > lo && price >= hi * 0.995) {
    const projected = price + (hi - lo);
    return {
      target: H.round(projected, 8),
      upsidePct: H.round((projected / price - 1) * 100),
      method: "measured move — 20-day base height projected up",
      strength: "moderate",
      note: "No overhead resistance: price is at or above its 20-day high",
    };
  }

  if (atr && atr > 0) {
    const projected = price + 3 * atr;
    return {
      target: H.round(projected, 8),
      upsidePct: H.round((projected / price - 1) * 100),
      method: "3x ATR projection",
      strength: "weak",
      note: "Arithmetic on volatility, not a forecast — no level or pattern supports this",
    };
  }

  return {
    target: null,
    upsidePct: null,
    method: "none available",
    strength: "none",
    note: "No pattern, no overhead level and no usable ATR — no honest target",
  };
}

// ── funding ─────────────────────────────────────────────────────────────────

export async function fundingBoard(
  side: "longs" | "shorts" | null = null,
  limit = 120,
  minTurnover: number | null = DEFAULT_MIN_TURNOVER,
  fresh = false,
) {
  const snap = await getSnapshot(fresh);
  const floor = minTurnover ?? 0;
  let pool = snap.rows.filter((r) => r.funding.ratePct8h !== null);
  if (floor > 0) pool = pool.filter((r) => r.turnoverUsd24h === null || r.turnoverUsd24h >= floor);

  const population = pool.map((r) => r.funding.ratePct8h!);
  const rows = pool
    .filter((r) => !side || r.funding.payer === side)
    .map((r) => {
      const rate = r.funding.ratePct8h!;
      // A funding rate whose sign disagrees with the price move is the most
      // informative row on this board: the crowd is positioned one way and the
      // price is going the other, which is the setup for the unwind.
      const ret = r.returns["1d"];
      let divergence: string | null = null;
      if (ret !== null && Math.abs(rate) >= 0.03) {
        if (rate > 0 && ret < -2) {
          divergence =
            `Longs are paying ${rate.toFixed(3)}% per 8h while price fell ${ret.toFixed(1)}% — ` +
            `the crowded side is underwater and still paying to stay there.`;
        } else if (rate < 0 && ret > 2) {
          divergence =
            `Shorts are paying ${Math.abs(rate).toFixed(3)}% per 8h while price rose ` +
            `${ret.toFixed(1)}% — shorts are underwater and financing the move against them.`;
        }
      }
      return {
        symbol: r.symbol,
        name: r.name,
        sector: r.sector,
        sectorLabel: r.sectorLabel,
        price: r.price,
        returnPct24h: ret,
        returnPct7d: r.returns["1w"],
        fundingPct8h: rate,
        fundingDailyPct: r.funding.dailyPct,
        fundingAnnualPct: r.funding.annualisedPct,
        payer: r.funding.payer,
        fundingText: r.funding.text,
        percentile: H.round(H.percentileRank(rate, population), 1),
        /** What a $1,000 long pays (or receives) per day, in dollars. */
        costPerDayPer1kUsd: H.round((rate * 3 * 1000) / 100, 4),
        /** And over a five-day swing hold — the number that kills swing plans. */
        costPer5dPer1kUsd: H.round((rate * 15 * 1000) / 100, 4),
        oiValueUsd: r.oi.oiValueUsd,
        oiBuildup: r.oi.buildup,
        basisBps: r.basis.basisBps,
        turnoverUsd24h: r.turnoverUsd24h,
        divergence,
      };
    });

  rows.sort((a, b) => Math.abs(b.fundingPct8h) - Math.abs(a.fundingPct8h));

  return {
    count: rows.length,
    sideFilter: side,
    minTurnover: floor,
    note:
      "Funding is quoted in PERCENT per 8-hour settlement, exactly as Delta publishes it. " +
      "Annualised multiplies by 1,095 (three settlements a day, 365 days) and is simple, not " +
      "compounded. Positive means longs pay shorts. This measurement has no equity analogue at " +
      "all — it is a continuously published price of positioning.",
    rows: rows.slice(0, limit),
  };
}

// ── open interest ───────────────────────────────────────────────────────────

export async function oiBoard(
  buildup: string | null = null,
  limit = 120,
  minTurnover: number | null = DEFAULT_MIN_TURNOVER,
  fresh = false,
) {
  const snap = await getSnapshot(fresh);
  const floor = minTurnover ?? 0;
  let pool = snap.rows;
  if (floor > 0) pool = pool.filter((r) => r.turnoverUsd24h === null || r.turnoverUsd24h >= floor);
  if (buildup) pool = pool.filter((r) => r.oi.buildup === buildup);

  const rows = pool.map((r) => ({
    symbol: r.symbol,
    name: r.name,
    sector: r.sector,
    sectorLabel: r.sectorLabel,
    price: r.price,
    returnPct24h: r.returns["1d"],
    priceChangePct6h: r.oi.priceChangePct6h,
    oiValueUsd: r.oi.oiValueUsd,
    oiContracts: r.oi.oiContracts,
    oiChangeUsd6h: r.oi.oiChangeUsd6h,
    oiChangePct6h: r.oi.oiChangePct6h,
    oiToTurnover: r.oi.oiToTurnover,
    buildup: r.oi.buildup,
    buildupLabel: r.oi.buildupLabel,
    text: r.oi.text,
    fundingPct8h: r.funding.ratePct8h,
    turnoverUsd24h: r.turnoverUsd24h,
  }));

  rows.sort((a, b) => Math.abs(b.oiChangeUsd6h ?? 0) - Math.abs(a.oiChangeUsd6h ?? 0));

  const byBuildup: Record<string, number> = {};
  for (const r of rows) byBuildup[r.buildup] = (byBuildup[r.buildup] ?? 0) + 1;

  return {
    count: rows.length,
    buildupFilter: buildup,
    byBuildup,
    totalOiUsd: snap.totalOiUsd,
    unclassified: rows.filter((r) => r.buildup === "unclassified").length,
    note:
      "Both axes are measured over the SAME six hours: the venue's own 6h open-interest change " +
      "against a 6h price change computed from hourly bars. A contract without seven hourly bars " +
      "reports as unclassified rather than being judged against its 24h move — pairing a 6h OI " +
      "delta with a 24h price change would be a statement about neither window. The equity " +
      "screener this page mirrors lists this reading as permanently unavailable.",
    rows: rows.slice(0, limit),
  };
}

// ── basis ───────────────────────────────────────────────────────────────────

export async function basisBoard(
  state: string | null = null,
  limit = 120,
  minTurnover: number | null = DEFAULT_MIN_TURNOVER,
  fresh = false,
) {
  const snap = await getSnapshot(fresh);
  const floor = minTurnover ?? 0;
  let pool = snap.rows.filter((r) => r.basis.basisBps !== null);
  if (floor > 0) pool = pool.filter((r) => r.turnoverUsd24h === null || r.turnoverUsd24h >= floor);
  if (state) pool = pool.filter((r) => r.basis.state === state);

  const rows = pool.map((r) => ({
    symbol: r.symbol,
    name: r.name,
    sector: r.sector,
    sectorLabel: r.sectorLabel,
    markPrice: r.markPrice,
    spotPrice: r.spotPrice,
    lastPrice: r.price,
    basisBps: r.basis.basisBps,
    state: r.basis.state,
    text: r.basis.text,
    fundingPct8h: r.funding.ratePct8h,
    // Basis and funding are the same force seen twice. When they DISAGREE —
    // a premium with negative funding, say — one of the two is about to move,
    // and the row says so rather than showing two numbers side by side.
    agreesWithFunding:
      r.basis.basisBps !== null && r.funding.ratePct8h !== null
        ? Math.sign(r.basis.basisBps) === Math.sign(r.funding.ratePct8h) ||
          Math.abs(r.basis.basisBps) < 5
        : null,
    returnPct24h: r.returns["1d"],
    oiValueUsd: r.oi.oiValueUsd,
    turnoverUsd24h: r.turnoverUsd24h,
  }));

  rows.sort((a, b) => Math.abs(b.basisBps ?? 0) - Math.abs(a.basisBps ?? 0));

  return {
    count: rows.length,
    stateFilter: state,
    note:
      "Basis is the perpetual's mark against the venue's spot index, in basis points. A perp has " +
      "no expiry, so nothing forces convergence except funding — basis and funding are the same " +
      "force measured twice, and the column flagging when they DISAGREE is where one of the two " +
      "is about to move.",
    rows: rows.slice(0, limit),
  };
}

// ── microstructure ──────────────────────────────────────────────────────────

export async function microBoard(
  tradableOnly = false,
  limit = 250,
  minTurnover: number | null = null,
  fresh = false,
) {
  const snap = await getSnapshot(fresh);
  const floor = minTurnover ?? 0;
  let pool = snap.rows;
  if (floor > 0) pool = pool.filter((r) => r.turnoverUsd24h === null || r.turnoverUsd24h >= floor);
  if (tradableOnly) pool = pool.filter((r) => r.micro.stopExpressible);

  const rows = pool.map((r) => ({
    symbol: r.symbol,
    name: r.name,
    sector: r.sector,
    sectorLabel: r.sectorLabel,
    price: r.price,
    bestBid: r.micro.bestBid,
    bestAsk: r.micro.bestAsk,
    spreadBps: r.micro.spreadBps,
    bidSize: r.micro.bidSize,
    askSize: r.micro.askSize,
    bookImbalance: r.micro.bookImbalance,
    imbalanceLabel: r.micro.imbalanceLabel,
    tickSize: r.micro.tickSize,
    tickBps: r.micro.tickBps,
    stopTicks: r.micro.stopTicks,
    stopExpressible: r.micro.stopExpressible,
    gridNote: r.micro.gridNote,
    contractValue: r.micro.contractValue,
    notionalPerContract: r.micro.notionalPerContract,
    maxLeverage: r.micro.maxLeverage,
    bandHeadroomUpPct: r.micro.bandHeadroomUpPct,
    bandHeadroomDownPct: r.micro.bandHeadroomDownPct,
    roundTripFeePct: r.micro.roundTripFeePct,
    breakEvenMovePct: r.micro.breakEvenMovePct,
    atrPct: r.atrPct,
    // The single most useful ratio on this tab: how much of a typical day's
    // range is eaten just getting in and out. Above 20% the contract is a
    // toll booth.
    costShareOfAtrPct:
      r.micro.breakEvenMovePct !== null && r.atrPct && r.atrPct > 0
        ? H.round((r.micro.breakEvenMovePct / r.atrPct) * 100, 1)
        : null,
    tradability: r.tradability,
    blockers: r.tradabilityBlockers,
    turnoverUsd24h: r.turnoverUsd24h,
  }));

  rows.sort((a, b) => (b.tradability ?? 0) - (a.tradability ?? 0));

  return {
    count: rows.length,
    tradableOnly,
    blockedCount: snap.rows.filter((r) => !r.micro.stopExpressible).length,
    note:
      "The equity screener this page mirrors explicitly REFUSES to report order-book strength, " +
      "on the grounds that it has no book and a proxy dressed up as one would be a lie. Delta " +
      "publishes best bid, best ask and both resting sizes for every contract, so the number that " +
      "module declined to invent is simply measured here. The grid column is the one that matters " +
      "most: 26 contracts on this venue have already been banned from the live desks because " +
      "their tick size cannot hold a stop, and this recomputes that test live for all of them.",
    rows: rows.slice(0, limit),
  };
}

// ── correlation ─────────────────────────────────────────────────────────────

export async function correlationBoard(
  limit = 250,
  minTurnover: number | null = DEFAULT_MIN_TURNOVER,
  fresh = false,
) {
  const snap = await getSnapshot(fresh);
  const floor = minTurnover ?? 0;
  let pool = snap.rows.filter((r) => r.symbol !== BENCHMARK_SYMBOL && r.btcCorrelation30d !== null);
  if (floor > 0) pool = pool.filter((r) => r.turnoverUsd24h === null || r.turnoverUsd24h >= floor);

  const rows = pool.map((r) => {
    const corr = r.btcCorrelation30d!;
    const b = r.btcBeta30d;
    const ret = r.returns["1m"];
    const benchRet = snap.benchmark.returns["1m"];
    // Alpha here means: the move that is NOT explained by BTC at this beta.
    // It is the only column on the board that separates a token that led from
    // one that was carried.
    const explained = b !== null && benchRet !== null ? b * benchRet : null;
    const alpha = ret !== null && explained !== null ? ret - explained : null;
    return {
      symbol: r.symbol,
      name: r.name,
      sector: r.sector,
      sectorLabel: r.sectorLabel,
      price: r.price,
      correlation30d: corr,
      beta30d: b,
      returnPct30d: ret,
      benchmarkPct30d: benchRet,
      explainedByBtcPct: H.round(explained),
      alphaPct: H.round(alpha),
      independence: H.round((1 - Math.abs(corr)) * 100, 1),
      realisedVol30d: r.realisedVol30d,
      turnoverUsd24h: r.turnoverUsd24h,
      verdict:
        corr >= 0.9
          ? "BTC proxy"
          : corr >= 0.75
            ? "mostly BTC"
            : corr >= 0.4
              ? "partly independent"
              : "independent",
    };
  });

  rows.sort((a, b) => (b.alphaPct ?? -999) - (a.alphaPct ?? -999));

  const corrs = rows.map((r) => r.correlation30d);
  const med = median(corrs);

  return {
    count: rows.length,
    benchmark: BENCHMARK_SYMBOL,
    medianCorrelation: H.round(med, 3),
    proxies: rows.filter((r) => r.correlation30d >= 0.9).length,
    independent: rows.filter((r) => r.correlation30d < 0.4).length,
    warning:
      med !== null && med >= 0.75
        ? `Median 30-day correlation to BTC across this board is ${med.toFixed(2)}. At that level, ` +
          `most of what the momentum and sector tabs are ranking is one trade in ${rows.length} ` +
          `costumes, and diversifying across the leaderboard would not diversify the risk.`
        : null,
    note:
      "Correlation and beta are computed over 30 days of daily returns, and only for contracts " +
      "with a full 30 days of history — correlating a two-week listing against a 400-day BTC " +
      "series over whatever they happen to share would produce a number computed on a different " +
      "sample for every row, which cannot be ranked against itself. Alpha is the 30-day return " +
      "minus beta times BTC's, so it is what the token did that BTC does not explain.",
    rows: rows.slice(0, limit),
  };
}

// ── patterns ────────────────────────────────────────────────────────────────

export type PatternOpts = {
  timeframe?: string | null;
  pattern?: string | null;
  family?: string | null;
  state?: string | null;
  direction?: string | null;
  sector?: string | null;
  limit?: number;
  fresh?: boolean;
};

export async function patternBoard(opts: PatternOpts = {}) {
  const snap = await getSnapshot(opts.fresh);
  const scan = await patternScan(snap, opts.fresh);
  let rows = scan.hits;

  if (opts.timeframe) rows = rows.filter((r) => r.timeframe === opts.timeframe);
  if (opts.pattern) rows = rows.filter((r) => r.template === opts.pattern);
  if (opts.family) rows = rows.filter((r) => r.family === opts.family);
  // Hoisted: narrowing a property does not survive into the filter callback.
  const wantState = opts.state ? opts.state.toUpperCase() : null;
  if (wantState) rows = rows.filter((r) => r.state === wantState);
  if (opts.direction) rows = rows.filter((r) => r.direction === opts.direction);
  if (opts.sector) rows = rows.filter((r) => r.sector === opts.sector);

  const sorted = [...rows].sort(
    (a, b) =>
      (a.state === "TRIGGERED" ? 0 : 1) - (b.state === "TRIGGERED" ? 0 : 1) ||
      b.confidence - a.confidence ||
      (b.rewardRisk ?? 0) - (a.rewardRisk ?? 0),
  );

  const priceBySymbol = new Map(snap.rows.map((r) => [r.symbol, r]));

  return {
    scanned: scan.scanned,
    count: sorted.length,
    triggered: sorted.filter((r) => r.state === "TRIGGERED").length,
    forming: sorted.filter((r) => r.state === "FORMING").length,
    elapsedMs: scan.elapsedMs,
    weeklyCoverage: {
      symbols: snap.rows.length,
      withEnoughWeeklyBars: scan.weeklyReady,
      pct: snap.rows.length ? H.round((scan.weeklyReady / snap.rows.length) * 100, 1) : 0,
      note:
        "A weekly bar needs 45 weeks of history before the longer shapes (cup & handle, rounding " +
        "bottom) can form. How much daily history the venue returns for a contract decides this, " +
        "and a token listed this year simply cannot show them.",
    },
    filters: {
      timeframe: opts.timeframe ?? null,
      pattern: opts.pattern ?? null,
      family: opts.family ?? null,
      state: opts.state ?? null,
      direction: opts.direction ?? null,
      sector: opts.sector ?? null,
    },
    catalog: P.catalog(),
    note:
      "TRIGGERED means price has CLOSED THROUGH the pattern's own boundary — an unbroken shape is " +
      "not a signal. FORMING means the shape is complete and only the break is missing, found by " +
      "appending one synthetic bar just past the structure and re-running the UNMODIFIED " +
      "detector; the trigger level shown is the exact price at which the break would happen. " +
      "Candlestick and structure templates are never probed: there is no forming engulfing " +
      "candle, and probing one would manufacture a signal rather than find one.",
    rows: sorted.slice(0, opts.limit ?? 300).map((h) => ({
      ...h,
      sectorLabel: sectorLabel(h.sector),
      livePrice: priceBySymbol.get(h.symbol)?.price ?? null,
      turnoverUsd24h: priceBySymbol.get(h.symbol)?.turnoverUsd24h ?? null,
    })),
  };
}

// ── setups ──────────────────────────────────────────────────────────────────

export async function setups(kind: PL.PlanKind, limit = 40, fresh = false) {
  if (!PL.PLAN_KINDS.includes(kind)) {
    throw new ScreenerRequestError(
      `unknown setup kind ${kind}; expected one of ${PL.PLAN_KINDS.join(", ")}`,
    );
  }
  const snap = await getSnapshot(fresh);
  const scan = await patternScan(snap);
  const byS = hitsBySymbol(scan.hits);

  const horizon: HorizonKey = kind === "scalp" ? "1d" : kind === "swing" ? "1m" : "1w";
  const sectorBoardData = S.rollUp(snap, horizon);
  const sectorIdx = new Map(sectorBoardData.sectors.map((s) => [s.sector, s]));
  const bench = snap.benchmark.returns[horizon];

  const qualified = [];
  const rejections = new Map<string, number>();

  for (const row of snap.rows) {
    const [passes, whyNot] = PL.gate(row, kind);
    if (!passes) {
      rejections.set(whyNot, (rejections.get(whyNot) ?? 0) + 1);
      continue;
    }
    const hits = byS.get(row.symbol) ?? [];
    const plan =
      kind === "scalp"
        ? PL.scalpPlan(row, hits)
        : kind === "swing"
          ? PL.swingPlan(row, hits)
          : PL.breakoutPlan(row, hits);
    if (!plan) {
      rejections.set("no usable levels for a plan", (rejections.get("no usable levels for a plan") ?? 0) + 1);
      continue;
    }

    const sec = sectorIdx.get(row.sector);
    const ret = row.returns[horizon];
    const rs = H.relativeStrength(ret, bench);
    const stack = R.build({
      symbol: row.symbol,
      returnPct: ret,
      volumeX: row.volumeX,
      breakout: row.breakout,
      pctFromPeriodHigh: row.pctFrom1yHigh,
      emaHoldPct: row.emaHoldPct,
      consistency: H.round(H.consistency(row._closes, H.HORIZONS[horizon])),
      upStreak: row.upStreak,
      rsBenchmark: rs,
      benchmarkSymbol: snap.benchmark.symbol,
      funding: row.funding,
      oi: row.oi,
      basis: row.basis,
      micro: row.micro,
      sector: sec
        ? { sector: sec.label, returnPct: sec.returnPct, rank: sec.rank, of: sectorBoardData.count }
        : null,
      correlation: { btc: row.btcCorrelation30d, beta: row.btcBeta30d },
    });

    qualified.push({
      symbol: row.symbol,
      name: row.name,
      sector: row.sector,
      sectorLabel: row.sectorLabel,
      price: row.price,
      returnPct: ret,
      volumeX: row.volumeX,
      rsBenchmark: H.round(rs),
      sectorReturnPct: sec?.returnPct ?? null,
      turnoverUsd24h: row.turnoverUsd24h,
      fundingPct8h: row.funding.ratePct8h,
      oiBuildup: row.oi.buildup,
      oiBuildupLabel: row.oi.buildupLabel,
      btcCorrelation30d: row.btcCorrelation30d,
      plan,
      why: R.chips(stack),
      whySummary: R.summarise(row.symbol, ret, stack),
      character: R.classify(stack),
      patterns: hits.slice(0, 3).map((h) => ({
        pattern: h.pattern,
        state: h.state,
        timeframe: h.timeframeLabel,
      })),
    });
  }

  // Rank by net reward-to-risk, then by the strength of the move behind it.
  // Plans that are not worth taking sink but stay visible.
  qualified.sort(
    (a, b) =>
      (a.plan.worthTaking ? 0 : 1) - (b.plan.worthTaking ? 0 : 1) ||
      (b.plan.netRr ?? 0) - (a.plan.netRr ?? 0) ||
      (b.returnPct ?? 0) - (a.returnPct ?? 0),
  );

  const worth = qualified.filter((q) => q.plan.worthTaking).length;
  const topRejections = [...rejections.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8)
    .map(([reason, n]) => ({ reason, n }));

  return {
    kind,
    label: PL.PLAN_LABELS[kind],
    horizon,
    universe: snap.rows.length,
    qualified: qualified.length,
    worthTaking: worth,
    rejected: snap.rows.length - qualified.length,
    rejectionReasons: topRejections,
    notionalPerTradeUsd: PL.PER_TRADE_NOTIONAL,
    holdHours: PL.PLAN_HOLD_HOURS[kind],
    note:
      "Reward-to-risk is NET of Delta's real taker fee on BOTH legs AND the funding this hold " +
      "would actually pay, charged in whole 8-hour settlements. Rows with worthTaking=false are " +
      "shown, not hidden — 'no setups today' and 'today's setups do not clear their costs' are " +
      "different facts, and on a venue where a five-day hold can cross fifteen funding stamps the " +
      "second one is common.",
    rows: qualified.slice(0, limit),
  };
}

// ── sources ─────────────────────────────────────────────────────────────────

/**
 * Per-feed honesty.
 *
 * This tab exists so a silent data failure reads as a data failure. Without it,
 * a venue outage or a rate limit looks identical to a quiet market — the tables
 * just come back thin — and that ambiguity is how a screener starts lying
 * without anyone changing a line of code.
 */
export async function sources(fresh = false) {
  let snap: Snapshot | null = null;
  let err: string | null = null;
  try {
    snap = await getSnapshot(fresh);
  } catch (e) {
    err = e instanceof Error ? e.message : String(e);
  }

  const feeds = [
    {
      name: "Delta /v2/tickers",
      role: "spine — price, funding, open interest, book, tick size, tags",
      ok: snap !== null,
      detail:
        snap !== null
          ? `${snap.universeListed} perpetuals listed, ${snap.universeScanned} with enough bars to rank`
          : `failed: ${err}`,
    },
    {
      name: "Delta /v2/history/candles (1d)",
      role: "every number with a memory — returns, averages, ATR, patterns, correlation",
      ok: snap !== null && snap.barsMissing.length < snap.universeListed * 0.1,
      detail:
        snap === null
          ? "not attempted — the ticker call failed first"
          : snap.barsMissing.length === 0
            ? `daily bars returned for all ${snap.universeScanned} contracts`
            : `${snap.barsMissing.length} contracts returned no daily bars: ` +
              `${snap.barsMissing.slice(0, 8).join(", ")}${snap.barsMissing.length > 8 ? "..." : ""}. ` +
              `Those rows report null for every horizon rather than a partial-window return.`,
    },
    {
      name: "Delta /v2/history/candles (1h)",
      role: "the 6-hour price change the open-interest quadrant is measured against",
      ok: snap !== null && snap.hourlyMissing.length < snap.universeListed * 0.2,
      detail:
        snap === null
          ? "not attempted"
          : `${snap.hourlyMissing.length} contracts returned no hourly bars. Each of those reports ` +
            `its buildup as unclassified rather than being judged against a 24h price move.`,
    },
    {
      name: `Benchmark (${BENCHMARK_SYMBOL})`,
      role: "relative strength, correlation and beta",
      ok: snap?.benchmark.available ?? false,
      detail: snap?.benchmark.available
        ? `benchmark bars present, last price ${snap.benchmark.price}`
        : "no benchmark bars — relative-strength and correlation columns read as unavailable " +
          "rather than zero, because zero would read as 'matched the market'",
    },
    {
      name: "Delta engine (Go)",
      role: "not used by this module",
      ok: null,
      detail:
        "This screener never calls the trading engine. Every input is a public Delta market-data " +
        "endpoint, so the real-money process is not on its path and cannot be affected by it.",
    },
    {
      name: "Market cap / supply",
      role: "sector weighting",
      ok: false,
      detail:
        "Not published by this venue. The sector board is therefore equal-weighted and " +
        "TURNOVER-weighted, and never claims to be cap-weighted — a weighting we cannot source " +
        "would be a fiction.",
    },
    {
      name: "News / on-chain narrative",
      role: "tier-3 reasons",
      ok: false,
      detail:
        "No source configured. News is not checked, and no narrative is invented in its place. " +
        "The reason engine reports 'unexplained' when nothing mechanical or positional supports " +
        "a move.",
    },
  ];

  return {
    feeds,
    snapshot: snap
      ? {
          builtAt: snap.builtAt,
          buildMs: snap.buildMs,
          cacheAgeMs: cacheAgeMs(),
          coverage: snap.coverage,
        }
      : null,
    checkedAt: Date.now(),
  };
}
