/**
 * Forex, metals, indices and energies as a paper venue — an Exness-style desk.
 *
 * ════════════════════════════════════════════════════════════════════════════
 * WHAT IS REAL HERE, AND WHAT IS MODELLED
 * ════════════════════════════════════════════════════════════════════════════
 *
 * This is the honesty boundary of the whole module and it is drawn explicitly
 * rather than left for a reader to discover.
 *
 * REAL — every price. Mid prices and OHLC candles come from Yahoo Finance,
 * which publishes live quotes and intraday bars for FX majors and crosses
 * (`EURUSD=X`), metals and energies (`GC=F`, `SI=F`, `CL=F`), indices
 * (`^GSPC`) and crypto (`BTC-USD`). Verified against all of them. A chart on
 * this desk is the real market.
 *
 * MODELLED — the spread, the swap, and the commission. No free feed publishes
 * a retail broker's bid and ask, and no feed at all publishes ONE broker's
 * overnight financing. Those three come from per-instrument tables below,
 * scaled by account type the way a real broker's tiers work. Every instrument
 * this venue returns therefore carries `spreadIsModelled: true`, and the
 * terminal says so on the ticket rather than in a footnote.
 *
 * That distinction matters because the spread is not a cosmetic detail — on a
 * scalping strategy it IS the strategy's cost base, and a desk that quoted an
 * invented spread as though a broker had published it would produce a P&L that
 * looks precise and is not. The prices are real; the cost of crossing them is
 * a model, and it is labelled as one everywhere it appears.
 *
 * ════════════════════════════════════════════════════════════════════════════
 * WHAT MAKES A FOREX DESK DIFFERENT FROM A CRYPTO ONE
 * ════════════════════════════════════════════════════════════════════════════
 *
 * LOTS AND PIPS. Size is lots, not contracts: one standard lot is 100,000
 * units of the base currency, and brokers accept 0.01 (a micro lot). A pip is
 * the fourth decimal on most pairs and the SECOND on yen pairs, which is the
 * classic off-by-100 in every homemade forex calculator. `pipSize` is declared
 * per instrument for exactly that reason.
 *
 * MARGIN LEVEL AND STOP OUT. Crypto liquidates a position against its own
 * maintenance margin. A forex account is managed on ACCOUNT-WIDE margin level
 * — equity divided by used margin — with a margin call at one threshold and a
 * stop out at a lower one, where the broker starts closing the worst position
 * until the level recovers. That is a different mechanism, not a different
 * number, and the engine implements both.
 *
 * SWAP, NOT FUNDING. Charged once at 21:00 UTC rollover, and TRIPLED on
 * Wednesday to carry value date over the weekend. Long and short legs have
 * separate rates and either can be positive.
 *
 * THE MARKET CLOSES. FX trades 24/5, shutting Friday evening and reopening
 * Sunday evening. A terminal that accepted orders at 03:00 on a Saturday would
 * be filling against a stale Friday price; `marketOpen()` says whether the
 * venue is live, and the engine refuses market orders when it is not.
 */

import type { AccountType, Candle, Instrument, OrderBook, VenueAdapter } from "../types";

/** Yahoo symbol, display name, and everything a broker would publish per instrument. */
type Spec = {
  symbol: string;
  yahoo: string;
  name: string;
  kind: Instrument["kind"];
  /** Units of the base per 1.00 lot. */
  contractSize: number;
  /** One pip, in price terms. The second decimal on yen pairs, fourth elsewhere. */
  pipSize: number;
  pricePrecision: number;
  /** Typical spread in PIPS on a Standard account. */
  spreadPips: number;
  /** Swap in points per day per lot, long and short. Points, not pips. */
  swapLong: number;
  swapShort: number;
  maxLeverage: number;
};

/**
 * The instrument table.
 *
 * Spreads are typical Standard-account figures, not a live quote. Swaps are
 * indicative. Both are stated here in one place so they can be corrected
 * against a real statement rather than being scattered through the engine.
 */
