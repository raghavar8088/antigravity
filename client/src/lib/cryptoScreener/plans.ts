/**
 * Trade plans — Scalp, Swing and Breakout — each priced NET OF REAL DELTA COSTS.
 *
 * WHY THE COSTS ARE NOT A FOOTNOTE. This codebase has already learned this the
 * expensive way on its equity side: when real broker charges were backfilled
 * onto a desk, a book showing +Rs23,500 became -Rs33,600 and 1,415 strategies
 * that had ranked as tournament winners turned out to be losers. Not one of
 * those numbers was wrong before the cutover — they were just gross. A screener
 * that ranked setups on gross reward-to-risk would reproduce that error one
 * screen earlier.
 *
 * A PERPETUAL HAS A COST AN EQUITY DOES NOT: FUNDING. Every eight hours the
 * crowded side pays the other. A swing plan that sleeps for five days crosses
 * fifteen funding settlements, and on a contract paying 0.08% per interval that
 * is 1.2% of notional — larger than both taker fees combined and larger than
 * many of the targets on this board. Charging only the fees would understate
 * the cost of exactly the holds this page recommends, so every plan here is
 * charged fees AND the funding it would actually pay over its own horizon.
 *
 * Funding is charged in WHOLE INTERVALS, rounded up. It settles at the stamp
 * and not pro-rata: a position open for one hour across a settlement pays a
 * full interval, and a plan that assumed one eighth of one would flatter the
 * short holds most.
 *
 * THE THREE MODES ARE DIFFERENT QUESTIONS, NOT THREE RISK SETTINGS.
 *
 *   Scalp     hours. Stop from ATR, target 2R. No square-off — this market
 *             never closes, so the exit is a time stop and not a bell.
 *   Swing     days. Stop under the last CONFIRMED swing low, target 2.5R.
 *   Breakout  a level being taken out. Entry AT the level, stop under the base,
 *             target the measured move.
 *
 * BREAKOUT REFUSES A GAPPED FILL. A break is only tradable within a small band
 * above the level. A contract that has already run far past its trigger is not
 * the trade the level described, and filling it because the level was crossed
 * would be the plan overruling the reason it existed. When that happens the
 * plan is returned with `tradable: false` and the distance attached, rather
 * than silently disappearing or silently filling.
 *
 * AND THE GRID GATE, which has no equity analogue at all. A stop that is fewer
 * than twenty ticks wide cannot survive rounding to the venue's price grid: the
 * order that reaches Delta is materially not the order the plan described. This
 * is a property of the CONTRACT and does not improve when the market calms
 * down. Plans on such contracts are returned untradable with the tick count
 * shown, because the same contracts keep topping momentum boards and the reader
 * has no other way to find out.
 */

import { fundingCostPct, MIN_STOP_TICKS, PERP_TAKER_FEE_RATE } from "./derivatives";
import { round } from "./horizons";
import type { PatternHit } from "./patterns";
import type { ScreenerRow } from "./universe";

/** Notional a single plan is sized to, in USD. */
export const PER_TRADE_NOTIONAL = 1000;

/** Below this net reward-to-risk a plan is reported but flagged not worth taking. */
export const MIN_RR = 1.0;

export type PlanKind = "scalp" | "swing" | "breakout";

export const PLAN_KINDS: PlanKind[] = ["scalp", "swing", "breakout"];

export const PLAN_LABELS: Record<PlanKind, string> = {
  scalp: "Scalp",
  swing: "Swing",
  breakout: "Breakout",
};

/** Expected hold per mode, in hours. Drives the funding charge. */
export const PLAN_HOLD_HOURS: Record<PlanKind, number> = {
  scalp: 6,
  swing: 120, // 5 days
  breakout: 72, // 3 days
};

const SCALP_STOP_ATR = 1.0;
const SCALP_TARGET_R = 2.0;

