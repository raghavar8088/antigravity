/**
 * Delta Exchange India public market data — the Crypto Screener's only feed.
 *
 * WHY THIS LIVES IN THE NEXT APP AND NOT IN THE GO ENGINE. Every input this
 * module needs is PUBLIC: `/v2/tickers` and `/v2/history/candles` take no key,
 * no signature and no IP allow-list. The engine binary is the process that
 * trades real money; rebuilding and redeploying it so an analytics page can
 * read a public endpoint would put a live trading system through a release for
 * no capability it does not already have. The screener therefore runs entirely
 * on the Vercel side, and the engine is untouched by it.
 *
 * TWO CALLS, AND THEY ARE NOT INTERCHANGEABLE.
 *
 *   /v2/tickers   ONE request returns all ~220 perpetuals with 24h OHLC, 24h
 *                 turnover, mark and spot price, funding rate, open interest
 *                 and its 6h change, the top of book, tick size, contract
 *                 value, max leverage and the venue's own tag list. This is
 *                 the widest single payload in the app, and most of what makes
 *                 this screener richer than the equity one comes from it.
 *
 *   /v2/history/candles  ONE request PER SYMBOL. Everything with a memory —
 *                 multi-horizon returns, moving averages, ATR, Donchian
 *                 levels, swing structure, chart patterns, correlation — needs
 *                 bars, and bars are per-symbol. 220 requests at concurrency
 *                 16 measured about 5s against the live venue.
 *
 * NUMERIC TYPES ARE INCONSISTENT AND THAT IS THE VENUE'S DOING, not a bug
 * here. In one ticker row `close`, `open`, `high`, `low`, `volume` and
 * `turnover_usd` arrive as JSON numbers while `mark_price`, `spot_price`,
 * `funding_rate`, `oi`, `tick_size` and `contract_value` arrive as quoted
 * strings. Every read goes through `num()`, which accepts both and returns
 * null for anything unusable — never 0. A missing price silently becoming 0 is
 * how a screener reports a token as down 100%.
 */

const DEFAULT_BASE = "https://api.india.delta.exchange";

function baseUrl(): string {
  const env = process.env.DELTA_API_BASE_URL?.trim();
  return (env && env.length > 0 ? env : DEFAULT_BASE).replace(/\/+$/, "");
}

/**
 * Tolerant numeric read. Returns null rather than 0 for anything unusable, so
 * "the venue did not report this" stays distinguishable from "the venue
 * reported zero" — for open interest and funding those are entirely different
 * market states.
 */
export function num(v: unknown): number | null {
  if (typeof v === "number") return Number.isFinite(v) ? v : null;
  if (typeof v === "string") {
    const t = v.trim();
    if (t === "") return null;
    const n = Number(t);
    return Number.isFinite(n) ? n : null;
  }
  return null;
}

/** One daily candle. `ts` is the bar's OPEN time in UTC seconds. */
export type Bar = {
  ts: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

/** Top of book as the venue reports it. */
export type Quotes = {
  bestBid: number | null;
  bestAsk: number | null;
  bidSize: number | null;
  askSize: number | null;
  markIv: number | null;
};

/** One listed perpetual, normalised out of `/v2/tickers`. */
export type PerpTicker = {
  symbol: string;
  productId: number;
  description: string;
  underlying: string;
  /** Venue tag list — `layer_1`, `defi`, `meme`, `ai`, `xStock`, ... */
  tags: string[];
  /** `crypto` or `tradfi`. The xStock and metal contracts are tradfi. */
  topTag: string | null;
  tradingStatus: string;

  /** Last traded price. */
  close: number | null;
  /** 24h rolling open/high/low, as the venue computes them. */
  open24h: number | null;
  high24h: number | null;
  low24h: number | null;
  /**
   * The venue's own ROLLING 24h % change. Deliberately kept separate from the
   * candle-derived 1d return: one measures the last 24 hours from now, the
   * other measures from the 00:00 UTC close. They are different windows and
   * routinely disagree by a percent or more, so both are carried and labelled
   * rather than one being quietly used as the other.
   */
  ltpChange24hPct: number | null;

  markPrice: number | null;
  spotPrice: number | null;
  /** mark/spot - 1, as a fraction, straight from the venue. */
  markBasis: number | null;

  /** 24h volume in CONTRACTS, and 24h turnover in USD. */
  volume24h: number | null;
  turnoverUsd24h: number | null;

  /**
   * Funding rate in PERCENT per 8-hour funding interval.
   *
   * The unit is argued rather than assumed, because getting it wrong is a 100x
   * error in a column a reader has no way to check. BTCUSD reports
   * funding_rate "0.01" alongside mark_basis 0.00001944 — a mark trading
   * 0.0019% over spot. Perp funding tracks basis; a funding rate of 1% per 8h
   * (the reading if this were a fraction) against a 0.0019% basis would be an
   * arbitrage the size of the contract. 0.01 PERCENT per 8h is the only
   * reading consistent with the basis printed beside it, and it annualises to
   * about 10.9%/yr, which is where BTC perp funding actually sits.
   */
  fundingRatePct8h: number | null;

  /** Open interest in underlying units, in contracts, and in USD. */
  oi: number | null;
  oiContracts: number | null;
  oiValueUsd: number | null;
  /** Change in OI notional over the last 6 hours, in USD. Signed. */
  oiChangeUsd6h: number | null;

  quotes: Quotes;

  /** Minimum price increment. Decides whether a stop can be expressed at all. */
  tickSize: number | null;
  /** Underlying quantity per contract: 1 BTCUSD = 0.001 BTC, 1 ADAUSD = 1 ADA. */
  contractValue: number | null;
  maxLeverage: number | null;
  priceBandLower: number | null;
  priceBandUpper: number | null;

  fetchedAt: number;
};

type RawTicker = Record<string, unknown>;

function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}

