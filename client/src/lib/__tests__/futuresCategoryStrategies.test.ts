/**
 * Structural tests for the 160-strategy research pool.
 *
 * Validates: unique IDs, signal keys, category tagging, RR ratios, ID-range
 * containment, no collisions with CORE/premium IDs.
 */

import { describe, expect, it } from "vitest";
import {
  SCALPING_STRATEGIES,
  DAY_TRADING_STRATEGIES,
  SWING_TRADING_STRATEGIES,
  POSITION_TRADING_STRATEGIES,
  TREND_TRADING_STRATEGIES,
  RANGE_TRADING_STRATEGIES,
  BREAKOUT_TRADING_STRATEGIES,
  MOMENTUM_TRADING_STRATEGIES,
  CATEGORY_POOL_160,
  LEGACY_CORE_CATEGORY_MAP,
} from "../trading/futuresCategoryStrategies";
import { CATEGORY_REGISTRY as CR } from "../trading/futuresCategoryRegistry";
import {
  CATEGORY_REGISTRY,
  CATEGORY_ID_RANGES,
  TRADING_CATEGORY_IDS,
  idInCategoryBlock,
  getCategoryProfile,
} from "../trading/futuresCategoryRegistry";
import { CORE_BTC_FT_STRATEGY_IDS, CATEGORY_STRATEGY_IDS, buildCategoryRoster } from "../trading/btcFtRoster";
import { BTC_FT_PREMIUM_STRATEGY_IDS } from "../trading/btcFtPremiumStrategies";
import type { FuturesStratDef } from "../trading/futuresStratTypes";

describe("CATEGORY_REGISTRY structural invariants", () => {
  it("has exactly 8 categories", () => {
    expect(TRADING_CATEGORY_IDS.length).toBe(8);
    expect(Object.keys(CATEGORY_REGISTRY).length).toBe(8);
  });

  it("each category profile has valid bounds", () => {
    for (const id of TRADING_CATEGORY_IDS) {
      const p = getCategoryProfile(id);
      expect(p.holdMinutesMin).toBeGreaterThan(0);
      expect(p.holdMinutesMax).toBeGreaterThan(p.holdMinutesMin);
      expect(p.slPctMin).toBeGreaterThan(0);
      expect(p.slPctMax).toBeGreaterThan(p.slPctMin);
      expect(p.defaultLeverage).toBeLessThanOrEqual(p.maxLeverage);
      expect(p.maxOpenPositions).toBeGreaterThan(0);
    }
  });

  it("swing/position categories never use 25x leverage", () => {
    expect(CATEGORY_REGISTRY.swing_trading.maxLeverage).toBeLessThanOrEqual(10);
    expect(CATEGORY_REGISTRY.position_trading.maxLeverage).toBeLessThanOrEqual(5);
  });

  it("ID ranges don't overlap", () => {
    const ranges = Object.values(CATEGORY_ID_RANGES);
    for (let i = 0; i < ranges.length; i++) {
      for (let j = i + 1; j < ranges.length; j++) {
        const [aLo, aHi] = ranges[i];
        const [bLo, bHi] = ranges[j];
        expect(aHi < bLo || bHi < aLo).toBe(true);
      }
    }
  });
});