/**
 * ATR is a DAILY range, and a scalp does not hold for a day.
 *
 * Every level in this module is derived from daily bars, so `atr14` measures
 * how far this contract typically travels in 24 hours. Sizing a six-hour trade
 * against it produced stops like ENAUSD's -7.0% on a plan whose exit rule is a
 * six-hour clock — a stop so wide the position would almost always close on
 * time rather than on price, which makes the reward-to-risk beside it a
 * description of a trade that never happens.
 *
 * Volatility over a shorter window scales roughly with the square root of time,
 * so a six-hour stop is about sqrt(6/24) = 0.5x the daily figure. That is an
 * approximation — it assumes returns are independent across the day, which
 * crypto only roughly satisfies around funding stamps and US session opens —
 * but it is a far better one than pretending six hours and twenty-four are the
 * same measurement. Applied only to the scalp: the swing and breakout holds are
 * multi-day, where their 2x daily multiple is already the right order.
 */
function holdScaledAtr(atr: number, holdHours: number): number {
  return atr * Math.sqrt(Math.min(holdHours, 24) / 24);
}
const SWING_STOP_ATR = 2.0;
const SWING_TARGET_R = 2.5;
const BREAKOUT_DRIFT_PCT = 2.0;
const BREAKOUT_MIN_VOL_X = 1.5;

export type PlanCosts = {
  entryFeeUsd: number;
  exitFeeUsd: number;
  fundingPct: number | null;
  fundingUsd: number;
  fundingIntervals: number;
  totalUsd: number;
};

export type TradePlan = {
  kind: PlanKind;
  label: string;
  tradable: boolean;
  blockedReason: string | null;

  entry: number;
  stop: number;
  target: number;
  stopPct: number;
  targetPct: number;

  horizon: string;
  exitRule: string;
  basis: string;

  contracts: number;
  notionalUsd: number;
  /** Stop width measured in venue ticks. Under 20 the plan is not tradable. */
  stopTicks: number | null;

  grossRewardUsd: number | null;
  grossRiskUsd: number | null;
  netRewardUsd: number | null;
  netRiskUsd: number | null;
  grossRr: number | null;
  netRr: number | null;
  costWin: PlanCosts | null;
  costLoss: PlanCosts | null;

  worthTaking: boolean;
  confirmingPatterns: {
    pattern: string;
    state: string;
    timeframe: string;
    confidence: number;
    rationale: string;
  }[];
};

/**
 * Whole contracts only.
 *
 * Contract values on this venue span a thousandfold — one BTCUSD contract is
 * 0.001 BTC, one ADAUSD contract is 1 ADA — so the contract count for a fixed
 * dollar notional is wildly different per symbol and cannot be assumed. A
 * fractional count would describe an order the venue will not accept.
 */
function contractsFor(price: number, contractValue: number | null, notional: number): number {
  if (price <= 0 || !contractValue || contractValue <= 0) return 0;
  const perContract = price * contractValue;
  return perContract > 0 ? Math.floor(notional / perContract) : 0;
}

/**
 * Gross and net reward/risk for one plan.
 *
 * The win case and the loss case are costed SEPARATELY, because they are
 * different round trips: exiting at the target and exiting at the stop have
 * different notionals and therefore different taker fees. Costing only one of
 * them and applying it to both would flatter whichever leg it was taken from.
 */
function priced(
  entry: number,
  stop: number,
  target: number,
  contracts: number,
  contractValue: number | null,
  fundingRatePct8h: number | null,
  holdHours: number,
): Pick<
  TradePlan,
  | "contracts"
  | "notionalUsd"
  | "grossRewardUsd"
  | "grossRiskUsd"
  | "netRewardUsd"
  | "netRiskUsd"
  | "grossRr"
  | "netRr"
  | "costWin"
  | "costLoss"