function normaliseTicker(raw: RawTicker, fetchedAt: number): PerpTicker | null {
  const symbol = str(raw.symbol).toUpperCase();
  const productId = num(raw.product_id);
  // A row without a symbol or a product id cannot be identified or priced.
  // Dropped rather than admitted with a placeholder.
  if (!symbol || productId === null || productId <= 0) return null;

  const q = (raw.quotes ?? {}) as Record<string, unknown>;
  const band = (raw.price_band ?? {}) as Record<string, unknown>;
  const tags = Array.isArray(raw.tags)
    ? raw.tags.filter((t): t is string => typeof t === "string")
    : [];

  return {
    symbol,
    productId,
    description: str(raw.description),
    underlying: str(raw.underlying_asset_symbol).toUpperCase(),
    tags,
    topTag: str(raw.top_tag) || null,
    tradingStatus: str(raw.product_trading_status) || "unknown",

    close: num(raw.close),
    open24h: num(raw.open),
    high24h: num(raw.high),
    low24h: num(raw.low),
    ltpChange24hPct: num(raw.ltp_change_24h),

    markPrice: num(raw.mark_price),
    spotPrice: num(raw.spot_price),
    markBasis: num(raw.mark_basis),

    volume24h: num(raw.volume),
    turnoverUsd24h: num(raw.turnover_usd),

    fundingRatePct8h: num(raw.funding_rate),

    oi: num(raw.oi),
    oiContracts: num(raw.oi_contracts),
    oiValueUsd: num(raw.oi_value_usd),
    oiChangeUsd6h: num(raw.oi_change_usd_6h),

    quotes: {
      bestBid: num(q.best_bid),
      bestAsk: num(q.best_ask),
      bidSize: num(q.bid_size),
      askSize: num(q.ask_size),
      markIv: num(q.mark_iv),
    },

    tickSize: num(raw.tick_size),
    contractValue: num(raw.contract_value),
    maxLeverage: num(raw.leverage),
    priceBandLower: num(band.lower_limit),
    priceBandUpper: num(band.upper_limit),

    fetchedAt,
  };
}

export class DeltaFeedError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "DeltaFeedError";
  }
}

async function getJson<T>(path: string, timeoutMs: number): Promise<T> {
  const res = await fetch(`${baseUrl()}${path}`, {
    cache: "no-store",
    headers: { accept: "application/json" },
    signal: AbortSignal.timeout(timeoutMs),
  });
  if (!res.ok) throw new DeltaFeedError(`delta ${path} returned HTTP ${res.status}`);
  return (await res.json()) as T;
}

type TickersResponse = { success?: boolean; result?: RawTicker[] };