describe("SCALPING_STRATEGIES (Category 1, 600–619)", () => {
  it("contains exactly 20 strategies", () => {
    expect(SCALPING_STRATEGIES.length).toBe(20);
  });

  it("all IDs are unique and in range 600–619", () => {
    const ids = SCALPING_STRATEGIES.map((d) => d.id);
    expect(new Set(ids).size).toBe(20);
    for (const id of ids) {
      expect(id).toBeGreaterThanOrEqual(600);
      expect(id).toBeLessThanOrEqual(619);
      expect(idInCategoryBlock(id, "scalping")).toBe(true);
    }
  });

  it("all signal keys are unique", () => {
    const keys = SCALPING_STRATEGIES.map((d) => d.signalKey);
    expect(new Set(keys).size).toBe(20);
  });

  it("all defs are researchOnly: true", () => {
    for (const d of SCALPING_STRATEGIES) {
      expect(d.researchOnly).toBe(true);
    }
  });

  it("all defs tagged tradingCategory: scalping", () => {
    for (const d of SCALPING_STRATEGIES) {
      expect(d.tradingCategory).toBe("scalping");
    }
  });

  it("all SL within scalping band [0.35, 0.65]", () => {
    const p = CATEGORY_REGISTRY.scalping;
    for (const d of SCALPING_STRATEGIES) {
      expect(d.slPct).toBeGreaterThanOrEqual(p.slPctMin);
      expect(d.slPct).toBeLessThanOrEqual(p.slPctMax);
    }
  });

  it("TP:SL ratio ≥ category min (2.5)", () => {
    const minRatio = CATEGORY_REGISTRY.scalping.tpSlRatioMin;
    for (const d of SCALPING_STRATEGIES) {
      const ratio = d.tpPct / d.slPct;
      expect(ratio).toBeGreaterThanOrEqual(minRatio - 1e-9);
    }
  });

  it("hold within scalping band", () => {
    const p = CATEGORY_REGISTRY.scalping;
    for (const d of SCALPING_STRATEGIES) {
      expect(d.holdMinutes).toBeGreaterThanOrEqual(p.holdMinutesMin);
      expect(d.holdMinutes).toBeLessThanOrEqual(p.holdMinutesMax);
    }
  });

  it("leverage ≤ category max (25x)", () => {
    for (const d of SCALPING_STRATEGIES) {
      expect(d.defaultLeverage ?? 0).toBeLessThanOrEqual(CATEGORY_REGISTRY.scalping.maxLeverage);
    }
  });

  it("templateFamily is set for burst guard", () => {
    for (const d of SCALPING_STRATEGIES) {
      expect(d.templateFamily).toBeTruthy();
      expect(d.templateFamily!.length).toBeGreaterThan(0);
    }
  });

  it("each templateFamily appears as both long and short", () => {
    const families = new Map<string, { long: boolean; short: boolean }>();
    for (const d of SCALPING_STRATEGIES) {
      const fam = d.templateFamily!;
      const entry = families.get(fam) ?? { long: false, short: false };
      if (d.signalKey.endsWith("_LONG")) entry.long = true;
      if (d.signalKey.endsWith("_SHORT")) entry.short = true;
      families.set(fam, entry);
    }
    for (const [fam, sides] of families) {
      expect(sides.long, `family ${fam} missing LONG`).toBe(true);
      expect(sides.short, `family ${fam} missing SHORT`).toBe(true);
    }
  });

  it("primaryBarInterval is 1m for all scalping", () => {
    for (const d of SCALPING_STRATEGIES) {
      expect(d.primaryBarInterval).toBe("1m");
    }
  });
});