> {
  const cv = contractValue ?? 0;
  const qty = contracts * cv;
  const notional = entry * qty;
  const grossReward = (target - entry) * qty;
  const grossRisk = (entry - stop) * qty;

  if (contracts <= 0 || grossRisk <= 0 || notional <= 0) {
    return {
      contracts,
      notionalUsd: round(notional, 2) ?? 0,
      grossRewardUsd: null,
      grossRiskUsd: null,
      netRewardUsd: null,
      netRiskUsd: null,
      grossRr: null,
      netRr: null,
      costWin: null,
      costLoss: null,
    };
  }

  const intervals = Math.max(1, Math.ceil(holdHours / 8));
  const fundPct = fundingCostPct(fundingRatePct8h, holdHours, "long");

  const costs = (exitPrice: number): PlanCosts => {
    const entryFee = entry * qty * PERP_TAKER_FEE_RATE;
    const exitFee = exitPrice * qty * PERP_TAKER_FEE_RATE;
    // Funding accrues on the notional held. Signed from the long's side, so a
    // negative funding rate is a CREDIT and reduces the cost rather than being
    // clamped to zero — being paid to hold is a real feature of this market and
    // hiding it would misprice every discount contract on the board.
    const fundingUsd = fundPct !== null ? (notional * fundPct) / 100 : 0;
    return {
      entryFeeUsd: round(entryFee, 4) ?? 0,
      exitFeeUsd: round(exitFee, 4) ?? 0,
      fundingPct: fundPct,
      fundingUsd: round(fundingUsd, 4) ?? 0,
      fundingIntervals: intervals,
      totalUsd: round(entryFee + exitFee + fundingUsd, 4) ?? 0,
    };
  };

  const win = costs(target);
  const loss = costs(stop);
  const netReward = grossReward - win.totalUsd;
  const netRisk = grossRisk + loss.totalUsd;

  return {
    contracts,
    notionalUsd: round(notional, 2) ?? 0,
    grossRewardUsd: round(grossReward, 4),
    grossRiskUsd: round(grossRisk, 4),
    netRewardUsd: round(netReward, 4),
    netRiskUsd: round(netRisk, 4),
    grossRr: round(grossReward / grossRisk),
    netRr: netRisk > 0 ? round(netReward / netRisk) : null,
    costWin: win,
    costLoss: loss,
  };
}

/** Stop width in venue ticks, and whether the grid can express it. */
function gridCheck(
  entry: number,
  stop: number,
  tickSize: number | null,
): { ticks: number | null; ok: boolean; reason: string | null } {
  if (!tickSize || tickSize <= 0) {
    return { ticks: null, ok: false, reason: "the venue did not report a tick size for this contract" };
  }
  const ticks = Math.abs(entry - stop) / tickSize;
  if (ticks >= MIN_STOP_TICKS) return { ticks: round(ticks, 1), ok: true, reason: null };
  return {
    ticks: round(ticks, 1),
    ok: false,
    reason:
      `the stop is only ${ticks.toFixed(1)} ticks wide, under the ${MIN_STOP_TICKS}-tick floor. ` +
      `Rounding to this contract's price grid would move the stop by more than 5% of the intended ` +
      `risk, so the order the venue receives is not the order this plan describes. That is a ` +
      `property of the contract, not of today's market.`,
  };
}

function finish(
  plan: Omit<TradePlan, "worthTaking" | "confirmingPatterns">,
  hits: PatternHit[],
): TradePlan {
  const full: TradePlan = {
    ...plan,
    worthTaking: Boolean(plan.netRr !== null && plan.netRr >= MIN_RR && plan.tradable),
    confirmingPatterns: confirming(plan.kind, hits),
  };
  return full;
}

