import { describe, expect, it } from "vitest";
import { fmtIST, fmtISTSeconds, fmtISTClock, fmtISTFull, istDayKey, fmtISTDayLabel } from "./istTime";

describe("istTime", () => {
  it("adds 5:30 to a mid-day instant", () => {
    expect(fmtIST("2026-08-09T09:15:00Z")).toBe("08-09 14:45");
    expect(fmtISTSeconds("2026-08-09T09:15:07Z")).toBe("08-09 14:45:07");
    expect(fmtISTClock("2026-08-09T09:15:07Z")).toBe("14:45:07");
    expect(fmtISTFull("2026-08-09T09:15:07Z")).toBe("2026-08-09 14:45:07");
  });

  // The case that motivated the change. A crypto desk trades through the night,
  // and everything closed between 18:30 and 24:00 UTC belongs to the NEXT day in
  // the operator's calendar. Under UTC rendering those rows read as yesterday.
  it("rolls to the next IST day for late-evening UTC instants", () => {
    expect(fmtIST("2026-08-09T21:30:00Z")).toBe("08-10 03:00");
    expect(istDayKey("2026-08-09T21:30:00Z")).toBe("2026-08-10");
  });

  it("keeps the same IST day just before the boundary", () => {
    expect(istDayKey("2026-08-09T18:29:00Z")).toBe("2026-08-09");
    expect(istDayKey("2026-08-09T18:30:00Z")).toBe("2026-08-10");
  });

  it("crosses month and year boundaries", () => {
    expect(fmtIST("2026-12-31T20:00:00Z")).toBe("01-01 01:30");
    expect(fmtISTFull("2026-12-31T20:00:00Z")).toBe("2027-01-01 01:30:00");
  });

  // A date-only string carries no instant. Routing it through a Date would peg
  // it to midnight UTC and any further shifting would move the calendar day, so
  // the label formatter must not parse it as a time at all.
  it("does not shift a date-only day label", () => {
    expect(fmtISTDayLabel("2026-08-09")).toBe("2026-08-09");
    expect(fmtISTDayLabel("2026-01-01")).toBe("2026-01-01");
  });

  it("returns the fallback for missing or unparseable input", () => {
    expect(fmtIST(null)).toBe("—");
    expect(fmtIST(undefined)).toBe("—");
    expect(fmtIST("")).toBe("—");
    expect(fmtIST("not-a-date")).toBe("—");
    expect(istDayKey(null)).toBe("");
    expect(fmtISTDayLabel(null)).toBe("—");
  });

  it("accepts epoch millis and Date as well as ISO strings", () => {
    const ms = Date.UTC(2026, 7, 9, 9, 15, 0);
    expect(fmtIST(ms)).toBe("08-09 14:45");
    expect(fmtIST(new Date(ms))).toBe("08-09 14:45");
  });
});