describe("DAY_TRADING_STRATEGIES (Category 2, 620–639)", () => {
  it("contains exactly 20 strategies", () => {
    expect(DAY_TRADING_STRATEGIES.length).toBe(20);
  });

  it("all IDs are unique and in range 620–639", () => {
    const ids = DAY_TRADING_STRATEGIES.map((d) => d.id);
    expect(new Set(ids).size).toBe(20);
    for (const id of ids) {
      expect(id).toBeGreaterThanOrEqual(620);
      expect(id).toBeLessThanOrEqual(639);
    }
  });

  it("all signal keys are unique and start with DAY_", () => {
    const keys = DAY_TRADING_STRATEGIES.map((d) => d.signalKey);
    expect(new Set(keys).size).toBe(20);
    for (const k of keys) {
      expect(k.startsWith("DAY_")).toBe(true);
    }
  });

  it("all defs researchOnly:true, tradingCategory:day_trading", () => {
    for (const d of DAY_TRADING_STRATEGIES) {
      expect(d.researchOnly).toBe(true);
      expect(d.tradingCategory).toBe("day_trading");
    }
  });

  it("all SL within day_trading band [0.50, 1.00]", () => {
    const p = CR.day_trading;
    for (const d of DAY_TRADING_STRATEGIES) {
      expect(d.slPct).toBeGreaterThanOrEqual(p.slPctMin);
      expect(d.slPct).toBeLessThanOrEqual(p.slPctMax);
    }
  });

  it("TP:SL ratio ≥ category min (2.0) for all 620–639", () => {
    const minRatio = CR.day_trading.tpSlRatioMin;
    for (const d of DAY_TRADING_STRATEGIES) {
      const ratio = d.tpPct / d.slPct;
      expect(ratio, `id ${d.id}`).toBeGreaterThanOrEqual(minRatio - 1e-9);
    }
  });

  it("hold within day_trading band [30, 480]", () => {
    const p = CR.day_trading;
    for (const d of DAY_TRADING_STRATEGIES) {
      expect(d.holdMinutes).toBeGreaterThanOrEqual(p.holdMinutesMin);
      expect(d.holdMinutes).toBeLessThanOrEqual(p.holdMinutesMax);
    }
  });

  it("leverage ≤ day_trading maxLeverage (20)", () => {
    for (const d of DAY_TRADING_STRATEGIES) {
      expect(d.defaultLeverage ?? 0).toBeLessThanOrEqual(CR.day_trading.maxLeverage);
    }
  });

  it("primaryBarInterval is 5m for all day_trading", () => {
    for (const d of DAY_TRADING_STRATEGIES) {
      expect(d.primaryBarInterval).toBe("5m");
    }
  });

  it("HTF strats carry confirmBarInterval: 15m", () => {
    for (const d of DAY_TRADING_STRATEGIES) {
      if (d.requiresHtf) {
        expect(d.confirmBarInterval).toBe("15m");
      }
    }
  });

  it("each templateFamily appears as both LONG and SHORT", () => {
    const families = new Map<string, { long: boolean; short: boolean }>();
    for (const d of DAY_TRADING_STRATEGIES) {
      const fam = d.templateFamily!;
      const entry = families.get(fam) ?? { long: false, short: false };
      if (d.signalKey.endsWith("_LONG")) entry.long = true;
      if (d.signalKey.endsWith("_SHORT")) entry.short = true;
      families.set(fam, entry);
    }
    for (const [fam, sides] of families) {
      expect(sides.long, `family ${fam} missing LONG`).toBe(true);
      expect(sides.short, `family ${fam} missing SHORT`).toBe(true);
    }
  });
});