export function scalpPlan(row: ScreenerRow, hits: PatternHit[] = []): TradePlan | null {
  const entry = row.price;
  const atr = row.atr14;
  if (!entry || !atr || atr <= 0) return null;

  const scaledAtr = holdScaledAtr(atr, PLAN_HOLD_HOURS.scalp);
  const stop = entry - SCALP_STOP_ATR * scaledAtr;
  if (stop <= 0) return null;
  const target = entry + SCALP_TARGET_R * (entry - stop);
  const contracts = contractsFor(entry, row.micro.contractValue, PER_TRADE_NOTIONAL);
  const grid = gridCheck(entry, stop, row.micro.tickSize);

  return finish(
    {
      kind: "scalp",
      label: PLAN_LABELS.scalp,
      tradable: grid.ok,
      blockedReason: grid.ok ? null : `Not tradable — ${grid.reason}`,
      entry: entry,
      stop: round(stop, 8)!,
      target: round(target, 8)!,
      stopPct: round((stop / entry - 1) * 100)!,
      targetPct: round((target / entry - 1) * 100)!,
      horizon: "hours",
      exitRule:
        `Time stop at ${PLAN_HOLD_HOURS.scalp}h. There is no square-off bell on this venue — the ` +
        `market never closes, so a scalp that has not resolved is closed on the clock rather than ` +
        `left to become an accidental swing.`,
      basis:
        `Stop is ${SCALP_STOP_ATR}x the 14-day ATR SCALED TO THE HOLD — the daily ATR is ` +
        `${atr.toPrecision(4)}, and a ${PLAN_HOLD_HOURS.scalp}-hour window is about ` +
        `${scaledAtr.toPrecision(4)} of it by square-root-of-time. Sizing a ${PLAN_HOLD_HOURS.scalp}h ` +
        `trade against a full day's range gives a stop so wide the position closes on the clock ` +
        `rather than on price. Still volatility-relative rather than a fixed percentage: daily ` +
        `ranges across this universe run from under 1% to over 30%, and one number cannot fit ` +
        `both. Target is ${SCALP_TARGET_R}R. Costed at ` +
        `${(PERP_TAKER_FEE_RATE * 100).toFixed(3)}% taker per side plus one funding interval.`,
      stopTicks: grid.ticks,
      ...priced(
        entry,
        stop,
        target,
        contracts,
        row.micro.contractValue,
        row.funding.ratePct8h,
        PLAN_HOLD_HOURS.scalp,
      ),
    },
    hits,
  );
}

export function swingPlan(row: ScreenerRow, hits: PatternHit[] = []): TradePlan | null {
  const entry = row.price;
  const atr = row.atr14;
  if (!entry || !atr || atr <= 0) return null;

  const atrStop = entry - SWING_STOP_ATR * atr;
  const swingLow = row.swingLow;
  let stop: number;
  let stopBasis: string;
  // Prefer the structural level, but never a stop so far away it makes the
  // trade meaningless — fall back to the ATR stop when the swing low is below it.
  if (swingLow && swingLow < entry && swingLow >= atrStop) {
    stop = swingLow;
    stopBasis = "the last confirmed swing low";
  } else {
    stop = atrStop;
    stopBasis = `${SWING_STOP_ATR}x ATR (no usable swing low nearby)`;
  }
  if (stop <= 0) return null;

  const target = entry + SWING_TARGET_R * (entry - stop);
  const contracts = contractsFor(entry, row.micro.contractValue, PER_TRADE_NOTIONAL);
  const grid = gridCheck(entry, stop, row.micro.tickSize);
  const intervals = Math.ceil(PLAN_HOLD_HOURS.swing / 8);

  return finish(
    {
      kind: "swing",
      label: PLAN_LABELS.swing,
      tradable: grid.ok,
      blockedReason: grid.ok ? null : `Not tradable — ${grid.reason}`,
      entry: entry,
      stop: round(stop, 8)!,
      target: round(target, 8)!,
      stopPct: round((stop / entry - 1) * 100)!,
      targetPct: round((target / entry - 1) * 100)!,
      horizon: "3-10 days",
      exitRule: "Trail the stop up under each new swing low once 1R is banked",
      basis:
        `Stop is ${stopBasis}. Target is ${SWING_TARGET_R}R. Costed on ${intervals} funding ` +
        `settlements as well as both taker legs — a five-day hold crosses fifteen funding stamps, ` +
        `which on a crowded contract costs more than the fees do.`,
      stopTicks: grid.ticks,
      ...priced(
        entry,
        stop,
        target,
        contracts,
        row.micro.contractValue,
        row.funding.ratePct8h,
        PLAN_HOLD_HOURS.swing,
      ),
    },
    hits,
  );
}