/** Every listed perpetual. One request for the whole universe. */
export async function fetchPerpTickers(): Promise<PerpTicker[]> {
  const body = await getJson<TickersResponse>(
    "/v2/tickers?contract_types=perpetual_futures",
    25_000,
  );
  if (body.success === false || !Array.isArray(body.result)) {
    throw new DeltaFeedError("delta /v2/tickers returned no usable result array");
  }
  const now = Date.now();
  const out: PerpTicker[] = [];
  for (const raw of body.result) {
    const t = normaliseTicker(raw, now);
    // One malformed row costs one symbol, never the scan.
    if (t) out.push(t);
  }
  if (out.length === 0) throw new DeltaFeedError("delta returned no usable perpetual tickers");
  return out;
}

type CandlesResponse = { success?: boolean; result?: Array<Record<string, unknown>> };

/**
 * Daily bars for one symbol, OLDEST FIRST.
 *
 * The venue returns newest-first. Every piece of bar maths downstream — moving
 * averages, ATR, Donchian windows, pattern pivots — assumes chronological
 * order, and silently reading a reversed series produces numbers that are
 * confidently wrong rather than obviously broken, so the sort happens here,
 * once, at the boundary.
 *
 * DAY BOUNDARIES ARE 00:00 UTC. Crypto has no session and no holiday, so a
 * calendar day is the natural bar; but the app displays IST, where a UTC day
 * runs 05:30 to 05:30. "Today's" bar is therefore the UTC day and not the IST
 * one, and every horizon in this module measures in UTC days for that reason.
 */
export async function fetchDailyBars(symbol: string, days: number): Promise<Bar[]> {
  return fetchBars(symbol, "1d", Math.max(2, days) * 86_400, 86_400);
}

/**
 * Hourly bars for one symbol, oldest first.
 *
 * Fetched for exactly one reason: the venue reports open-interest change over
 * SIX HOURS, and classifying that against a 24-hour price change would pair two
 * different windows into one claim. Six hourly closes give the aligned price
 * move, so the buildup quadrant describes a single window or reports itself as
 * unclassified.
 */
export async function fetchHourlyBars(symbol: string, hours: number): Promise<Bar[]> {
  return fetchBars(symbol, "1h", Math.max(2, hours) * 3_600, 3_600);
}

/**
 * Bars between two explicit instants, at any resolution the venue offers
 * (`1m`, `5m`, `15m`, `1h`, `4h`, `1d`).
 *
 * The paper desk manages open positions by REPLAYING the bars that elapsed
 * since it last looked, rather than by comparing the current price to a stop.
 * That needs a window with both ends pinned — "the last 48 hours" is the wrong
 * question when a position opened five days ago.
 *
 * `from` and `to` are unix seconds. A window is padded by one bucket on each
 * side so a stop touched in the bar straddling `from` is not missed.
 */
export async function fetchBarsBetween(
  symbol: string,
  resolution: string,
  from: number,
  to: number,
): Promise<Bar[]> {
  const bucket = RESOLUTION_SECONDS[resolution] ?? 900;
  const start = Math.max(0, Math.floor(from) - bucket);
  const end = Math.max(start + bucket, Math.ceil(to) + bucket);
  return fetchBarsRange(symbol, resolution, start, end, bucket);
}

export const RESOLUTION_SECONDS: Record<string, number> = {
  "1m": 60,
  "5m": 300,
  "15m": 900,
  "1h": 3_600,
  "4h": 14_400,
  "1d": 86_400,
};

async function fetchBars(
  symbol: string,
  resolution: string,
  spanSeconds: number,
  bucketSeconds: number,
): Promise<Bar[]> {
  const end = Math.floor(Date.now() / 1000);
  return fetchBarsRange(symbol, resolution, end - spanSeconds, end, bucketSeconds);
}

async function fetchBarsRange(
  symbol: string,
  resolution: string,
  start: number,
  end: number,
  bucketSeconds: number,
): Promise<Bar[]> {
  const path =
    `/v2/history/candles?resolution=${resolution}&symbol=${encodeURIComponent(symbol)}` +
    `&start=${start}&end=${end}`;
  const body = await getJson<CandlesResponse>(path, 20_000);
  if (!Array.isArray(body.result)) return [];

  const bars: Bar[] = [];
  for (const r of body.result) {
    const ts = num(r.time);
    const o = num(r.open);
    const h = num(r.high);
    const l = num(r.low);
    const c = num(r.close);
    if (ts === null || o === null || h === null || l === null || c === null) continue;
    if (o <= 0 || h <= 0 || l <= 0 || c <= 0) continue;
    bars.push({ ts, open: o, high: h, low: l, close: c, volume: num(r.volume) ?? 0 });
  }

  // Deduplicate by bucket, keeping the last copy — a request that straddles a
  // bar close can return the same period twice, and the later copy is the more
  // complete one. The Map also gives the chronological sort for free.
  const byBucket = new Map<number, Bar>();
  for (const b of bars.sort((a, b) => a.ts - b.ts)) {
    byBucket.set(Math.floor(b.ts / bucketSeconds), b);
  }
  return [...byBucket.entries()].sort((a, b) => a[0] - b[0]).map(([, b]) => b);
}

