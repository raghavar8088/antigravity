import { describe, expect, it } from "vitest";
import {
  BTC_FT_PREMIUM_DEFS,
  BTC_FT_PREMIUM_STRATEGY_IDS,
  PREMIUM_NOTIONAL_MULTIPLIER,
  isPremiumStrategy,
} from "./btcFtPremiumStrategies";
import { buildSignalInputs, evalMinuteSignal, passesEntryConfirmation } from "./futuresSignals";
import { FUTURES_STRAT_DEFS } from "./futuresStrategies";
import { CORE_BTC_FT_STRATEGY_IDS } from "./btcFtRoster";
import type { BtcFtTemplateId, FuturesStratDef } from "./futuresStratTypes";

const PREMIUM_SCORE_THRESHOLD = 26;

function stratByTpl(tpl: BtcFtTemplateId, side: "LONG" | "SHORT"): FuturesStratDef {
  const s = BTC_FT_PREMIUM_DEFS.find(
    (d) => d.btcFtTemplate === tpl && d.signalKey.endsWith(side === "LONG" ? "_LONG" : "_SHORT"),
  );
  if (!s) throw new Error(`No premium strat for ${tpl}/${side}`);
  return s;
}

// ============================================================================
// Registry integrity
// ============================================================================
describe("btcFtPremiumStrategies — registry", () => {
  it("registers exactly 4 strategies (2 hypotheses × 2 sides)", () => {
    expect(BTC_FT_PREMIUM_DEFS.length).toBe(4);
    expect(BTC_FT_PREMIUM_STRATEGY_IDS).toEqual([500, 501, 502, 503]);
  });

  it("all premium defs carry tier=premium", () => {
    for (const def of BTC_FT_PREMIUM_DEFS) {
      expect(def.tier).toBe("premium");
      expect(isPremiumStrategy(def)).toBe(true);
    }
  });

  it("regular strategies are NOT premium", () => {
    const regular = FUTURES_STRAT_DEFS.find((d) => d.id === 91);
    expect(regular).toBeDefined();
    expect(isPremiumStrategy(regular!)).toBe(false);
  });

  it("premium IDs are included in CORE roster (always active)", () => {
    for (const id of BTC_FT_PREMIUM_STRATEGY_IDS) {
      expect(CORE_BTC_FT_STRATEGY_IDS).toContain(id);
    }
  });

  it("PREMIUM_NOTIONAL_MULTIPLIER is 2.0", () => {
    expect(PREMIUM_NOTIONAL_MULTIPLIER).toBe(2.0);
  });

  it("premium defs use tighter R:R (TP/SL >= 2.0)", () => {
    for (const def of BTC_FT_PREMIUM_DEFS) {
      expect(def.tpPct / def.slPct).toBeGreaterThanOrEqual(2.0);
    }
  });

  it("premium defs have higher confluenceMin than generated (>=6)", () => {
    for (const def of BTC_FT_PREMIUM_DEFS) {
      expect(def.confluenceMin).toBeGreaterThanOrEqual(6);
    }
  });
});

// ============================================================================
// Hypothesis 1: VWAP Session Rejection
// ============================================================================

/**
 * Build bars where current price is sitting 0.6% BELOW a recent VWAP-equivalent
 * mean, with low RSI, high vol on the dip, and overall HTF up-trend. Should
 * trigger the LONG signal during session open.
 */
function vwapDipBars(base = 100_000) {
  const opens: number[] = [];
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  let price = base;
  // Phase 1 — 100 bars of strong steady uptrend at NORMAL volume.
  // This builds HTF EMA9 > EMA21, and accumulates VWAP at higher prices.
  for (let i = 0; i < 100; i++) {
    price *= 1.0012; // ~12% uptrend over 100 bars
    const h = price * 1.0006;
    const l = price * 0.9996;
    const o = i === 0 ? l : closes[i - 1]!;
    opens.push(o); closes.push(price); highs.push(h); lows.push(l);
    volumes.push(8_000);
  }
  // Phase 2 — 15 bars of sharp dip on rising volume (the "rejection" event).
  // Drops ~2% so price ends well below rolling VWAP. Volume builds to >2x baseline.
  for (let i = 0; i < 15; i++) {
    price *= 0.9985;
    const h = price * 1.0004;
    const l = price * 0.998;
    const o = closes[closes.length - 1]!;
    opens.push(o); closes.push(price); highs.push(h); lows.push(l);
    volumes.push(18_000 + i * 1200); // ramp 18k → 36k (vs 8k baseline → ratio > 2x)
  }
  return { opens, closes, highs, lows, volumes };
}