export function breakoutPlan(row: ScreenerRow, hits: PatternHit[] = []): TradePlan | null {
  const ltp = row.price;
  const level = row.donchianHigh20;
  const baseLow = row.baseLow20;
  const atr = row.atr14;
  if (!ltp || !level || !baseLow || !atr || atr <= 0) return null;
  if (baseLow >= level) return null;

  // Entry is the LEVEL, not the last price: that is what the setup describes.
  const entry = level;
  const driftPct = (ltp / level - 1) * 100;
  const height = level - baseLow;
  const stop = Math.max(baseLow, entry - SWING_STOP_ATR * atr);
  if (stop <= 0 || stop >= entry) return null;
  const target = entry + height;

  const contracts = contractsFor(entry, row.micro.contractValue, PER_TRADE_NOTIONAL);
  const grid = gridCheck(entry, stop, row.micro.tickSize);
  const withinBand = driftPct <= BREAKOUT_DRIFT_PCT;

  const confirmations: string[] = [];
  if (row.volumeX !== null) {
    confirmations.push(
      row.volumeX >= BREAKOUT_MIN_VOL_X
        ? `volume ${row.volumeX.toFixed(1)}x its average — confirmed`
        : `volume only ${row.volumeX.toFixed(1)}x its average — unconfirmed break`,
    );
  }
  if (row.oi.buildup === "long_buildup") {
    confirmations.push("open interest rising with price — new longs, not a squeeze");
  } else if (row.oi.buildup === "short_covering") {
    confirmations.push(
      "open interest FALLING as price rises — this break is short covering, which tends to " +
        "exhaust rather than trend",
    );
  }

  let blocked: string | null = null;
  if (!grid.ok) blocked = `Not tradable — ${grid.reason}`;
  else if (!withinBand) {
    blocked =
      `Price is ${driftPct.toFixed(1)}% above the ${level.toPrecision(6)} level, outside the ` +
      `${BREAKOUT_DRIFT_PCT}% drift band. A contract that has already run this far past its ` +
      `trigger is not the trade the level described — taking it here is paying up for a breakout ` +
      `that already happened.`;
  }

  return finish(
    {
      kind: "breakout",
      label: PLAN_LABELS.breakout,
      tradable: grid.ok && withinBand,
      blockedReason: blocked,
      entry: entry,
      stop: round(stop, 8)!,
      target: round(target, 8)!,
      stopPct: round((stop / entry - 1) * 100)!,
      targetPct: round((target / entry - 1) * 100)!,
      horizon: "1-5 days",
      exitRule: "Exit on a close back inside the base, or at the measured move",
      basis:
        `Entry is the 20-day high (${level.toPrecision(6)}). Stop is under the base ` +
        `(${baseLow.toPrecision(6)}). Target is the measured move — the ${height.toPrecision(4)} ` +
        `base height projected up from the break. ` +
        (confirmations.length ? confirmations.join("; ") : ""),
      stopTicks: grid.ticks,
      ...priced(
        entry,
        stop,
        target,
        contracts,
        row.micro.contractValue,
        row.funding.ratePct8h,
        PLAN_HOLD_HOURS.breakout,
      ),
    },
    hits,
  );
}

export function plansFor(row: ScreenerRow, hits: PatternHit[] = []): TradePlan[] {
  return [scalpPlan(row, hits), swingPlan(row, hits), breakoutPlan(row, hits)].filter(
    (p): p is TradePlan => p !== null,
  );
}