const SPECS: Spec[] = [
  // ── FX majors ─────────────────────────────────────────────────────────────
  { symbol: "EURUSD", yahoo: "EURUSD=X", name: "Euro / US Dollar", kind: "fx", contractSize: 100_000, pipSize: 0.0001, pricePrecision: 5, spreadPips: 0.8, swapLong: -7.2, swapShort: 2.1, maxLeverage: 2000 },
  { symbol: "GBPUSD", yahoo: "GBPUSD=X", name: "British Pound / US Dollar", kind: "fx", contractSize: 100_000, pipSize: 0.0001, pricePrecision: 5, spreadPips: 1.1, swapLong: -6.4, swapShort: 1.2, maxLeverage: 2000 },
  { symbol: "USDJPY", yahoo: "USDJPY=X", name: "US Dollar / Japanese Yen", kind: "fx", contractSize: 100_000, pipSize: 0.01, pricePrecision: 3, spreadPips: 0.9, swapLong: 4.8, swapShort: -12.6, maxLeverage: 2000 },
  { symbol: "USDCHF", yahoo: "USDCHF=X", name: "US Dollar / Swiss Franc", kind: "fx", contractSize: 100_000, pipSize: 0.0001, pricePrecision: 5, spreadPips: 1.2, swapLong: 3.9, swapShort: -9.8, maxLeverage: 2000 },
  { symbol: "AUDUSD", yahoo: "AUDUSD=X", name: "Australian Dollar / US Dollar", kind: "fx", contractSize: 100_000, pipSize: 0.0001, pricePrecision: 5, spreadPips: 1.0, swapLong: -5.1, swapShort: 0.8, maxLeverage: 2000 },
  { symbol: "USDCAD", yahoo: "USDCAD=X", name: "US Dollar / Canadian Dollar", kind: "fx", contractSize: 100_000, pipSize: 0.0001, pricePrecision: 5, spreadPips: 1.3, swapLong: 1.4, swapShort: -6.9, maxLeverage: 2000 },
  { symbol: "NZDUSD", yahoo: "NZDUSD=X", name: "New Zealand Dollar / US Dollar", kind: "fx", contractSize: 100_000, pipSize: 0.0001, pricePrecision: 5, spreadPips: 1.5, swapLong: -4.6, swapShort: 0.5, maxLeverage: 2000 },
  // ── FX crosses ────────────────────────────────────────────────────────────
  { symbol: "EURGBP", yahoo: "EURGBP=X", name: "Euro / British Pound", kind: "fx", contractSize: 100_000, pipSize: 0.0001, pricePrecision: 5, spreadPips: 1.4, swapLong: -3.2, swapShort: -0.9, maxLeverage: 2000 },
  { symbol: "EURJPY", yahoo: "EURJPY=X", name: "Euro / Japanese Yen", kind: "fx", contractSize: 100_000, pipSize: 0.01, pricePrecision: 3, spreadPips: 1.6, swapLong: -2.1, swapShort: -6.4, maxLeverage: 2000 },
  { symbol: "GBPJPY", yahoo: "GBPJPY=X", name: "British Pound / Japanese Yen", kind: "fx", contractSize: 100_000, pipSize: 0.01, pricePrecision: 3, spreadPips: 2.2, swapLong: -1.4, swapShort: -8.2, maxLeverage: 2000 },
  { symbol: "AUDJPY", yahoo: "AUDJPY=X", name: "Australian Dollar / Japanese Yen", kind: "fx", contractSize: 100_000, pipSize: 0.01, pricePrecision: 3, spreadPips: 2.0, swapLong: 1.1, swapShort: -7.8, maxLeverage: 2000 },
  { symbol: "EURAUD", yahoo: "EURAUD=X", name: "Euro / Australian Dollar", kind: "fx", contractSize: 100_000, pipSize: 0.0001, pricePrecision: 5, spreadPips: 2.4, swapLong: -2.8, swapShort: -1.6, maxLeverage: 2000 },
  // ── metals ────────────────────────────────────────────────────────────────
  { symbol: "XAUUSD", yahoo: "GC=F", name: "Gold / US Dollar", kind: "metal", contractSize: 100, pipSize: 0.01, pricePrecision: 2, spreadPips: 18, swapLong: -14.2, swapShort: 4.8, maxLeverage: 1000 },
  { symbol: "XAGUSD", yahoo: "SI=F", name: "Silver / US Dollar", kind: "metal", contractSize: 5_000, pipSize: 0.001, pricePrecision: 3, spreadPips: 22, swapLong: -11.6, swapShort: 3.1, maxLeverage: 1000 },
  // ── energies ──────────────────────────────────────────────────────────────
  { symbol: "USOIL", yahoo: "CL=F", name: "WTI Crude Oil", kind: "commodity", contractSize: 1_000, pipSize: 0.01, pricePrecision: 2, spreadPips: 3, swapLong: -8.4, swapShort: 1.9, maxLeverage: 200 },
  // ── indices ───────────────────────────────────────────────────────────────
  { symbol: "US500", yahoo: "^GSPC", name: "S&P 500 Index", kind: "index", contractSize: 10, pipSize: 0.1, pricePrecision: 2, spreadPips: 4, swapLong: -12.8, swapShort: 2.4, maxLeverage: 200 },
  { symbol: "USTEC", yahoo: "^NDX", name: "Nasdaq 100 Index", kind: "index", contractSize: 5, pipSize: 0.1, pricePrecision: 2, spreadPips: 12, swapLong: -15.2, swapShort: 3.1, maxLeverage: 200 },
  // ── crypto, as a broker lists it ──────────────────────────────────────────
  // A BTC CFD is quoted in WHOLE DOLLARS. Carrying the FX convention of a
  // hundredth-of-a-unit pip made the ticket read "25000.0 pts" of spread on a
  // $77,000 instrument, which looks like a broken number rather than the $25 it
  // actually is.
  { symbol: "BTCUSD.fx", yahoo: "BTC-USD", name: "Bitcoin / US Dollar (CFD)", kind: "crypto", contractSize: 1, pipSize: 1, pricePrecision: 2, spreadPips: 25, swapLong: -30, swapShort: -30, maxLeverage: 400 },
];

