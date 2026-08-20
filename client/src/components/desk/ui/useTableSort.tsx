"use client";

import { useMemo, useState, type CSSProperties, type ReactNode } from "react";
import { compareSortable, parseSortable } from "./sortValue";

/**
 * Sorting for hand-rolled <table> markup.
 *
 * DeskDataTable sorts itself. Dozens of tables in this app predate it and build
 * their own thead/tbody, and those cannot be fixed by editing one component —
 * so this is the smallest thing that makes each of them sortable: a hook that
 * orders the array, and a <SortableTh> that renders the control.
 *
 * Deliberately NOT a DOM-level enhancer that reorders <tr> nodes after render.
 * These pages poll every 15-20 seconds and React replaces the rows on each
 * refresh, which would silently undo a DOM sort roughly four times a minute —
 * a control that appears to work and quietly stops.
 */

export type SortDir = "asc" | "desc";
export type TableSort = { key: string; dir: SortDir } | null;

/** How to read the sort value for a column key. Values may be null (absent). */
export type SortAccessor<T> = (row: T) => number | string | null | undefined;

export function useTableSort<T = unknown>(
  /**
   * Optional. Supply accessors to sort an ARRAY yourself via sortRows; omit
   * them when pairing with <SortableRows>, which sorts rendered rows by their
   * cell text and needs only the shared sort state.
   */
  accessors: Record<string, SortAccessor<T>> = {},
  initial: TableSort = null,
) {
  const [sort, setSort] = useState<TableSort>(initial);

  /** First click descending — on these boards the interesting end is the top. */
  const toggle = (key: string) =>
    setSort((cur) => {
      if (!cur || cur.key !== key) return { key, dir: "desc" };
      if (cur.dir === "desc") return { key, dir: "asc" };
      return null; // third click restores the original order
    });

  const sortRows = useMemo(
    () => (rows: T[]) => {
      if (!sort) return rows;
      const get = accessors[sort.key];
      if (!get) return rows;
      const keyed = rows.map((row, i) => {
        const raw = get(row);
        const v = typeof raw === "string" ? parseSortable(raw) : (raw ?? null);
        return { row, i, v };
      });
      keyed.sort((a, b) => {
        const c = compareSortable(a.v, b.v, sort.dir);
        return c !== 0 ? c : a.i - b.i; // stable
      });
      return keyed.map((k) => k.row);
    },
    [sort, accessors],
  );

  return { sort, toggle, sortRows };
}

/**
 * A <th> carrying a sort control, styled to inherit whatever the surrounding
 * table already uses so it can be dropped into existing markup.
 */
export function SortableTh({
  label,
  sortKey,
  sort,
  onSort,
  align = "left",
  style,
  children,
}: {
  label?: ReactNode;
  sortKey: string;
  sort: TableSort;
  onSort: (key: string) => void;
  align?: "left" | "right" | "center";
  style?: CSSProperties;
  children?: ReactNode;
}) {
  const active = sort?.key === sortKey;
  return (
    <th
      aria-sort={active ? (sort?.dir === "asc" ? "ascending" : "descending") : "none"}
      style={{ textAlign: align, whiteSpace: "nowrap", ...style }}
    >
      <button
        type="button"
        onClick={() => onSort(sortKey)}
        title={active ? `Sorted ${sort?.dir === "asc" ? "ascending" : "descending"} — click to change` : "Click to sort"}
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 4,
          justifyContent: align === "right" ? "flex-end" : align === "center" ? "center" : "flex-start",
          width: "100%",
          background: "transparent",
          border: "none",
          padding: 0,
          font: "inherit",
          color: active ? "var(--desk-primary, #d4a017)" : "inherit",
          fontWeight: active ? 700 : undefined,
          cursor: "pointer",
        }}
      >
        {children ?? label}
        <span aria-hidden style={{ fontSize: 8, lineHeight: 1, opacity: active ? 1 : 0.55 }}>
          {active ? (sort?.dir === "desc" ? "▼" : "▲") : "▲▼"}
        </span>
      </button>
    </th>
  );
}