/**
 * Bullish pattern hits on the timeframe that matches the plan's horizon.
 *
 * A scalp or breakout plan is confirmed by a daily pattern; a swing plan by a
 * weekly one. Letting a weekly cup & handle "confirm" a trade that closes in
 * six hours would be borrowing authority from a horizon the trade never reaches.
 */
export function confirming(kind: PlanKind, hits: PatternHit[]): TradePlan["confirmingPatterns"] {
  const tf = kind === "swing" ? "1w" : "1d";
  return hits
    .filter((h) => h.direction === "bullish" && h.timeframe === tf)
    .slice(0, 3)
    .map((h) => ({
      pattern: h.pattern,
      state: h.state,
      timeframe: h.timeframeLabel,
      confidence: h.confidence,
      rationale: h.rationale,
    }));
}

/**
 * Does this contract qualify for this mode at all? Returns [passes, why-not].
 *
 * The gates are deliberately about the SETUP, not about the token being good. A
 * contract can be the strongest thing on the venue over six months and a
 * terrible scalp today, and the reverse.
 */
export function gate(row: ScreenerRow, kind: PlanKind): [boolean, string] {
  const r1d = row.returns["1d"];
  const r1w = row.returns["1w"];
  const r1m = row.returns["1m"];

  // Liquidity is checked FIRST and for every mode. On this venue 24h turnover
  // spans from $1.8bn down to $4,400 — a four-hundred-thousandfold range, far
  // wider than any equity index — and a plan on a contract that traded four
  // thousand dollars yesterday is arithmetic, not a trade.
  if (row.turnoverUsd24h === null || row.turnoverUsd24h < 250_000) {
    return [
      false,
      `24h turnover is ${row.turnoverUsd24h === null ? "unknown" : "$" + Math.round(row.turnoverUsd24h).toLocaleString()} — too thin to fill`,
    ];
  }
  if (!row.micro.stopExpressible) {
    return [false, `tick grid cannot hold a stop (${row.micro.stopTicks ?? "?"} ticks)`];
  }

  if (kind === "scalp") {
    if (r1d === null || r1d <= 0) return [false, "not up over 24h"];
    if (row.volumeX === null || row.volumeX < 1.2) return [false, "no unusual volume today"];
    if (row.sma20 && row.price && row.price < row.sma20) {
      return [false, "trading below its own 20-day average"];
    }
    return [true, ""];
  }

  if (kind === "swing") {
    if (r1w === null || r1m === null) return [false, "not enough history for a weekly and monthly read"];
    if (r1w <= 0 || r1m <= 0) return [false, "week and month must both be positive"];
    if (row.sma50 && row.price && row.price < row.sma50) return [false, "below its 50-day average"];
    if ((row.emaHoldPct ?? 0) < 50) return [false, "has not been holding above its 9 EMA"];
    // A swing hold crossing fifteen funding settlements at an extreme rate can
    // cost more than the target is worth. Gated here rather than left to show
    // up as a quietly negative net R:R further down the row.
    if (row.funding.ratePct8h !== null && row.funding.ratePct8h > 0.15) {
      return [
        false,
        `funding at ${row.funding.ratePct8h.toFixed(3)}% per 8h would cost ` +
          `${(row.funding.ratePct8h * 15).toFixed(2)}% of notional over a five-day hold`,
      ];
    }
    return [true, ""];
  }

  if (row.donchianHigh20 === null) return [false, "no 20-day level stored"];
  if (row.volumeX === null || row.volumeX < BREAKOUT_MIN_VOL_X) {
    return [false, `volume below ${BREAKOUT_MIN_VOL_X}x its average — unconfirmed`];
  }
  if (!row.breakout) return [false, "has not broken a multi-day high"];
  return [true, ""];
}
