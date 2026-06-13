import { describe, expect, it } from "vitest";
import { coerceEpochMs, toMinuteUtcChartTime, toUtcChartTime } from "./utils/chartTime";

describe("chartTime", () => {
  it("coerces epoch ms, seconds, and ISO strings", () => {
    expect(coerceEpochMs(1_700_000_000_000)).toBe(1_700_000_000_000);
    expect(coerceEpochMs(1_700_000_000)).toBe(1_700_000_000_000);
    expect(coerceEpochMs("2024-06-01T12:00:00.000Z")).toBe(Date.parse("2024-06-01T12:00:00.000Z"));
    expect(coerceEpochMs(0)).toBe(0);
    expect(coerceEpochMs("bad")).toBe(0);
  });

  it("returns null for invalid chart times", () => {
    expect(toUtcChartTime(0)).toBeNull();
    expect(toUtcChartTime("bad")).toBeNull();
    expect(toUtcChartTime(1_700_000_000_000)).toBe(1_700_000_000);
  });

  it("aligns marker times to minute boundaries", () => {
    expect(toMinuteUtcChartTime(1_700_000_045_000)).toBe(1_700_000_040);
  });
});
