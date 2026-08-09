import { describe, expect, it } from "vitest";
import { fmtIST, fmtISTSeconds, fmtISTClock, fmtISTFull, istDayKey, fmtISTDayLabel } from "./istTime";

describe("istTime", () => {
  it("adds 5:30 to a mid-day instant", () => {
    expect(fmtIST("2026-08-09T09:15:00Z")).toBe("08-09 02:45 PM");
    expect(fmtISTSeconds("2026-08-09T09:15:07Z")).toBe("08-09 02:45:07 PM");
    expect(fmtISTClock("2026-08-09T09:15:07Z")).toBe("02:45:07 PM");
    expect(fmtISTFull("2026-08-09T09:15:07Z")).toBe("2026-08-09 02:45:07 PM");
  });

  // The case that motivated the change. A crypto desk trades through the night,
  // and everything closed between 18:30 and 24:00 UTC belongs to the NEXT day in
  // the operator's calendar. Under UTC rendering those rows read as yesterday.
  it("rolls to the next IST day for late-evening UTC instants", () => {
    expect(fmtIST("2026-08-09T21:30:00Z")).toBe("08-10 03:00 AM");
    expect(istDayKey("2026-08-09T21:30:00Z")).toBe("2026-08-10");
  });

  it("keeps the same IST day just before the boundary", () => {
    expect(istDayKey("2026-08-09T18:29:00Z")).toBe("2026-08-09");
    expect(istDayKey("2026-08-09T18:30:00Z")).toBe("2026-08-10");
  });

  it("crosses month and year boundaries", () => {
    expect(fmtIST("2026-12-31T20:00:00Z")).toBe("01-01 01:30 AM");
    expect(fmtISTFull("2026-12-31T20:00:00Z")).toBe("2027-01-01 01:30:00 AM");
  });

  // A date-only string carries no instant. Routing it through a Date would peg
  // it to midnight UTC and any further shifting would move the calendar day, so
  // the label formatter must not parse it as a time at all.
  it("does not shift a date-only day label", () => {
    expect(fmtISTDayLabel("2026-08-09")).toBe("2026-08-09");
    expect(fmtISTDayLabel("2026-01-01")).toBe("2026-01-01");
  });

  // Where 12-hour clocks are usually got wrong: hour 0 and hour 12 both map to
  // 12, and the meridiem flips at noon, not at 1 PM.
  it("handles midnight and noon", () => {
    expect(fmtIST("2026-08-09T18:30:00Z")).toBe("08-10 12:00 AM"); // IST midnight
    expect(fmtIST("2026-08-09T18:31:00Z")).toBe("08-10 12:01 AM");
    expect(fmtIST("2026-08-09T06:30:00Z")).toBe("08-09 12:00 PM"); // IST noon
    expect(fmtIST("2026-08-09T06:29:00Z")).toBe("08-09 11:59 AM");
    expect(fmtIST("2026-08-09T07:30:00Z")).toBe("08-09 01:00 PM");
  });

  it("pads the hour so table columns stay aligned", () => {
    expect(fmtIST("2026-08-09T02:35:00Z")).toBe("08-09 08:05 AM");
    expect(fmtIST("2026-08-09T17:56:00Z")).toBe("08-09 11:26 PM");
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
    expect(fmtIST(ms)).toBe("08-09 02:45 PM");
    expect(fmtIST(new Date(ms))).toBe("08-09 02:45 PM");
  });
});