describe("Premium #1 — VWAP Session-Open Rejection (LONG)", () => {
  it("scores high + confirms when price is 0.5%+ below VWAP in session window with vol surge + RSI oversold", () => {
    const bars = vwapDipBars();
    const strat = stratByTpl("PRM_VWAP_REJECT", "LONG");
    // UTC 08:30 — in London open window
    const t = Date.UTC(2026, 4, 18, 8, 30, 0);
    const s = buildSignalInputs(
      bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes,
      bars.closes.at(-1)!, t,
    );
    // Diagnostic to ensure synthetic fixture meets confirmation gates (uses mean20):
    const meanDistPct = ((s.price - s.mean20) / s.mean20) * 100;
    expect(meanDistPct).toBeLessThan(-0.5); // price below recent mean
    expect(s.volRatio).toBeGreaterThan(1.4); // vol surge
    expect(s.rsi14).toBeLessThan(40); // RSI oversold
    expect(s.htf5_trend).toBeGreaterThanOrEqual(0); // HTF still up
    const ev = evalMinuteSignal(s, strat);
    expect(ev.score).toBeGreaterThanOrEqual(PREMIUM_SCORE_THRESHOLD);
    expect(passesEntryConfirmation(s, strat)).toBe(true);
  });

  it("does NOT confirm outside session hours (UTC 03:00) even with same setup", () => {
    const bars = vwapDipBars();
    const strat = stratByTpl("PRM_VWAP_REJECT", "LONG");
    const t = Date.UTC(2026, 4, 18, 3, 0, 0); // outside session
    const s = buildSignalInputs(
      bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes,
      bars.closes.at(-1)!, t,
    );
    expect(passesEntryConfirmation(s, strat)).toBe(false);
  });

  it("does NOT confirm on flat market (no VWAP rejection signal)", () => {
    const flatBars = (() => {
      const o: number[] = [], c: number[] = [], h: number[] = [], l: number[] = [], v: number[] = [];
      const base = 100_000;
      for (let i = 0; i < 120; i++) {
        const p = base * (1 + (Math.sin(i / 4) * 0.0003));
        o.push(p); c.push(p); h.push(p * 1.0001); l.push(p * 0.9999); v.push(5_000);
      }
      return { opens: o, closes: c, highs: h, lows: l, volumes: v };
    })();
    const strat = stratByTpl("PRM_VWAP_REJECT", "LONG");
    const t = Date.UTC(2026, 4, 18, 8, 30, 0);
    const s = buildSignalInputs(
      flatBars.opens, flatBars.closes, flatBars.highs, flatBars.lows, flatBars.volumes,
      flatBars.closes.at(-1)!, t,
    );
    expect(passesEntryConfirmation(s, strat)).toBe(false);
  });
});

// ============================================================================
// Hypothesis 2: Volume-Price Divergence Reversal
// ============================================================================

/**
 * Build bars where price hits a new 20-bar LOW but OBV (running cumulative
 * signed volume) is RISING and volume on the new low is THIN. Should trigger
 * the LONG divergence signal.
 */
function bullishDivergenceBars(base = 100_000) {
  const opens: number[] = [];
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  let price = base;
  // Phase 1 — 90 bars: slow downtrend at moderate volume ending around 99K.
  // OBV drifts down here (mostly down-close days).
  for (let i = 0; i < 90; i++) {
    price *= 0.9999;
    const h = price * 1.0003;
    const l = price * 0.9995;
    const o = i === 0 ? h : closes[i - 1]!;
    opens.push(o); closes.push(price); highs.push(h); lows.push(l);
    volumes.push(40_000);
  }
  // Phase 2 — 6 bars of strong up-closes at HIGH volume.
  // This makes OBV slope rise sharply right before the final dip.
  // Each up-close at 80k volume → OBV adds 480k over 6 bars (vs 0 net before).
  for (let i = 0; i < 6; i++) {
    price *= 1.0005;
    const h = price * 1.0005;
    const l = price * 0.9999;
    const o = closes[closes.length - 1]!;
    opens.push(o); closes.push(price); highs.push(h); lows.push(l);
    volumes.push(80_000);
  }
  // Phase 3 — last 4 bars: 3 mild down-closes then 1 final new low.
  // Each down-close at THIN volume (3k each) only subtracts a tiny amount from OBV.
  // So slope over last 5 bars (1 phase-2 up + 4 small down) stays POSITIVE,
  // while the final bar still prints below the 20-bar low established earlier.
  const pivotHigh = price;
  for (let i = 0; i < 3; i++) {
    price *= 0.9996;
    const h = price * 1.0001;
    const l = price * 0.9998;
    const o = closes[closes.length - 1]!;
    opens.push(o); closes.push(price); highs.push(h); lows.push(l);
    volumes.push(3_000); // very thin
  }
  // Final bar: dip just below the lowest of the recent 20 bars on THIN volume.
  const recentLows = lows.slice(-20);
  const minRecent = Math.min(...recentLows);
  price = minRecent * 0.999;
  opens.push(closes[closes.length - 1]!);
  closes.push(price);
  highs.push(price * 1.0001);
  lows.push(price * 0.999);
  volumes.push(2_500);
  return { opens, closes, highs, lows, volumes };
}

describe("Premium #2 — Volume-Price Divergence (LONG)", () => {
  it("scores high + confirms on new 20-bar low with OBV rising and thin volume", () => {
    const bars = bullishDivergenceBars();
    const strat = stratByTpl("PRM_VOL_DIVERGENCE", "LONG");
    const s = buildSignalInputs(
      bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes,
      bars.closes.at(-1)!,
    );
    // Diagnostic to ensure synthetic fixture meets confirmation gates:
    expect(s.price).toBeLessThanOrEqual(s.low20 * 1.002); // near or at new 20-bar low
    expect(s.obvSlope).toBeGreaterThan(0); // OBV diverges from price
    expect(s.volRatio).toBeLessThan(1.3); // thin volume
    expect(s.rsi14).toBeLessThan(40); // RSI extended
    const ev = evalMinuteSignal(s, strat);
    expect(ev.score).toBeGreaterThanOrEqual(PREMIUM_SCORE_THRESHOLD);
    expect(passesEntryConfirmation(s, strat)).toBe(true);
  });

  it("does NOT confirm when volume is HIGH on the new low (no divergence)", () => {
    const bars = bullishDivergenceBars();
    // Override last 5 bars to have HIGH volume → no divergence, no entry
    const high = [...bars.volumes];
    for (let i = high.length - 5; i < high.length; i++) high[i] = 80_000;
    const strat = stratByTpl("PRM_VOL_DIVERGENCE", "LONG");
    const s = buildSignalInputs(
      bars.opens, bars.closes, bars.highs, bars.lows, high,
      bars.closes.at(-1)!,
    );
    expect(passesEntryConfirmation(s, strat)).toBe(false);
  });
});
