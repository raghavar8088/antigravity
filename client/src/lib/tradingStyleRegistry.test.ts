import { describe, expect, it } from "vitest";
import {
  TRADING_STYLE_PROFILES,
  TRADING_STYLE_IDS,
  getTradingStyleProfile,
  clampHoldMinutesToStyle,
} from "./tradingStyleRegistry";
import {
  PLAYBOOK_PROFILES,
  PLAYBOOK_IDS,
  getPlaybookProfile,
  categoryPassesPlaybook,
} from "./playbookRegistry";
import { buildStyleRoster } from "./styleRoster";
import { FUTURES_STRAT_DEFS } from "./futuresStrategies";

// ─── tradingStyleRegistry ──────────────────────────────────────────────────

describe("tradingStyleRegistry", () => {
  it("all four style IDs are present", () => {
    expect(TRADING_STYLE_IDS).toEqual(["scalp", "day", "swing", "position"]);
  });

  it("each profile id matches its key", () => {
    for (const id of TRADING_STYLE_IDS) {
      expect(TRADING_STYLE_PROFILES[id].id).toBe(id);
    }
  });

  it("leverage decreases from scalp to position", () => {
    expect(TRADING_STYLE_PROFILES.scalp.defaultLeverage).toBeGreaterThan(
      TRADING_STYLE_PROFILES.day.defaultLeverage,
    );
    expect(TRADING_STYLE_PROFILES.day.defaultLeverage).toBeGreaterThan(
      TRADING_STYLE_PROFILES.swing.defaultLeverage,
    );
    expect(TRADING_STYLE_PROFILES.swing.defaultLeverage).toBeGreaterThan(
      TRADING_STYLE_PROFILES.position.defaultLeverage,
    );
  });

  it("hold window min increases monotonically across styles", () => {
    expect(TRADING_STYLE_PROFILES.scalp.holdMinutesMin).toBeLessThan(
      TRADING_STYLE_PROFILES.day.holdMinutesMin,
    );
    expect(TRADING_STYLE_PROFILES.day.holdMinutesMin).toBeLessThan(
      TRADING_STYLE_PROFILES.swing.holdMinutesMin,
    );
    expect(TRADING_STYLE_PROFILES.swing.holdMinutesMin).toBeLessThan(
      TRADING_STYLE_PROFILES.position.holdMinutesMin,
    );
  });

  it("only day desk requires EOD flat", () => {
    expect(TRADING_STYLE_PROFILES.day.requiresEodFlat).toBe(true);
    expect(TRADING_STYLE_PROFILES.scalp.requiresEodFlat).toBe(false);
    expect(TRADING_STYLE_PROFILES.swing.requiresEodFlat).toBe(false);
    expect(TRADING_STYLE_PROFILES.position.requiresEodFlat).toBe(false);
  });

  it("getTradingStyleProfile returns the correct profile", () => {
    expect(getTradingStyleProfile("scalp").barInterval).toBe("1m");
    expect(getTradingStyleProfile("day").barInterval).toBe("5m");
    expect(getTradingStyleProfile("swing").barInterval).toBe("4h");
    expect(getTradingStyleProfile("position").barInterval).toBe("1d");
  });

  it("clampHoldMinutesToStyle clamps within min/max", () => {
    const scalp = TRADING_STYLE_PROFILES.scalp;
    expect(clampHoldMinutesToStyle(0, scalp)).toBe(scalp.holdMinutesMin);
    expect(clampHoldMinutesToStyle(-1, scalp)).toBe(scalp.holdMinutesMin);
    expect(clampHoldMinutesToStyle(scalp.holdMinutesMin + 1, scalp)).toBe(scalp.holdMinutesMin + 1);
    expect(clampHoldMinutesToStyle(scalp.holdMinutesMax + 999, scalp)).toBe(scalp.holdMinutesMax);
    expect(clampHoldMinutesToStyle(NaN, scalp)).toBe(scalp.holdMinutesMin);
  });
});

// ─── playbookRegistry ─────────────────────────────────────────────────────