/**
 * Account tiers, and what each one actually changes.
 *
 * Modelled on how retail brokers tier execution: the cheap-looking account
 * hides its cost in the spread, and the "raw" one shows a tight spread and
 * charges commission instead. The total is usually within a rounding error of
 * the other, which is the point worth being able to see on a paper desk.
 */
const ACCOUNT_TYPES: { key: AccountType; label: string; note: string }[] = [
  { key: "standard", label: "Standard", note: "No commission. Cost is entirely in the spread." },
  { key: "raw", label: "Raw Spread", note: "Near-zero spread plus $3.50 per lot per side." },
  { key: "zero", label: "Zero", note: "Zero spread on majors plus $2.00 per lot per side." },
  { key: "pro", label: "Pro", note: "No commission, tighter spread than Standard." },
];

/** Spread multiplier and per-lot commission by account tier. */
const TIERS: Record<AccountType, { spreadX: number; commissionPerLot: number }> = {
  standard: { spreadX: 1, commissionPerLot: 0 },
  pro: { spreadX: 0.6, commissionPerLot: 0 },
  raw: { spreadX: 0.12, commissionPerLot: 3.5 },
  zero: { spreadX: 0.02, commissionPerLot: 2.0 },
};

export function tierFor(accountType: AccountType) {
  return TIERS[accountType] ?? TIERS.standard;
}

const TTL_MS = 15_000;
const quoteCache = new Map<string, { at: number; q: YahooQuote }>();

type YahooQuote = {
  price: number;
  previousClose: number | null;
  high: number | null;
  low: number | null;
  candles: Candle[];
};

type YahooChart = {
  chart?: {
    result?: Array<{
      meta?: Record<string, unknown>;
      timestamp?: number[];
      indicators?: { quote?: Array<Record<string, (number | null)[]>> };
    }>;
    error?: unknown;
  };
};

const YAHOO = "https://query1.finance.yahoo.com/v8/finance/chart";

async function yahoo(yahooSymbol: string, range: string, interval: string): Promise<YahooQuote | null> {
  const url = `${YAHOO}/${encodeURIComponent(yahooSymbol)}?range=${range}&interval=${interval}`;
  try {
    const res = await fetch(url, {
      cache: "no-store",
      // Yahoo refuses a request with no user agent.
      headers: { "user-agent": "Mozilla/5.0", accept: "application/json" },
      signal: AbortSignal.timeout(15_000),
    });
    if (!res.ok) return null;
    const body = (await res.json()) as YahooChart;
    const r = body.chart?.result?.[0];
    if (!r) return null;
    const meta = r.meta ?? {};
    const price = numOf(meta.regularMarketPrice);
    if (price === null || price <= 0) return null;

    const ts = r.timestamp ?? [];
    const q = r.indicators?.quote?.[0] ?? {};
    const candles: Candle[] = [];
    for (let i = 0; i < ts.length; i++) {
      const o = q.open?.[i];
      const h = q.high?.[i];
      const l = q.low?.[i];
      const c = q.close?.[i];
      // Yahoo pads the series with nulls for periods it has no print for —
      // the forming bar, and every minute a closed market did not trade.
      // Carrying those through would draw gaps in the chart and, worse, let
      // the matcher "fill" an order against a bar that never existed.
      if (o == null || h == null || l == null || c == null) continue;
      candles.push({ time: ts[i]!, open: o, high: h, low: l, close: c, volume: q.volume?.[i] ?? 0 });
    }

    return {
      price,
      previousClose: numOf(meta.previousClose),
      high: numOf(meta.regularMarketDayHigh),
      low: numOf(meta.regularMarketDayLow),
      candles,
    };
  } catch {
    return null;
  }
}

