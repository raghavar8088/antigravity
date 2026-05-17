import { afterEach, describe, expect, it, vi } from "vitest";

import {
  BTC_FT_GENERATED_COUNT_DEFAULT,
  BTC_FT_GENERATED_DEFS,
  BTC_FT_GENERATED_ID_END,
  BTC_FT_GENERATED_ID_START,
  BTC_FT_GENERATED_STRATEGY_IDS,
  BTC_FT_GENERATED_TEMPLATE_CYCLE,
  buildGeneratedStrategies,
  isGeneratedPoolEnabled,
} from "@/lib/btcFtStrategyGenerator";
import {
  BTC_FT_RESEARCH_FULL_POOL,
  CORE_BTC_FT_STRATEGY_IDS,
  BTC_FUTURE_TRADING_STRATEGY_IDS,
} from "@/lib/btcFtRoster";
import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import { BTC_FT_TEMPLATE_META } from "@/lib/btcFtStrategyTemplates";
import { buildSignalInputs, evalMinuteSignal } from "@/lib/futuresSignals";

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("btcFtStrategyGenerator — pool integrity", () => {
  it("generates exactly 100 strategies in the 300–399 range", () => {
    expect(BTC_FT_GENERATED_DEFS.length).toBe(BTC_FT_GENERATED_COUNT_DEFAULT);
    expect(BTC_FT_GENERATED_DEFS.length).toBe(100);
    for (const def of BTC_FT_GENERATED_DEFS) {
      expect(def.id).toBeGreaterThanOrEqual(BTC_FT_GENERATED_ID_START);
      expect(def.id).toBeLessThanOrEqual(BTC_FT_GENERATED_ID_END);
    }
  });

  it("all generated IDs and names are unique", () => {
    const ids = BTC_FT_GENERATED_DEFS.map((d) => d.id);
    const names = BTC_FT_GENERATED_DEFS.map((d) => d.name);
    expect(new Set(ids).size).toBe(ids.length);
    expect(new Set(names).size).toBe(names.length);
  });

  it("every generated def has a real btcFtTemplate (no display-name-only rows)", () => {
    const validTemplates = new Set(BTC_FT_GENERATED_TEMPLATE_CYCLE);
    for (const def of BTC_FT_GENERATED_DEFS) {
      expect(def.btcFtTemplate).toBeDefined();
      expect(validTemplates.has(def.btcFtTemplate!)).toBe(true);
      expect(BTC_FT_TEMPLATE_META[def.btcFtTemplate!]).toBeDefined();
      expect(typeof def.btcFtVariant).toBe("number");
      expect(def.btcFtVariant!).toBeGreaterThanOrEqual(0);
      expect(def.btcFtVariant!).toBeLessThanOrEqual(3);
    }
  });

  it("does NOT collide with existing CORE or extended (200–299) IDs", () => {
    const existing = new Set<number>([
      ...CORE_BTC_FT_STRATEGY_IDS,
      ...BTC_FUTURE_TRADING_STRATEGY_IDS,
    ]);
    for (const id of BTC_FT_GENERATED_STRATEGY_IDS) {
      expect(existing.has(id)).toBe(false);
    }
  });

  it("generated IDs are NOT in production roster (CORE / BTC_FUTURE_TRADING_STRATEGY_IDS)", () => {
    for (const id of BTC_FT_GENERATED_STRATEGY_IDS) {
      expect(CORE_BTC_FT_STRATEGY_IDS).not.toContain(id);
      expect(BTC_FUTURE_TRADING_STRATEGY_IDS).not.toContain(id);
    }
  });

  it("generated IDs ARE in the research full pool", () => {
    const research = new Set(BTC_FT_RESEARCH_FULL_POOL);
    for (const id of BTC_FT_GENERATED_STRATEGY_IDS) {
      expect(research.has(id)).toBe(true);
    }
  });

  it("FUTURES_STRAT_DEFS includes all generated defs for signal routing", () => {
    const registryIds = new Set(FUTURES_STRAT_DEFS.map((d) => d.id));
    for (const id of BTC_FT_GENERATED_STRATEGY_IDS) {
      expect(registryIds.has(id)).toBe(true);
    }
  });

  it("SL/TP/hold params stay inside generator clamps", () => {
    for (const def of BTC_FT_GENERATED_DEFS) {
      expect(def.slPct).toBeGreaterThanOrEqual(0.22);
      expect(def.slPct).toBeLessThanOrEqual(0.45);
      expect(def.tpPct).toBeGreaterThanOrEqual(0.55);
      expect(def.tpPct).toBeLessThanOrEqual(1.1);
      expect(def.holdMinutes).toBeGreaterThanOrEqual(10);
      expect(def.holdMinutes).toBeLessThanOrEqual(40);
    }
  });

  it("each cycle template appears at least once", () => {
    const seen = new Set(BTC_FT_GENERATED_DEFS.map((d) => d.btcFtTemplate));
    for (const tpl of BTC_FT_GENERATED_TEMPLATE_CYCLE) {
      expect(seen.has(tpl)).toBe(true);
    }
  });

  it("LONG / SHORT split is balanced (50 / 50)", () => {
    const longs = BTC_FT_GENERATED_DEFS.filter((d) => d.signalKey.endsWith("_LONG")).length;
    const shorts = BTC_FT_GENERATED_DEFS.filter((d) => d.signalKey.endsWith("_SHORT")).length;
    expect(longs).toBe(50);
    expect(shorts).toBe(50);
  });
});

