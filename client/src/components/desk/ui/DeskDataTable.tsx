"use client";

import { useMemo, useState, type ReactNode } from "react";
import { cn } from "./cn";
import { compareSortable, nodeText, parseSortable } from "./sortValue";

export type DeskColumn<T> = {
  id: string;
  header: ReactNode;
  align?: "left" | "right" | "center";
  cell: (row: T, index: number) => ReactNode;
  width?: string;
  /**
   * Explicit sort value for this column.
   *
   * Optional. Without it the column sorts on the text its cell renders, which is
   * right for the overwhelming majority — currency, percentages, counts, names.
   * Declare it where the display is lossy or ordered differently from how it
   * reads: a timestamp shown as "08-09 11:26 PM" sorts correctly as text only
   * within one year, and a chip reading "too few trades" has no numeric meaning
   * at all.
   */
  sortValue?: (row: T, index: number) => number | string | null | undefined;
  /** Opt out — for action buttons, toggles, or anything with no order. */
  sortable?: boolean;
};

type DeskDataTableProps<T> = {
  columns: DeskColumn<T>[];
  rows: T[];
  getRowKey: (row: T, index: number) => string;
  stickyHeader?: boolean;
  zebra?: boolean;
  minWidth?: number;
  empty?: ReactNode;
  rowClassName?: (row: T, index: number) => string | undefined;
  /**
   * Initial ordering. Omit to show the rows in the order the caller supplied —
   * which is frequently meaningful already (newest trade first, leaderboard
   * rank) and should not be silently re-ordered on first paint.
   */
  defaultSort?: { id: string; dir: "asc" | "desc" };
  /** Turn sorting off entirely for one table. */
  sortable?: boolean;
};

export function DeskDataTable<T>({
  columns,
  rows,
  getRowKey,
  stickyHeader = true,
  zebra = true,
  minWidth,
  empty,
  rowClassName,
  defaultSort,
  sortable = true,
}: DeskDataTableProps<T>) {
  const [sort, setSort] = useState<{ id: string; dir: "asc" | "desc" } | null>(defaultSort ?? null);

  const sortedRows = useMemo(() => {
    if (!sortable || !sort) return rows;
    const col = columns.find((c) => c.id === sort.id);
    if (!col || col.sortable === false) return rows;
    // Decorate-sort-undecorate: the sort value of a cell can be expensive to
    // derive (it renders the node and walks it), and a comparator would
    // recompute it O(n log n) times instead of once per row.
    const keyed = rows.map((row, i) => ({
      row,
      i,
      key:
        col.sortValue !== undefined
          ? (col.sortValue(row, i) ?? null)
          : parseSortable(nodeText(col.cell(row, i))),
    }));
    keyed.sort((a, b) => {
      const c = compareSortable(a.key ?? null, b.key ?? null, sort.dir);
      // Stable: equal keys keep the caller's original order rather than
      // shuffling on every re-render, which these pages do every 15-20s.
      return c !== 0 ? c : a.i - b.i;
    });
    return keyed.map((k) => k.row);
  }, [rows, columns, sort, sortable]);

  if (rows.length === 0 && empty) {
    return <>{empty}</>;
  }

  const toggle = (id: string) => {
    setSort((cur) => {
      // First click on a column sorts DESCENDING. On a trading board the
      // interesting end is almost always the top — biggest P&L, most trades —
      // and ascending-first would make every first click the wrong one.
      if (!cur || cur.id !== id) return { id, dir: "desc" };
      if (cur.dir === "desc") return { id, dir: "asc" };
      return null; // third click restores the caller's ordering
    });
  };

  return (
    <div style={{ overflowX: "auto", borderRadius: "var(--desk-radius-chip)", border: "1px solid var(--desk-outline)" }}>
      <table
        className="desk-data-table"
        style={{ width: "100%", borderCollapse: "collapse", minWidth: minWidth ?? 480 }}
      >
        <thead>
          <tr
            style={{
              background: "var(--desk-surface-container)",
              position: stickyHeader ? "sticky" : undefined,
              top: stickyHeader ? 0 : undefined,
              zIndex: stickyHeader ? 1 : undefined,
            }}
          >
            {columns.map((col) => {
              const canSort = sortable && col.sortable !== false;
              const active = sort?.id === col.id;
              const align = col.align ?? "left";
              return (
                <th
                  key={col.id}
                  className="desk-label-md"
                  aria-sort={active ? (sort?.dir === "asc" ? "ascending" : "descending") : "none"}
                  style={{
                    padding: "10px 14px",
                    textAlign: align,
                    borderBottom: "1px solid var(--desk-outline)",
                    width: col.width,
                    fontWeight: 500,
                    whiteSpace: "nowrap",
                  }}
                >
                  {canSort ? (
                    <button
                      type="button"
                      onClick={() => toggle(col.id)}
                      title={active ? `Sorted ${sort?.dir === "asc" ? "ascending" : "descending"} — click to change` : "Click to sort"}
                      style={{
                        display: "inline-flex",
                        alignItems: "center",
                        gap: 4,
                        // Keep the header aligned with its column: a right-aligned
                        // number column whose header drifts left is harder to scan
                        // than one that never gained a sort control.
                        justifyContent: align === "right" ? "flex-end" : align === "center" ? "center" : "flex-start",
                        width: "100%",
                        background: "transparent",
                        border: "none",
                        padding: 0,
                        font: "inherit",
                        cursor: "pointer",
                        color: active ? "var(--desk-primary)" : "inherit",
                        fontWeight: active ? 700 : 500,
                      }}
                    >
                      {col.header}
                      <span aria-hidden style={{ fontSize: 8, lineHeight: 1, opacity: active ? 1 : 0.55 }}>
                        {active ? (sort?.dir === "desc" ? "▼" : "▲") : "▲▼"}
                      </span>
                    </button>
                  ) : (
                    col.header
                  )}
                </th>
              );
            })}
          </tr>
        </thead>
        <tbody>
          {sortedRows.map((row, i) => (
            <tr
              key={getRowKey(row, i)}
              className={cn(rowClassName?.(row, i))}
              style={{
                background: zebra && i % 2 === 1 ? "var(--desk-surface-container)" : "var(--desk-surface)",
              }}
            >
              {columns.map((col) => (
                <td
                  key={col.id}
                  className="desk-body-md desk-mono"
                  style={{
                    padding: "12px 14px",
                    textAlign: col.align ?? "left",
                    borderBottom: "1px solid var(--desk-outline-variant)",
                    verticalAlign: "middle",
                    fontFamily: col.align === "right" ? "var(--desk-font-mono)" : undefined,
                  }}
                >
                  {col.cell(row, i)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