function numOf(v: unknown): number | null {
  return typeof v === "number" && Number.isFinite(v) ? v : null;
}

async function quoteFor(spec: Spec): Promise<YahooQuote | null> {
  const hit = quoteCache.get(spec.symbol);
  if (hit && Date.now() - hit.at < TTL_MS) return hit.q;
  const q = await yahoo(spec.yahoo, "1d", "5m");
  if (q) quoteCache.set(spec.symbol, { at: Date.now(), q });
  return q;
}

/**
 * Is the forex market open right now?
 *
 * FX runs 24/5: it opens Sunday 21:00 UTC and closes Friday 21:00 UTC. Outside
 * that the last print is stale, and a terminal that filled a market order
 * against it would be inventing a trade at a price nobody was quoting. The
 * engine refuses market orders when this is false rather than filling anyway.
 */
export function marketOpen(at: Date = new Date()): boolean {
  const day = at.getUTCDay();
  const hour = at.getUTCHours();
  if (day === 6) return false; // Saturday
  if (day === 0) return hour >= 21; // Sunday, from the open
  if (day === 5) return hour < 21; // Friday, until the close
  return true;
}

export function nextOpenNote(at: Date = new Date()): string {
  if (marketOpen(at)) return "";
  return (
    "The forex market is closed — it trades 24/5, opening Sunday 21:00 UTC and closing Friday " +
    "21:00 UTC. Prices shown are the last print before the close, and market orders are refused " +
    "rather than filled against a stale quote."
  );
}

function toInstrument(spec: Spec, q: YahooQuote, accountType: AccountType): Instrument {
  const tier = tierFor(accountType);
  // The modelled spread, in price terms. Never below one tick — a "zero
  // spread" account still cannot trade inside the price grid.
  const spread = Math.max(spec.pipSize * spec.spreadPips * tier.spreadX, spec.pipSize * 0.1);
  const mid = q.price;
  const change =
    q.previousClose && q.previousClose > 0 ? (mid / q.previousClose - 1) * 100 : null;

  return {
    venue: "forex",
    symbol: spec.symbol,
    displayName: spec.name,
    kind: spec.kind,
    contractSize: spec.contractSize,
    sizeUnit: "lots",
    minSize: 0.01,
    sizeStep: 0.01,
    tickSize: spec.pipSize / 10, // brokers quote a fractional pip
    pipSize: spec.pipSize,
    pricePrecision: spec.pricePrecision,
    maxLeverage: spec.maxLeverage,
    // Retail forex margin is set by leverage, not by a separate maintenance
    // tier: the account is managed on margin level and stopped out there.
    maintenanceMarginPct: 0,

    takerFeeRate: 0,
    makerFeeRate: 0,
    commissionPerLotUsd: tier.commissionPerLot,

    carryKind: "swap",
    fundingRatePct8h: null,
    swapLongPointsPerDay: spec.swapLong,
    swapShortPointsPerDay: spec.swapShort,

    last: mid,
    bid: mid - spread / 2,
    ask: mid + spread / 2,
    markPrice: mid,
    change24hPct: change,
    high24h: q.high,
    low24h: q.low,
    quoteCurrency: "USD",

    spreadIsModelled: true,
    source: "Yahoo Finance (real mid and candles) — spread, swap and commission are MODELLED",
  };
}

/**
 * Find a spec regardless of how the caller cased the symbol.
 *
 * Symbols on this desk are NOT all uppercase — `BTCUSD.fx` distinguishes the
 * CFD from the Delta perpetual of the same name. The API normalises inbound
 * symbols to uppercase, so an exact match silently failed for that one
 * instrument: its chart, book and tape all came back empty and an order on it
 * was rejected as "not listed", while the ticker kept quoting a price because
 * listing does not take a symbol. Matching case-insensitively removes the whole
 * class of failure rather than renaming the one symbol that tripped it.
 */
function findSpec(symbol: string): Spec | undefined {
  const needle = symbol.trim().toLowerCase();
  return SPECS.find((s) => s.symbol.toLowerCase() === needle);
}

