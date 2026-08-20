import { createElement, type ReactElement } from "react";
import { describe, expect, it } from "vitest";
import { sortBody } from "./AutoSortTable";

/** A <tr> of plain-text <td> cells. */
function row(key: string, cells: string[]): ReactElement {
  return createElement(
    "tr",
    { key },
    cells.map((c, i) => createElement("td", { key: i }, c)),
  );
}

function keysOf(node: unknown): string[] {
  return (node as ReactElement[]).map((r) => String(r.key).replace(/^\.\$/, ""));
}

describe("AutoSortTable row ordering", () => {
  const rows = [
    row("a", ["Alpha", "$9.50", "3"]),
    row("b", ["Bravo", "$10.00", "12"]),
    row("c", ["Charlie", "-$1.00", "7"]),
    row("d", ["Delta", "—", "1"]),
  ];

  it("orders a currency column by value, not by text", () => {
    // The bug this prevents: "$9.50" after "$10.00" because "9" > "1".
    expect(keysOf(sortBody(rows, { index: 1, dir: "desc" }))).toEqual(["b", "a", "c", "d"]);
    expect(keysOf(sortBody(rows, { index: 1, dir: "asc" }))).toEqual(["c", "a", "b", "d"]);
  });

  it("keeps absent values last in BOTH directions", () => {
    // "d" holds an em-dash and stays at the bottom either way.
    expect(keysOf(sortBody(rows, { index: 1, dir: "asc" })).at(-1)).toBe("d");
    expect(keysOf(sortBody(rows, { index: 1, dir: "desc" })).at(-1)).toBe("d");
  });

  it("orders a numeric column numerically", () => {
    expect(keysOf(sortBody(rows, { index: 2, dir: "desc" }))).toEqual(["b", "c", "a", "d"]);
  });

  it("orders a text column alphabetically", () => {
    expect(keysOf(sortBody(rows, { index: 0, dir: "asc" }))).toEqual(["a", "b", "c", "d"]);
  });

  it("returns the original order when nothing is sorted", () => {
    expect(keysOf(sortBody(rows, null))).toEqual(["a", "b", "c", "d"]);
  });

  it("is stable: equal values keep their original order", () => {
    const tied = [row("x", ["same"]), row("y", ["same"]), row("z", ["same"])];
    expect(keysOf(sortBody(tied, { index: 0, dir: "desc" }))).toEqual(["x", "y", "z"]);
  });

  it("leaves rows untouched when the column index is out of range", () => {
    // Rather than throwing, or emptying the table.
    expect(keysOf(sortBody(rows, { index: 99, dir: "desc" }))).toEqual(["a", "b", "c", "d"]);
  });
});