describe("SWING_TRADING_STRATEGIES (Category 3, 640–659)", () => {
  it("contains exactly 20 strategies", () => {
    expect(SWING_TRADING_STRATEGIES.length).toBe(20);
  });

  it("all IDs are unique and in range 640–659", () => {
    const ids = SWING_TRADING_STRATEGIES.map((d) => d.id);
    expect(new Set(ids).size).toBe(20);
    for (const id of ids) {
      expect(id).toBeGreaterThanOrEqual(640);
      expect(id).toBeLessThanOrEqual(659);
      expect(idInCategoryBlock(id, "swing_trading")).toBe(true);
    }
  });

  it("all signal keys are unique and start with SWG_", () => {
    const keys = SWING_TRADING_STRATEGIES.map((d) => d.signalKey);
    expect(new Set(keys).size).toBe(20);
    for (const k of keys) {
      expect(k.startsWith("SWG_")).toBe(true);
    }
  });

  it("all defs researchOnly:true, tradingCategory:swing_trading", () => {
    for (const d of SWING_TRADING_STRATEGIES) {
      expect(d.researchOnly).toBe(true);
      expect(d.tradingCategory).toBe("swing_trading");
    }
  });

  it("all SL within swing band [1.0, 2.5]", () => {
    const p = CR.swing_trading;
    for (const d of SWING_TRADING_STRATEGIES) {
      expect(d.slPct, `id ${d.id}`).toBeGreaterThanOrEqual(p.slPctMin);
      expect(d.slPct, `id ${d.id}`).toBeLessThanOrEqual(p.slPctMax);
    }
  });

  it("TP:SL ratio ≥ swing min (2.0)", () => {
    const minRatio = CR.swing_trading.tpSlRatioMin;
    for (const d of SWING_TRADING_STRATEGIES) {
      const ratio = d.tpPct / d.slPct;
      expect(ratio, `id ${d.id}`).toBeGreaterThanOrEqual(minRatio - 1e-9);
    }
  });

  it("hold within swing band [1440, 20160] minutes (1–14 days)", () => {
    const p = CR.swing_trading;
    for (const d of SWING_TRADING_STRATEGIES) {
      expect(d.holdMinutes, `id ${d.id}`).toBeGreaterThanOrEqual(p.holdMinutesMin);
      expect(d.holdMinutes, `id ${d.id}`).toBeLessThanOrEqual(p.holdMinutesMax);
    }
  });

  it("leverage ≤ swing maxLeverage (10×) — never 25×", () => {
    for (const d of SWING_TRADING_STRATEGIES) {
      expect(d.defaultLeverage ?? 0, `id ${d.id}`).toBeLessThanOrEqual(CR.swing_trading.maxLeverage);
    }
  });

  it("primaryBarInterval is 4h for all swing strategies", () => {
    for (const d of SWING_TRADING_STRATEGIES) {
      expect(d.primaryBarInterval).toBe("4h");
    }
  });

  it("confluenceMin ≥ swing category default (6)", () => {
    const p = CR.swing_trading;
    for (const d of SWING_TRADING_STRATEGIES) {
      expect(d.confluenceMin).toBeGreaterThanOrEqual(p.confluenceMinDefault);
    }
  });

  it("each templateFamily appears as both LONG and SHORT", () => {
    const families = new Map<string, { long: boolean; short: boolean }>();
    for (const d of SWING_TRADING_STRATEGIES) {
      const fam = d.templateFamily!;
      const entry = families.get(fam) ?? { long: false, short: false };
      if (d.signalKey.endsWith("_LONG")) entry.long = true;
      if (d.signalKey.endsWith("_SHORT")) entry.short = true;
      families.set(fam, entry);
    }
    for (const [fam, sides] of families) {
      expect(sides.long, `family ${fam} missing LONG`).toBe(true);
      expect(sides.short, `family ${fam} missing SHORT`).toBe(true);
    }
  });
});

describe("Categories 4–8 (stubs, PRs 5–9)", () => {
  it("are all empty arrays in this PR", () => {
    expect(POSITION_TRADING_STRATEGIES.length).toBe(0);
    expect(TREND_TRADING_STRATEGIES.length).toBe(0);
    expect(RANGE_TRADING_STRATEGIES.length).toBe(0);
    expect(BREAKOUT_TRADING_STRATEGIES.length).toBe(0);
    expect(MOMENTUM_TRADING_STRATEGIES.length).toBe(0);
  });
});