/** The canonical spelling of a symbol on this venue, or null if unknown. */
export function canonicalSymbol(symbol: string): string | null {
  return findSpec(symbol)?.symbol ?? null;
}

/** The pip size for an instrument, which the P&L display needs. */
export function pipSizeOf(symbol: string): number {
  return findSpec(symbol)?.pipSize ?? 0.0001;
}

export function specFor(symbol: string): Spec | undefined {
  return findSpec(symbol);
}

async function listInstruments(accountType: AccountType = "standard"): Promise<Instrument[]> {
  const out: Instrument[] = [];
  // Bounded concurrency: one request per instrument and Yahoo is a shared,
  // unauthenticated endpoint. An instrument whose quote fails is DROPPED
  // rather than listed at a stale or zero price.
  let cursor = 0;
  const workers = Array.from({ length: Math.min(6, SPECS.length) }, async () => {
    for (;;) {
      const i = cursor++;
      if (i >= SPECS.length) return;
      const spec = SPECS[i]!;
      const q = await quoteFor(spec);
      if (q) out.push(toInstrument(spec, q, accountType));
    }
  });
  await Promise.all(workers);
  const order = new Map(SPECS.map((s, i) => [s.symbol, i]));
  out.sort((a, b) => (order.get(a.symbol) ?? 0) - (order.get(b.symbol) ?? 0));
  return out;
}

export const RESOLUTIONS = [
  { key: "1m", label: "1m", seconds: 60 },
  { key: "5m", label: "5m", seconds: 300 },
  { key: "15m", label: "15m", seconds: 900 },
  { key: "1h", label: "1H", seconds: 3_600 },
  { key: "1d", label: "1D", seconds: 86_400 },
];

/** Yahoo pairs each interval with a maximum range it will serve. */
const RANGE_FOR: Record<string, string> = {
  "1m": "1d",
  "5m": "5d",
  "15m": "1mo",
  "1h": "3mo",
  "1d": "2y",
};
/**
 * A wider range to retry with when the first request comes back empty.
 *
 * Needed because the range that serves an interval is not the same for every
 * instrument class. `range=1d&interval=1m` returns the last trading day for an
 * FX pair, and NOTHING for the futures-backed instruments (gold, silver, oil)
 * once their session has closed — Yahoo has the bars, it just will not scope
 * them to a calendar day with no trading in it. `range=5d` returns 5,158 of
 * them for the same contract.
 *
 * Widening rather than special-casing three symbols: the same shape will bite
 * the next instrument class added here, and a retry costs one request on a path
 * that was returning an empty chart anyway. 1-minute data is capped at seven
 * days upstream, which is why its fallback stops at 5d rather than a month.
 */
const FALLBACK_RANGE: Record<string, string> = {
  "1m": "5d",
  "5m": "1mo",
  "15m": "3mo",
  "1h": "1y",
  "1d": "5y",
};

const YAHOO_INTERVAL: Record<string, string> = {
  "1m": "1m",
  "5m": "5m",
  "15m": "15m",
  "1h": "1h",
  "1d": "1d",
};

