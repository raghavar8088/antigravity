/**
 * The universe snapshot — every per-contract measurement, computed once.
 *
 * This is the expensive call, and everything else in the module reads its
 * output. The momentum board, the sector roll-up, the funding board, the open
 * interest quadrants, the setups and the drill-downs all share ONE snapshot
 * rather than each re-walking the bar set. That is not only a performance
 * choice: it is what makes the tabs agree with each other. Two tabs that each
 * recomputed a return would eventually disagree by a rounding step, and a
 * reader would have no way to know which one to believe.
 *
 * THREE FETCH PASSES, AND EACH EARNS ITS PLACE.
 *
 *   tickers   one request, the whole universe. Prices, 24h stats, funding,
 *             open interest, top of book, tick size, tags.
 *   daily     one request per symbol. Everything with a memory.
 *   hourly    one request per symbol, 48 hours. Exists solely so the 6-hour
 *             open-interest change can be classified against a 6-hour price
 *             change instead of the 24-hour one that happens to be at hand.
 *
 * The hourly pass roughly doubles the cold-build cost, and it buys one column.
 * It is here because the alternative is not "a cheaper buildup column" but "a
 * buildup column that is wrong in a way nobody can see" — the same defect the
 * equity module's sector code was fixed for.
 *
 * SECTORS ARE THE VENUE'S OWN TAGS, and they overlap. `smart_contracts`
 * co-occurs with `layer_1` on 24 contracts and with `layer_2` on 7; it is a
 * modifier, not a peer. So the board partitions on a PRIMARY tag chosen by a
 * fixed, documented priority, and every row still carries its full tag list so
 * the choice can be checked. Thirty contracts — XRPUSD among them — carry no
 * tag at all and are reported as Unclassified rather than being assigned
 * somewhere convenient.
 */

import {
  fetchDailyBarsMany,
  fetchHourlyBarsMany,
  fetchPerpTickers,
  fetchProductSpecs,
  type Bar,
  type PerpTicker,
  type ProductSpec,
} from "./delta";
import {
  readBasis,
  readFunding,
  readMicro,
  readOi,
  tradabilityScore,
  type BasisRead,
  type FundingRead,
  type MicroRead,
  type OiRead,
} from "./derivatives";
import * as H from "./horizons";
import type { HorizonKey } from "./horizons";

/** BTC is this venue's index: the thing everything else is measured against. */
export const BENCHMARK_SYMBOL = "BTCUSD";

/** How long a built snapshot is served before it is rebuilt. */
export const SNAPSHOT_TTL_MS = 5 * 60 * 1000;

/**
 * Primary-tag priority, most specific first.
 *
 * `smart_contracts` sits last on purpose: it is the venue's broadest tag (47
 * contracts) and is almost always a co-tag of a more specific one, so letting
 * it win would collapse layer_1, layer_2, defi and gaming into a single bucket
 * that describes nothing.
 */
const SECTOR_PRIORITY = [
  "metal",
  "xStock",
  "meme",
  "ai",
  "gaming",
  "nft",
  "sol_ecosystem",
  "layer_2",
  "layer_1",
  "defi",
  "smart_contracts",
];

export const SECTOR_LABELS: Record<string, string> = {
  metal: "Tokenised Metals",
  xStock: "Tokenised Equities",
  meme: "Meme",
  ai: "AI",
  gaming: "Gaming",
  nft: "NFT",
  sol_ecosystem: "Solana Ecosystem",
  layer_2: "Layer 2",
  layer_1: "Layer 1",
  defi: "DeFi",
  smart_contracts: "Smart Contract Platforms",
  Unclassified: "Unclassified",
};

export function primarySector(tags: string[]): string {
  for (const p of SECTOR_PRIORITY) if (tags.includes(p)) return p;
  return "Unclassified";
}

export function sectorLabel(key: string): string {
  return SECTOR_LABELS[key] ?? key;
}