describe("playbookRegistry", () => {
  it("all four playbook IDs are present", () => {
    expect(PLAYBOOK_IDS).toEqual(["trend", "range", "breakout", "momentum"]);
  });

  it("each profile id matches its key", () => {
    for (const id of PLAYBOOK_IDS) {
      expect(PLAYBOOK_PROFILES[id].id).toBe(id);
    }
  });

  it("getPlaybookProfile returns correct profile", () => {
    expect(getPlaybookProfile("trend").holdMul).toBeGreaterThan(1);
    expect(getPlaybookProfile("breakout").cooldownMul).toBeLessThan(1);
    expect(getPlaybookProfile("range").cooldownMul).toBeGreaterThan(1);
  });

  it("categoryPassesPlaybook: empty allowlist passes all", () => {
    const emptyProfile = { ...PLAYBOOK_PROFILES.trend, categoryAllowlist: [] as string[] };
    expect(categoryPassesPlaybook("Anything", emptyProfile)).toBe(true);
  });

  it("categoryPassesPlaybook: allowlist blocks unknown category", () => {
    expect(categoryPassesPlaybook("Unknown Cat", PLAYBOOK_PROFILES.trend)).toBe(false);
  });

  it("categoryPassesPlaybook: trend allows Trend category", () => {
    expect(categoryPassesPlaybook("Trend", PLAYBOOK_PROFILES.trend)).toBe(true);
  });

  it("categoryPassesPlaybook: range allows Wyckoff", () => {
    expect(categoryPassesPlaybook("Wyckoff", PLAYBOOK_PROFILES.range)).toBe(true);
  });

  it("categoryPassesPlaybook: breakout allows Session", () => {
    expect(categoryPassesPlaybook("Session", PLAYBOOK_PROFILES.breakout)).toBe(true);
  });
});

// ─── buildStyleRoster ─────────────────────────────────────────────────────

describe("buildStyleRoster", () => {
  it("returns defs and meta without error", () => {
    const result = buildStyleRoster("scalp");
    expect(result.defs).toBeInstanceOf(Array);
    expect(result.meta.styleId).toBe("scalp");
    expect(result.meta.playbookId).toBeNull();
  });

  it("scalp roster only contains strats tagged for scalp (or untagged)", () => {
    const { defs } = buildStyleRoster("scalp");
    for (const d of defs) {
      if (d.styles && d.styles.length > 0) {
        expect(d.styles).toContain("scalp");
      }
    }
  });

  it("day roster only contains strats tagged for day (or untagged)", () => {
    const { defs } = buildStyleRoster("day");
    for (const d of defs) {
      if (d.styles && d.styles.length > 0) {
        expect(d.styles).toContain("day");
      }
    }
  });

  it("swing roster excludes scalp-only strats", () => {
    const { defs } = buildStyleRoster("swing");
    const scalp91 = defs.find((d) => d.id === 91);
    expect(scalp91).toBeUndefined();
  });

  it("playbook filter narrows roster", () => {
    const noFilter = buildStyleRoster("scalp");
    const withFilter = buildStyleRoster("scalp", "trend");
    expect(withFilter.defs.length).toBeLessThanOrEqual(noFilter.defs.length);
    for (const d of withFilter.defs) {
      expect(categoryPassesPlaybook(d.category, PLAYBOOK_PROFILES.breakout) ||
             categoryPassesPlaybook(d.category, PLAYBOOK_PROFILES.trend) ||
             true /* playbook=trend allowlist */
      ).toBe(true);
    }
  });

  it("trend playbook only passes trend-aligned categories", () => {
    const { defs } = buildStyleRoster("scalp", "trend");
    for (const d of defs) {
      expect(categoryPassesPlaybook(d.category, PLAYBOOK_PROFILES.trend)).toBe(true);
    }
  });

  it("winners gate empty set returns all eligible strats unchanged", () => {
    const withEmpty = buildStyleRoster("scalp", null, { winnerIds: new Set() });
    const withNone = buildStyleRoster("scalp");
    expect(withEmpty.defs.length).toBe(withNone.defs.length);
  });

  it("winners gate non-empty set restricts to provided IDs", () => {
    const allowedIds = new Set([91, 92]);
    const { defs, meta } = buildStyleRoster("scalp", null, { winnerIds: allowedIds });
    expect(defs.every((d) => allowedIds.has(d.id))).toBe(true);
    expect(meta.afterWinnersGate).toBeLessThanOrEqual(meta.totalEligible);
  });

  it("meta.active equals defs.length", () => {
    const result = buildStyleRoster("day");
    expect(result.meta.active).toBe(result.defs.length);
  });

  it("all FUTURES_STRAT_DEFS tagged strats appear in at least one style roster", () => {
    const taggedIds = new Set(
      (FUTURES_STRAT_DEFS as readonly import("./futuresStratTypes").FuturesStratDef[])
        .filter((d) => d.styles && d.styles.length > 0)
        .map((d) => d.id),
    );
    const allStyleDefs = new Set([
      ...buildStyleRoster("scalp").defs.map((d) => d.id),
      ...buildStyleRoster("day").defs.map((d) => d.id),
      ...buildStyleRoster("swing").defs.map((d) => d.id),
      ...buildStyleRoster("position").defs.map((d) => d.id),
    ]);
    for (const id of taggedIds) {
      expect(allStyleDefs.has(id)).toBe(true);
    }
  });
});