/**
 * Daily bars for many symbols, with a bounded number of requests in flight.
 *
 * Bounded rather than unbounded: firing 220 simultaneous fetches from one
 * serverless invocation is how a shared upstream starts refusing the whole
 * batch, and that failure would read as an empty market rather than as a rate
 * limit. A symbol whose fetch fails is simply absent from the map and reports
 * as "no history" everywhere downstream, which is what it is.
 */
export async function fetchBarsMany(
  symbols: string[],
  fetchOne: (symbol: string) => Promise<Bar[]>,
  concurrency = 16,
): Promise<{ bars: Map<string, Bar[]>; failed: string[] }> {
  const bars = new Map<string, Bar[]>();
  const failed: string[] = [];
  let cursor = 0;

  async function worker(): Promise<void> {
    for (;;) {
      const i = cursor++;
      if (i >= symbols.length) return;
      const sym = symbols[i]!;
      try {
        const b = await fetchOne(sym);
        if (b.length > 0) bars.set(sym, b);
        else failed.push(sym);
      } catch {
        failed.push(sym);
      }
    }
  }

  await Promise.all(Array.from({ length: Math.min(concurrency, symbols.length) }, worker));
  return { bars, failed };
}

export function fetchDailyBarsMany(
  symbols: string[],
  days: number,
  concurrency = 16,
): Promise<{ bars: Map<string, Bar[]>; failed: string[] }> {
  return fetchBarsMany(symbols, (s) => fetchDailyBars(s, days), concurrency);
}

export function fetchHourlyBarsMany(
  symbols: string[],
  hours: number,
  concurrency = 16,
): Promise<{ bars: Map<string, Bar[]>; failed: string[] }> {
  return fetchBarsMany(symbols, (s) => fetchHourlyBars(s, hours), concurrency);
}

// ── product specs ───────────────────────────────────────────────────────────

/**
 * Margin specification for one contract, from `/v2/products`.
 *
 * WHY A SECOND ENDPOINT. `/v2/tickers` does NOT carry `maintenance_margin` —
 * measured on the live venue, it is absent for all 220 perpetuals. `/v2/products`
 * carries it for all 220, along with the initial margin the venue's maximum
 * leverage is derived from.
 *
 * This matters because a paper desk that sizes with leverage has to know where
 * the venue would liquidate the position. Assuming a maintenance margin is the
 * precise shape of a failure this codebase has already had: an assumed number
 * put the liquidation price INSIDE the strategy's own stop, so the venue closed
 * trades the strategy believed it was still managing. The values here span
 * 0.25% to 2.5% — a tenfold range — so no single assumption is safe.
 */
export type ProductSpec = {
  symbol: string;
  /** Maintenance margin as a PERCENT of notional. 0.25 to 2.5 on this venue. */
  maintenanceMarginPct: number | null;
  /** Initial margin as a PERCENT of notional. Max leverage is 100 / this. */
  initialMarginPct: number | null;
  positionSizeLimit: number | null;
  tradingStatus: string;
};

type ProductsResponse = { success?: boolean; result?: Array<Record<string, unknown>> };

/** Margin specs for every listed perpetual, keyed by symbol. One request. */
export async function fetchProductSpecs(): Promise<Map<string, ProductSpec>> {
  const body = await getJson<ProductsResponse>(
    "/v2/products?contract_types=perpetual_futures&page_size=500",
    25_000,
  );
  const out = new Map<string, ProductSpec>();
  if (!Array.isArray(body.result)) return out;
  for (const p of body.result) {
    const symbol = str(p.symbol).toUpperCase();
    if (!symbol) continue;
    out.set(symbol, {
      symbol,
      maintenanceMarginPct: num(p.maintenance_margin),
      initialMarginPct: num(p.initial_margin),
      positionSizeLimit: num(p.position_size_limit),
      tradingStatus: str(p.trading_status) || "unknown",
    });
  }
  return out;
}