export type ScreenerRow = {
  symbol: string;
  name: string;
  underlying: string;
  productId: number;
  sector: string;
  sectorLabel: string;
  tags: string[];
  assetClass: string;

  /** Last traded price — the number the board's returns are measured to. */
  price: number | null;
  markPrice: number | null;
  spotPrice: number | null;

  returns: Record<HorizonKey, number | null>;
  /** The venue's own rolling 24h change. A different window from returns["1d"]. */
  venue24hPct: number | null;

  turnoverUsd24h: number | null;
  volume24h: number | null;
  volumeX: number | null;

  emaHoldPct: number | null;
  upStreak: number;
  pctFrom1yHigh: number | null;
  pctFrom1yLow: number | null;
  breakout: { window: number; date: string } | null;

  sma20: number | null;
  sma50: number | null;
  sma200: number | null;
  ema20: number | null;
  atr14: number | null;
  atrPct: number | null;
  realisedVol30d: number | null;

  swingLow: number | null;
  donchianHigh20: number | null;
  donchianHigh50: number | null;
  baseLow20: number | null;

  funding: FundingRead;
  oi: OiRead;
  basis: BasisRead;
  micro: MicroRead;
  tradability: number | null;
  tradabilityBlockers: string[];

  /**
   * Venue margin spec, from `/v2/products`. Absent from the ticker payload, and
   * required before any leveraged paper position can be sized: it is what says
   * where the venue would liquidate.
   */
  maintenanceMarginPct: number | null;
  initialMarginPct: number | null;
  venueMaxLeverage: number | null;

  btcCorrelation30d: number | null;
  btcBeta30d: number | null;

  bars: number;
  /**
   * Tail of the close series, kept so per-horizon consistency can be measured
   * without re-reading the bar set. 181 covers the deepest horizon plus the bar
   * it measures from. Prefixed so serialisers can strip it from API payloads.
   */
  _closes: number[];
};

export type Snapshot = {
  rows: ScreenerRow[];
  benchmark: {
    symbol: string;
    available: boolean;
    returns: Record<HorizonKey, number | null>;
    price: number | null;
  };
  coverage: Record<HorizonKey, ReturnType<typeof H.coverage>>;
  universeListed: number;
  universeScanned: number;
  /** How many contracts returned a margin spec. 0 means the paper desk cannot size. */
  marginSpecs: number;
  barsMissing: string[];
  hourlyMissing: string[];
  totalTurnoverUsd24h: number;
  totalOiUsd: number;
  builtAt: number;
  buildMs: number;
};

let cached: { at: number; snap: Snapshot } | null = null;
let inFlight: Promise<Snapshot> | null = null;

/**
 * The daily bars the last build fetched, kept so the pattern scan does not
 * re-fetch them.
 *
 * The scan needs full OHLC series, which the snapshot rows deliberately do not
 * carry (they keep only a close tail). Before this cache existed the scan
 * re-requested all 220 daily series from the venue — a third full pass, on top
 * of the two the build already makes, for bars that were in memory a second
 * earlier. Held here rather than on the Snapshot object so it is dropped
 * independently and a stale snapshot cannot pin a bar set alive.
 */
let cachedBars: { at: number; bars: Map<string, Bar[]> } | null = null;

/** Bars from the most recent build, or null if none has completed here. */
export function lastBuildBars(): Map<string, Bar[]> | null {
  return cachedBars?.bars ?? null;
}

export class ScreenerError extends Error {}

/**
 * Build (or serve) the snapshot.
 *
 * Concurrent callers share one build. Without the in-flight latch, the first
 * page load fires eleven tab requests at a cold lambda and every one of them
 * starts its own 440-request scan of the venue — which is both slow and the
 * fastest way to get rate limited by the exchange.
 */
export async function getSnapshot(fresh = false): Promise<Snapshot> {
  const now = Date.now();
  if (!fresh && cached && now - cached.at < SNAPSHOT_TTL_MS) return cached.snap;
  if (inFlight) return inFlight;

  inFlight = build()
    .then((snap) => {
      cached = { at: Date.now(), snap };
      return snap;
    })
    .finally(() => {
      inFlight = null;
    });
  return inFlight;
}

export function cacheAgeMs(): number | null {
  return cached ? Date.now() - cached.at : null;
}

