import { describe, expect, it } from "vitest";
import { compareSortable, parseSortable } from "./sortValue";

describe("parseSortable", () => {
  it("reads currency as a number, not text", () => {
    // The single most common way a sortable table lies: "$9.50" sorts AFTER
    // "$10.00" as text, because "9" > "1".
    expect(parseSortable("$9.50")).toBe(9.5);
    expect(parseSortable("$10.00")).toBe(10);
    expect(parseSortable("-$1,234.56")).toBe(-1234.56);
    expect(parseSortable("+$0.0299")).toBe(0.0299);
  });

  it("reads percentages, multipliers and thousands separators", () => {
    expect(parseSortable("+11.29%")).toBe(11.29);
    expect(parseSortable("-60.2%")).toBe(-60.2);
    expect(parseSortable("3x")).toBe(3);
    expect(parseSortable("1,234")).toBe(1234);
  });

  it("reads reward:risk ratios by their magnitude", () => {
    // As text "1:10" sorts before "1:2".
    expect(parseSortable("1:6.00")).toBe(6);
    expect(parseSortable("1:10")).toBe(10);
    expect(parseSortable("1:2")).toBe(2);
  });

  it("normalises durations to seconds so units compare", () => {
    expect(parseSortable("45s ago")).toBe(45);
    expect(parseSortable("3m ago")).toBe(180);
    expect(parseSortable("2h")).toBe(7200);
    expect(parseSortable("1d")).toBe(86400);
  });

  it("treats placeholders as absent rather than as zero", () => {
    for (const p of ["—", "-", "", "  ", "n/a", "N/A", "none"]) {
      expect(parseSortable(p)).toBeNull();
    }
  });

  it("falls back to text for genuinely non-numeric cells", () => {
    expect(parseSortable("BTCUSD")).toBe("btcusd");
    expect(parseSortable("too few trades")).toBe("too few trades");
  });
});

describe("compareSortable", () => {
  it("orders numbers numerically in both directions", () => {
    expect(compareSortable(9.5, 10, "asc")).toBeLessThan(0);
    expect(compareSortable(9.5, 10, "desc")).toBeGreaterThan(0);
  });

  it("sorts absent values LAST regardless of direction", () => {
    // An ascending sort whose first twenty rows are em-dashes has told the
    // reader nothing, so null loses both ways.
    expect(compareSortable(null, 5, "asc")).toBeGreaterThan(0);
    expect(compareSortable(null, 5, "desc")).toBeGreaterThan(0);
    expect(compareSortable(5, null, "asc")).toBeLessThan(0);
    expect(compareSortable(5, null, "desc")).toBeLessThan(0);
    expect(compareSortable(null, null, "asc")).toBe(0);
  });

  it("sorts text naturally, so item2 precedes item10", () => {
    expect(compareSortable("item2", "item10", "asc")).toBeLessThan(0);
  });

  it("puts the biggest value first on a descending sort", () => {
    const vals = [3, 1, 10, 2].sort((a, b) => compareSortable(a, b, "desc"));
    expect(vals).toEqual([10, 3, 2, 1]);
  });

  it("keeps a full column of mixed values coherent", () => {
    const col = ["$10.00", "—", "$9.50", "-$1.00", "n/a", "$100.00"];
    const sorted = col
      .map((t) => ({ t, v: parseSortable(t) }))
      .sort((a, b) => compareSortable(a.v, b.v, "desc"))
      .map((x) => x.t);
    expect(sorted).toEqual(["$100.00", "$10.00", "$9.50", "-$1.00", "—", "n/a"]);
  });
});
