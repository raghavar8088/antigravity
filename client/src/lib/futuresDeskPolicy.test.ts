import { describe, expect, it } from "vitest";
import { FUTURES_STRAT_DEFS } from "./futuresStrategies";
import {
  buildPaperDeskStrategies,
  deskEffectiveHoldMinutesAtOpen,
  deskHoldMinutesCategoryMul,
  FAKE_DIVERSITY_STRAT_IDS,
  HOLD_MUL_AFTER_TP_WIDEN,
} from "./futuresDeskPolicy";

describe("deskEffectiveHoldMinutesAtOpen", () => {
  it("bumps base hold only for scalp_aggro_v1 when desk widened TP", () => {
    expect(deskEffectiveHoldMinutesAtOpen(20, "baseline", true)).toEqual({ holdMinutes: 20, profileAdjusted: false });
    expect(deskEffectiveHoldMinutesAtOpen(20, "scalp_aggro_v1", false)).toEqual({ holdMinutes: 20, profileAdjusted: false });
    expect(deskEffectiveHoldMinutesAtOpen(20, "scalp_aggro_v1", undefined)).toEqual({ holdMinutes: 20, profileAdjusted: false });
    const r = deskEffectiveHoldMinutesAtOpen(20, "scalp_aggro_v1", true);
    expect(r.profileAdjusted).toBe(true);
    expect(r.holdMinutes).toBeCloseTo(20 * HOLD_MUL_AFTER_TP_WIDEN, 8);
  });
});

describe("buildPaperDeskStrategies", () => {
  it("filters fake-diversity IDs 79–110 when allowFakeDiversity is false", () => {
    const r = buildPaperDeskStrategies(FUTURES_STRAT_DEFS, {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: false,
    });
    const ids = new Set(r.strategies.map((s) => s.id));
    for (const id of FAKE_DIVERSITY_STRAT_IDS) {
      expect(ids.has(id)).toBe(false);
    }
    expect(r.fakeDiversityFilteredCount).toBe(FAKE_DIVERSITY_STRAT_IDS.length);
  });

  it("records TP-widened ids when ratio was below min", () => {
    const r = buildPaperDeskStrategies(FUTURES_STRAT_DEFS, {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.tpWidenedStratIds.length).toBeGreaterThan(0);
    expect(r.lowRrSkippedStratIds.length).toBe(0);
  });

  it("tags strategies with deskTpWidened when TP% was raised", () => {
    const r = buildPaperDeskStrategies(FUTURES_STRAT_DEFS, {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    const widened = r.strategies.filter((s) => s.deskTpWidened === true);
    expect(widened.length).toBe(r.tpWidenedStratIds.length);
    for (const s of r.strategies) {
      expect(typeof s.deskTpWidened).toBe("boolean");
    }
  });

  it("applies category hold multiplier in desk build (MeanRev > raw)", () => {
    const raw = FUTURES_STRAT_DEFS.find((s) => s.id === 3)!;
    expect(deskHoldMinutesCategoryMul(raw.category)).toBeGreaterThan(1);
    const r = buildPaperDeskStrategies([raw], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    const built = r.strategies.find((s) => s.id === 3);
    expect(built!.holdMinutes).toBeCloseTo(raw.holdMinutes * deskHoldMinutesCategoryMul(raw.category), 4);
  });
});