export const forexVenue: VenueAdapter = {
  id: "forex",
  label: "Forex Paper Trading",
  dataNote:
    "Real mid prices and candles from Yahoo Finance for FX majors and crosses, metals, energies, " +
    "indices and crypto CFDs. The SPREAD, SWAP and COMMISSION are modelled per instrument and per " +
    "account tier — no free feed publishes a retail broker's bid and ask — and every quote on this " +
    "desk is flagged as such. Paper money only.",
  // MetaTrader-style hedging: a long and a short on the same pair coexist as
  // separate tickets, which is how the platform this clones behaves.
  positionMode: "hedging",
  sizeUnit: "lots",
  defaultStartingBalance: 10_000,
  defaultLeverage: 500,
  leverageChoices: [50, 100, 200, 500, 1000, 2000],
  accountTypes: ACCOUNT_TYPES,
  // Margin call at 60%, stop out at 30% — the account is managed on margin
  // level rather than on any one position's maintenance margin.
  stopOutLevelPct: 30,
  marginCallLevelPct: 60,
  resolutions: RESOLUTIONS,

  listInstruments: () => listInstruments("standard"),

  async getInstrument(symbol: string) {
    const spec = findSpec(symbol);
    if (!spec) return null;
    const q = await quoteFor(spec);
    return q ? toInstrument(spec, q, "standard") : null;
  },

  /**
   * A book synthesised from the mid and the modelled spread.
   *
   * Marked `modelled: true` so the terminal renders it differently from Delta's
   * real depth. It exists because the order ticket needs a bid and an ask, NOT
   * to imply that this is anyone's resting liquidity — the sizes are a decaying
   * ladder, and the UI says so.
   */
  async getBook(symbol: string, depth: number): Promise<OrderBook | null> {
    const spec = findSpec(symbol);
    if (!spec) return null;
    const q = await quoteFor(spec);
    if (!q) return null;
    const inst = toInstrument(spec, q, "standard");
    const step = spec.pipSize;
    const bids = Array.from({ length: depth }, (_, i) => ({
      price: inst.bid - i * step,
      size: Math.round((1 + i * 0.6) * 100) / 100,
    }));
    const asks = Array.from({ length: depth }, (_, i) => ({
      price: inst.ask + i * step,
      size: Math.round((1 + i * 0.6) * 100) / 100,
    }));
    return { symbol: spec.symbol, bids, asks, asOf: Date.now(), modelled: true };
  },

  /**
   * Bars for the chart.
   *
   * THE WINDOW IS A PREFERENCE, NOT A FILTER — because this market closes.
   *
   * The caller asks for "the last N bars" by turning N into a time range, which
   * is exactly right on a 24/7 venue and wrong here. On a Sunday morning the
   * newest FX print is Friday 21:00 UTC; a request for the last 25 hours of
   * 5-minute bars therefore matched NOTHING, and the chart rendered "the venue
   * returned no bars for this instrument and interval" while Yahoo was in fact
   * returning 1,422 perfectly good ones. The market being shut is not a data
   * failure, and it must not be reported as one.
   *
   * So the window is applied when it contains something, and otherwise the most
   * recent bars are returned instead. That is what a real terminal shows over a
   * weekend: the last session. The page already says the market is closed, and
   * `staleAfter` on the chart notes how old the newest bar is.
   */
  async getCandles(symbol: string, resolution: string, from: number, to: number): Promise<Candle[]> {
    const spec = findSpec(symbol);
    if (!spec) return [];
    const interval = YAHOO_INTERVAL[resolution] ?? "5m";
    let q = await yahoo(spec.yahoo, RANGE_FOR[resolution] ?? "5d", interval);
    if (!q || q.candles.length === 0) {
      const wider = FALLBACK_RANGE[resolution];
      if (wider) q = await yahoo(spec.yahoo, wider, interval);
    }
    if (!q || q.candles.length === 0) return [];

    const inWindow = q.candles.filter((c) => c.time >= from && c.time <= to);
    if (inWindow.length > 0) return inWindow;

    // Nothing traded in the requested window. Hand back the same NUMBER of bars
    // the caller asked for, taken from the end of what exists.
    const seconds = RESOLUTIONS.find((r) => r.key === resolution)?.seconds ?? 300;
    const wanted = Math.max(1, Math.round((to - from) / seconds));
    return q.candles.slice(-wanted);
  },
};

export { SPECS as FOREX_SPECS };

/**
 * A trade tape DERIVED FROM 1-MINUTE BARS, because there is no public one.
 *
 * No free forex feed publishes a print-by-print tape, so this reconstructs one
 * bar at a time: each closed minute becomes a single synthetic print at its
 * close, with the side taken from whether the bar closed up or down and the
 * size from its volume. That is enough to show the market breathing and to
 * give the terminal a recent-price column — and it is NOT a real tape, so it
 * is returned with `derived: true` and the UI labels it.
 *
 * The alternative was to leave the panel empty on this venue, which would have
 * been honest but less useful; inventing prints and calling them trades would
 * have been useful and dishonest. Labelling is the third option.
 */
export async function fetchDerivedTape(symbol: string, limit = 40): Promise<
  { price: number; size: number; side: "buy" | "sell"; at: number }[]
> {
  const spec = findSpec(symbol);
  if (!spec) return [];
  const q = await yahoo(spec.yahoo, "1d", "1m");
  if (!q) return [];
  const bars = q.candles.slice(-limit);
  return bars
    .map((c) => ({
      price: c.close,
      // Volume is absent on most FX series — Yahoo reports it only for futures
      // and crypto — so it falls back to a nominal size rather than rendering
      // every print as a zero.
      size: c.volume > 0 ? c.volume : 1,
      side: (c.close >= c.open ? "buy" : "sell") as "buy" | "sell",
      at: c.time,
    }))
    .reverse();
}