async function build(): Promise<Snapshot> {
  const started = Date.now();

  const tickers = await fetchPerpTickers();
  const bySymbol = new Map<string, PerpTicker>();
  for (const t of tickers) bySymbol.set(t.symbol, t);

  const symbols = tickers.map((t) => t.symbol);
  // Both bar passes run together: they hit different resolutions of the same
  // endpoint and neither depends on the other's result.
  const [daily, hourly, specs] = await Promise.all([
    fetchDailyBarsMany(symbols, H.LOOKBACK_DAYS, 16),
    fetchHourlyBarsMany(symbols, 48, 16),
    // One request for the whole universe. Failing soft: without it the paper
    // desk refuses to size a position rather than assuming a margin number,
    // which is the safe direction for the one input that decides where the
    // venue closes a trade out from under you.
    fetchProductSpecs().catch(() => new Map<string, ProductSpec>()),
  ]);

  const benchBars = daily.bars.get(BENCHMARK_SYMBOL) ?? [];
  const benchTicker = bySymbol.get(BENCHMARK_SYMBOL);
  const benchCloses = liveCloses(benchBars, benchTicker);
  const benchReturns: Record<HorizonKey, number | null> = benchCloses.length
    ? H.allHorizonReturns(benchCloses)
    : { "1d": null, "1w": null, "1m": null, "6m": null };

  const rows: ScreenerRow[] = [];
  let totalTurnover = 0;
  let totalOi = 0;

  for (const t of tickers) {
    const bars = daily.bars.get(t.symbol) ?? [];
    // Two bars is the floor for any return at all. A contract listed today has
    // no momentum to rank and is excluded from the board rather than shown with
    // an empty row that sorts to the top of an ascending column.
    if (bars.length < 2) continue;

    const closes = liveCloses(bars, t);
    const price = closes[closes.length - 1] ?? null;
    const returns = H.allHorizonReturns(closes);
    const hl = H.highLowContext(bars, 365, price);
    const atr14 = H.atr(bars);

    // Widest window first: reporting a 200-day breakout as a 20-day one
    // understates the event.
    let breakout: { window: number; date: string } | null = null;
    for (const w of [200, 50, 20]) {
      const when = H.donchianBreak(bars, w);
      if (when) {
        breakout = { window: w, date: when };
        break;
      }
    }

    const hourlyBars = hourly.bars.get(t.symbol) ?? [];
    const chg6h = sixHourChangePct(hourlyBars);

    const funding = readFunding(t);
    const oi = readOi(t, chg6h);
    const basis = readBasis(t);
    const micro = readMicro(t);
    const trad = tradabilityScore(micro, t.turnoverUsd24h);

    const sector = primarySector(t.tags);

    if (t.turnoverUsd24h) totalTurnover += t.turnoverUsd24h;
    if (t.oiValueUsd) totalOi += t.oiValueUsd;

    rows.push({
      symbol: t.symbol,
      name: t.description || t.symbol,
      underlying: t.underlying,
      productId: t.productId,
      sector,
      sectorLabel: sectorLabel(sector),
      tags: t.tags,
      // `tradfi` covers the tokenised equity and metal contracts. They trade on
      // the same venue with the same mechanics, but calling them crypto on a
      // crypto screener would misdescribe a third of the tradfi bucket.
      assetClass: t.topTag ?? "unknown",

      price,
      markPrice: t.markPrice,
      spotPrice: t.spotPrice,

      // Rounded at the source. Every board reads `returns` straight through, so
      // leaving the raw float here put 13.00940438871474% into a table cell.
      returns: {
        "1d": H.round(returns["1d"]),
        "1w": H.round(returns["1w"]),
        "1m": H.round(returns["1m"]),
        "6m": H.round(returns["6m"]),
      },
      venue24hPct: H.round(t.ltpChange24hPct),

      turnoverUsd24h: t.turnoverUsd24h,
      volume24h: t.volume24h,
      volumeX: H.round(H.volumeRatio(bars)),

      emaHoldPct: H.round(H.daysAboveEma(closes)),
      upStreak: H.upStreak(closes),
      pctFrom1yHigh: H.round(hl.pctFromHigh),
      pctFrom1yLow: H.round(hl.pctFromLow),
      breakout,

      sma20: H.round(H.sma(closes, 20), 6),
      sma50: H.round(H.sma(closes, 50), 6),
      sma200: H.round(H.sma(closes, 200), 6),
      ema20: H.round(H.emaLast(closes, 20), 6),
      atr14: H.round(atr14, 6),
      atrPct: price && atr14 ? H.round((atr14 / price) * 100, 2) : null,
      realisedVol30d: H.round(H.realisedVol(closes), 1),

      swingLow: H.round(H.lastSwingLow(bars), 6),
      donchianHigh20: H.round(H.donchianHigh(bars, 20), 6),
      donchianHigh50: H.round(H.donchianHigh(bars, 50), 6),
      baseLow20: H.round(H.donchianLow(bars, 20), 6),

      funding,
      oi,
      basis,
      micro,
      tradability: trad.score,
      tradabilityBlockers: trad.blockers,

      maintenanceMarginPct: specs.get(t.symbol)?.maintenanceMarginPct ?? null,
      initialMarginPct: specs.get(t.symbol)?.initialMarginPct ?? null,
      // The venue's own cap, derived from initial margin where the ticker's
      // `leverage` field disagrees. 100 / 0.5% = 200x on BTC.
      venueMaxLeverage:
        t.maxLeverage ??
        (specs.get(t.symbol)?.initialMarginPct
          ? Math.floor(100 / specs.get(t.symbol)!.initialMarginPct!)
          : null),

      btcCorrelation30d:
        t.symbol === BENCHMARK_SYMBOL ? 1 : H.round(H.correlation(closes, benchCloses), 3),
      btcBeta30d: t.symbol === BENCHMARK_SYMBOL ? 1 : H.round(H.beta(closes, benchCloses), 3),

      bars: bars.length,
      _closes: closes.slice(-181).map((c) => H.round(c, 8)!),
    });
  }

  const barsForCoverage = new Map<string, Bar[]>();
  for (const r of rows) barsForCoverage.set(r.symbol, daily.bars.get(r.symbol) ?? []);
  cachedBars = { at: Date.now(), bars: barsForCoverage };

  return {
    rows,
    benchmark: {
      symbol: BENCHMARK_SYMBOL,
      available: benchCloses.length > 0,
      returns: {
        "1d": H.round(benchReturns["1d"]),
        "1w": H.round(benchReturns["1w"]),
        "1m": H.round(benchReturns["1m"]),
        "6m": H.round(benchReturns["6m"]),
      },
      price: benchCloses[benchCloses.length - 1] ?? null,
    },
    coverage: {
      "1d": H.coverage(barsForCoverage, "1d"),
      "1w": H.coverage(barsForCoverage, "1w"),
      "1m": H.coverage(barsForCoverage, "1m"),
      "6m": H.coverage(barsForCoverage, "6m"),
    },
    universeListed: tickers.length,
    universeScanned: rows.length,
    marginSpecs: specs.size,
    barsMissing: daily.failed,
    hourlyMissing: hourly.failed,
    totalTurnoverUsd24h: totalTurnover,
    totalOiUsd: totalOi,
    builtAt: Date.now(),
    buildMs: Date.now() - started,
  };
}

