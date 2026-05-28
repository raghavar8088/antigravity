import { describe, expect, it } from "vitest";
import {
  paperDeskRequiresMomentumAlignment,
  paperEnsureAllowsRelaxedSignalConfirm,
  paperEnsureEffectiveThresholdDrop,
  paperEnsureThresholdDrop,
  paperQuietEntryBoost,
  paperThroughputRecentOpenCount,
  paperThroughputTopUpDue,
} from "./useBTCFuturesScalperEngine";

describe("paperEnsureThresholdDrop", () => {
  const mounted = 1_000_000;

  it("returns 0 when disabled", () => {
    expect(paperEnsureThresholdDrop(false, mounted + 20 * 60_000, mounted, 0)).toBe(0);
  });

  it("ramps down threshold over time for quiet strats", () => {
    expect(paperEnsureThresholdDrop(true, mounted + 3 * 60_000, mounted, 0)).toBe(6);
    expect(paperEnsureThresholdDrop(true, mounted + 6 * 60_000, mounted, 0)).toBe(10);
    expect(paperEnsureThresholdDrop(true, mounted + 11 * 60_000, mounted, 0)).toBe(12);
  });

  it("stops dropping once strat has 5+ trades", () => {
    expect(paperEnsureThresholdDrop(true, mounted + 20 * 60_000, mounted, 5)).toBe(0);
  });

  it("halves drop for strats that already have samples", () => {
    expect(paperEnsureThresholdDrop(true, mounted + 11 * 60_000, mounted, 2)).toBe(6);
  });
});

describe("paperQuietEntryBoost", () => {
  const now = 1_000_000;

  it("returns 0 when positions are open", () => {
    expect(paperQuietEntryBoost(true, now, now - 600_000, 1)).toBe(0);
  });

  it("ramps boost when desk is idle after last close", () => {
    expect(paperQuietEntryBoost(true, now, now - 3 * 60_000, 0)).toBe(3);
    expect(paperQuietEntryBoost(true, now, now - 6 * 60_000, 0)).toBe(4);
    expect(paperQuietEntryBoost(true, now, now - 11 * 60_000, 0)).toBe(6);
  });

  it("can use mount time as the idle clock before the first close", () => {
    const mountedAt = now - 6 * 60_000;
    expect(paperQuietEntryBoost(true, now, mountedAt, 0)).toBe(4);
  });
});

describe("paperEnsureEffectiveThresholdDrop", () => {
  it("never lowers production paper thresholds", () => {
    expect(paperEnsureEffectiveThresholdDrop(true, 12, 0)).toBe(0);
    expect(paperEnsureEffectiveThresholdDrop(true, 6, 9)).toBe(0);
    expect(paperEnsureEffectiveThresholdDrop(true, 12, 10)).toBe(0);
    expect(paperEnsureEffectiveThresholdDrop(false, 12, 100)).toBe(0);
  });
});

describe("paperEnsureAllowsRelaxedSignalConfirm", () => {
  it("never relaxes confirmation for production paper seed data collection", () => {
    expect(paperEnsureAllowsRelaxedSignalConfirm(true, true, 0, 6)).toBe(false);
    expect(paperEnsureAllowsRelaxedSignalConfirm(true, true, 9, 10)).toBe(false);
    expect(paperEnsureAllowsRelaxedSignalConfirm(true, true, 10, 10)).toBe(false);
  });

  it("does not alter non-profit or disabled paper ensure paths", () => {
    expect(paperEnsureAllowsRelaxedSignalConfirm(false, true, 0, 6)).toBe(false);
    expect(paperEnsureAllowsRelaxedSignalConfirm(true, false, 0, 6)).toBe(false);
    expect(paperEnsureAllowsRelaxedSignalConfirm(true, true, 0, 0)).toBe(false);
  });
});

describe("paper throughput floor helpers", () => {
  const now = 1_000_000;
  const windowMs = 10 * 60_000;

  it("counts opens inside the rolling window", () => {
    expect(
      paperThroughputRecentOpenCount(now, [
        now - 30_000,
        now - 9 * 60_000,
        now - 11 * 60_000,
        now + 1_000,
      ]),
    ).toBe(2);
  });

  it("paces top-ups to one slot per target interval", () => {
    const due = paperThroughputTopUpDue(true, now, [now - 2 * 60_000], now - 61_000, 10, windowMs);
    expect(due).toMatchObject({ due: true, recentOpenCount: 1, deficit: 9, minIntervalMs: 60_000 });

    const tooSoon = paperThroughputTopUpDue(true, now, [now - 2 * 60_000], now - 30_000, 10, windowMs);
    expect(tooSoon.due).toBe(false);
  });

  it("does not top up when disabled or already at target", () => {
    expect(paperThroughputTopUpDue(false, now, [], 0, 10, windowMs).due).toBe(false);
    const tenOpens = Array.from({ length: 10 }, (_, i) => now - i * 30_000);
    expect(paperThroughputTopUpDue(true, now, tenOpens, now - 60_000, 10, windowMs).due).toBe(false);
  });
});

describe("paperDeskRequiresMomentumAlignment", () => {
  it("requires momentum alignment for trend and breakout strategies", () => {
    expect(
      paperDeskRequiresMomentumAlignment({
        category: "Trend",
        name: "Trend_Continuation_Long",
        templateFamily: "TREND_CONT",
        playbooks: ["trend"],
      }),
    ).toBe(true);
    expect(
      paperDeskRequiresMomentumAlignment({
        category: "Breakout",
        name: "Breakout_Long",
        templateFamily: "BREAKOUT",
        playbooks: ["breakout"],
      }),
    ).toBe(true);
  });

  it("does not block mean-reversion/range signals on opposite short-term momentum", () => {
    expect(
      paperDeskRequiresMomentumAlignment({
        category: "Wyckoff",
        name: "Wyckoff_Spring_Long",
        templateFamily: "WYCKOFF",
        playbooks: ["range"],
      }),
    ).toBe(false);
    expect(
      paperDeskRequiresMomentumAlignment({
        category: "Mean Reversion",
        name: "VWAP_Revert_Long",
        templateFamily: "VWAP_REVERT",
        playbooks: ["range"],
      }),
    ).toBe(false);
  });
});