describe("isGeneratedPoolEnabled — env gating", () => {
  it("defaults ON when research mode is on", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_GENERATED_POOL", "");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_RESEARCH_MODE", "1");
    expect(isGeneratedPoolEnabled()).toBe(true);
  });

  it("defaults OFF when research mode is off", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_GENERATED_POOL", "");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_RESEARCH_MODE", "");
    expect(isGeneratedPoolEnabled()).toBe(false);
  });

  it("explicit =1 forces ON", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_GENERATED_POOL", "1");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_RESEARCH_MODE", "");
    expect(isGeneratedPoolEnabled()).toBe(true);
  });

  it("explicit =0 forces OFF even when research mode is on", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_GENERATED_POOL", "0");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_RESEARCH_MODE", "1");
    expect(isGeneratedPoolEnabled()).toBe(false);
  });
});

describe("buildGeneratedStrategies — explicit options", () => {
  it("throws when startId+count would exceed reserved 300–399 range", () => {
    expect(() => buildGeneratedStrategies({ startId: 350, count: 60 })).toThrow();
  });

  it("supports a smaller pool slice", () => {
    const slice = buildGeneratedStrategies({ count: 12 });
    expect(slice.length).toBe(12);
    expect(slice[0]!.id).toBe(300);
    expect(slice[11]!.id).toBe(311);
  });
});

// --------------------------------------------------------------------------
// Smoke replay — score must be finite & non-negative on a synthetic 1m series
// for a sample of 3 generated strategies (CI time budget).
// --------------------------------------------------------------------------

function syntheticBars(direction: "up" | "down", base = 100_000, count = 120) {
  const opens: number[] = [];
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  let price = base;
  for (let i = 0; i < count; i++) {
    const step = direction === "up" ? 1.0018 + (i % 4) * 0.00025 : 0.9982 - (i % 4) * 0.00025;
    price *= step;
    const h = price * 1.0012;
    const l = price * 0.9985;
    const o = i === 0 ? (direction === "up" ? l : h) : closes[i - 1]!;
    opens.push(o);
    closes.push(price);
    highs.push(h);
    lows.push(l);
    volumes.push(6000 + i * 180);
  }
  return { opens, closes, highs, lows, volumes };
}

describe("generated strategies — smoke score evaluation", () => {
  // 3 generated strats spanning different templates (CI cost ~10ms each)
  const SMOKE_IDS = [300, 305, 310];

  it.each(SMOKE_IDS)("strategy id %i returns a finite score on a bullish series", (id) => {
    const strat = BTC_FT_GENERATED_DEFS.find((d) => d.id === id);
    expect(strat).toBeDefined();
    const bars = syntheticBars("up");
    const t = Date.UTC(2026, 4, 17, 14, 5, 0); // London/NY overlap so SESSION_* templates can fire
    const s = buildSignalInputs(
      bars.opens,
      bars.closes,
      bars.highs,
      bars.lows,
      bars.volumes,
      bars.closes.at(-1)!,
      t,
    );
    const ev = evalMinuteSignal(s, strat!);
    expect(Number.isFinite(ev.score)).toBe(true);
    expect(ev.score).toBeGreaterThanOrEqual(0);
  });
});