/**
 * The close series with the forming bar replaced by the live traded price.
 *
 * Every horizon then measures from a COMPLETED past close to the current price,
 * which is one consistent basis across 24h, 7d, 30d and 6m. Leaving the forming
 * bar's own close in place would make the 24h column mean "since 00:00 UTC" — a
 * window that shrinks toward zero every midnight and resets, while the other
 * three columns kept measuring to now.
 *
 * Last TRADED price rather than mark. Mark is the venue's fair value and is the
 * right number for margin and for the basis column, where it appears; a
 * momentum board should show the price that actually printed.
 */
function liveCloses(bars: Bar[], t: PerpTicker | undefined): number[] {
  const closes = bars.map((b) => b.close);
  if (closes.length === 0) return closes;
  const live = t?.close ?? t?.markPrice ?? null;
  if (live !== null && live > 0) closes[closes.length - 1] = live;
  return closes;
}

/**
 * Price change over the last six hours, from hourly closes.
 *
 * Returns null rather than a shorter-window approximation when fewer than seven
 * bars are available: the entire point of this number is that it matches the
 * window the venue reports open-interest change over.
 */
function sixHourChangePct(hourly: Bar[]): number | null {
  if (hourly.length < 7) return null;
  const then = hourly[hourly.length - 7]!.close;
  const now = hourly[hourly.length - 1]!.close;
  if (then <= 0) return null;
  return (now / then - 1) * 100;
}

/** Strip the private compute-aid fields before a row crosses the API boundary. */
export function publicRow(r: ScreenerRow): Omit<ScreenerRow, "_closes"> {
  const { _closes: _drop, ...rest } = r;
  void _drop;
  return rest;
}
