import { isValidElement, type ReactNode } from "react";

/**
 * Turning a rendered cell back into something sortable.
 *
 * Columns render ReactNode — chips, coloured spans, formatted currency — so the
 * value a reader sorts by is not a field on the row, it is whatever the cell
 * drew. Extracting it from the node tree is what lets EVERY column be sortable
 * without each table declaring a comparator per column, which is the difference
 * between sorting working everywhere and working on the three tables somebody
 * remembered to wire up.
 *
 * Columns can still declare `sortValue` explicitly, and should wherever the
 * displayed text is ambiguous or lossy — a relative "3m ago" sorts as the number
 * 3, which is wrong next to "45s ago".
 */

/** Recursively collect the visible text of a rendered cell. */
export function nodeText(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(nodeText).join(" ");
  if (isValidElement(node)) {
    return nodeText((node.props as { children?: ReactNode }).children);
  }
  return "";
}

/**
 * Placeholders mean "no value", not a small one. They sort to the BOTTOM in both
 * directions rather than competing with real numbers — an ascending sort whose
 * first twenty rows are em-dashes has told the reader nothing.
 */
const PLACEHOLDERS = new Set(["—", "–", "-", "", "n/a", "na", "none", "null", "undefined"]);

/**
 * Parse a displayed string into something orderable.
 *
 * Returns null for "no value". Numbers come back as numbers so that $9.50 sorts
 * below $10.00 — comparing those as text puts "$9.50" after "$10.00", which is
 * the single most common way a sortable table lies.
 */
export function parseSortable(text: string): number | string | null {
  const t = text.trim();
  if (!t || PLACEHOLDERS.has(t.toLowerCase())) return null;

  // Ratios: "1:6.00" -> 6. Sorting these as text orders 1:10 before 1:2.
  const ratio = /^1\s*:\s*(-?[\d.]+)$/.exec(t);
  if (ratio?.[1]) {
    const n = Number(ratio[1]);
    if (Number.isFinite(n)) return n;
  }

  // Durations: "45s ago", "3m ago", "2h" -> seconds, so the units compare.
  const dur = /^(-?[\d.]+)\s*(s|m|h|d)\b/i.exec(t);
  if (dur?.[1] && dur?.[2] && !/[$%]/.test(t)) {
    const n = Number(dur[1]);
    const mult = { s: 1, m: 60, h: 3600, d: 86400 }[dur[2].toLowerCase() as "s" | "m" | "h" | "d"];
    if (Number.isFinite(n) && mult) return n * mult;
  }

  // Currency, percentages, multipliers, thousands separators, leading sign.
  const cleaned = t
    .replace(/[$,%×x\s]/gi, "")
    .replace(/^\+/, "")
    .replace(/^\((.*)\)$/, "-$1"); // (1.23) accounting negatives
  if (cleaned !== "" && cleaned !== "-" && Number.isFinite(Number(cleaned))) {
    return Number(cleaned);
  }

  return t.toLowerCase();
}

/**
 * Compare two extracted values. Nulls always sort last, whichever direction the
 * reader picked, so an empty column never buries the populated rows.
 */
export function compareSortable(a: number | string | null, b: number | string | null, dir: "asc" | "desc"): number {
  if (a === null && b === null) return 0;
  if (a === null) return 1;
  if (b === null) return -1;
  let cmp: number;
  if (typeof a === "number" && typeof b === "number") {
    cmp = a - b;
  } else {
    cmp = String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: "base" });
  }
  return dir === "asc" ? cmp : -cmp;
}