describe("CATEGORY_POOL_160 — combined pool", () => {
  it("contains 60 strategies after PR4 (Scalping + Day + Swing)", () => {
    expect(CATEGORY_POOL_160.length).toBe(60);
  });

  it("no ID collides with CORE 91–152 or premium 500–503", () => {
    const reservedIds = new Set<number>([
      ...CORE_BTC_FT_STRATEGY_IDS,
      ...BTC_FT_PREMIUM_STRATEGY_IDS,
    ]);
    for (const d of CATEGORY_POOL_160) {
      expect(reservedIds.has(d.id)).toBe(false);
    }
  });

  it("all IDs are unique across the full pool", () => {
    const ids = CATEGORY_POOL_160.map((d) => d.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("all signal keys unique across the full pool", () => {
    const keys = CATEGORY_POOL_160.map((d) => d.signalKey);
    expect(new Set(keys).size).toBe(keys.length);
  });

  it("no overlap with CORE signal keys (no SCP_ key reused by legacy)", () => {
    const legacyKeys = new Set([
      "TREND_CONT_LONG", "TREND_CONT_SHORT", "BREAKOUT_LONG", "BREAKOUT_SHORT",
      "OPEN_DRIVE_LONG", "OPEN_DRIVE_SHORT",
    ]);
    for (const d of CATEGORY_POOL_160) {
      expect(legacyKeys.has(d.signalKey)).toBe(false);
    }
  });
});

describe("LEGACY_CORE_CATEGORY_MAP", () => {
  it("covers all 20 CORE + 4 premium IDs", () => {
    expect(LEGACY_CORE_CATEGORY_MAP.size).toBe(24);
  });

  it("each mapped category is a valid TradingCategoryId", () => {
    for (const cat of LEGACY_CORE_CATEGORY_MAP.values()) {
      expect(TRADING_CATEGORY_IDS).toContain(cat);
    }
  });
});

describe("CATEGORY_STRATEGY_IDS roster", () => {
  it("scalping has exactly 20 IDs", () => {
    expect(CATEGORY_STRATEGY_IDS.scalping.length).toBe(20);
  });

  it("day_trading has exactly 20 IDs", () => {
    expect(CATEGORY_STRATEGY_IDS.day_trading.length).toBe(20);
  });

  it("swing_trading has exactly 20 IDs", () => {
    expect(CATEGORY_STRATEGY_IDS.swing_trading.length).toBe(20);
  });

  it("remaining categories are empty (stubs)", () => {
    expect(CATEGORY_STRATEGY_IDS.position_trading.length).toBe(0);
    expect(CATEGORY_STRATEGY_IDS.trend_trading.length).toBe(0);
    expect(CATEGORY_STRATEGY_IDS.range_trading.length).toBe(0);
    expect(CATEGORY_STRATEGY_IDS.breakout_trading.length).toBe(0);
    expect(CATEGORY_STRATEGY_IDS.momentum_trading.length).toBe(0);
  });
});

describe("buildCategoryRoster", () => {
  it("returns ≤8 scalping strategies in research mode", () => {
    const roster: FuturesStratDef[] = buildCategoryRoster("scalping", { researchMode: true });
    expect(roster.length).toBeGreaterThan(0);
    expect(roster.length).toBeLessThanOrEqual(8);
    for (const d of roster) {
      expect(d.tradingCategory).toBe("scalping");
    }
  });

  it("returns ≤8 day_trading defs in research mode", () => {
    const roster: FuturesStratDef[] = buildCategoryRoster("day_trading", { researchMode: true });
    expect(roster.length).toBeGreaterThan(0);
    expect(roster.length).toBeLessThanOrEqual(8);
    for (const d of roster) {
      expect(d.tradingCategory).toBe("day_trading");
    }
  });

  it("returns ≤8 swing_trading defs in research mode", () => {
    const roster: FuturesStratDef[] = buildCategoryRoster("swing_trading", { researchMode: true });
    expect(roster.length).toBeGreaterThan(0);
    expect(roster.length).toBeLessThanOrEqual(8);
    for (const d of roster) {
      expect(d.tradingCategory).toBe("swing_trading");
    }
  });

  it("returns empty for still-stub category", () => {
    const roster: FuturesStratDef[] = buildCategoryRoster("position_trading", { researchMode: true });
    expect(roster.length).toBe(0);
  });

  it("applies winners gate when researchMode false and winnerIds non-empty", () => {
    const winners = new Set([600, 601]);
    const roster: FuturesStratDef[] = buildCategoryRoster("scalping", { winnerIds: winners, researchMode: false });
    expect(roster.length).toBe(2);
    expect(roster.every((d) => winners.has(d.id))).toBe(true);
  });
});
