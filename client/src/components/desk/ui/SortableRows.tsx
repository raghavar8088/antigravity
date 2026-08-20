"use client";

import { Children, isValidElement, useMemo, type ReactElement, type ReactNode } from "react";
import { compareSortable, nodeText, parseSortable } from "./sortValue";
import type { TableSort } from "./useTableSort";

/**
 * Sorts already-rendered <tr> elements by the text in one of their cells.
 *
 * For the many hand-rolled tables that predate DeskDataTable. Those build their
 * own rows, so there is no column definition to hang a comparator on — but the
 * rows ARE React elements, and reordering an array of elements is something
 * React handles correctly. Reordering the DOM after render is not: these pages
 * refresh every 15-20 seconds and React rebuilds the rows each time, which would
 * quietly undo a DOM sort about four times a minute.
 *
 * Works off the same text extraction as DeskDataTable, so "$9.50" still sorts
 * below "$10.00" and an em-dash still counts as absent rather than as zero.
 *
 * Rows must be keyed, as React requires for any reordered list — without keys
 * React reuses elements by position and the sorted table shows the right values
 * against the wrong rows.
 */
export function SortableRows({
  children,
  sort,
  columnIndex,
}: {
  children: ReactNode;
  sort: TableSort;
  /** Maps a sort key to the cell position it refers to. */
  columnIndex: Record<string, number>;
}) {
  const rows = useMemo(() => Children.toArray(children).filter(isValidElement), [children]);

  const sorted = useMemo(() => {
    if (!sort) return rows;
    const idx = columnIndex[sort.key];
    if (idx === undefined) return rows;
    const keyed = rows.map((row, i) => {
      const cells = Children.toArray(
        (row as ReactElement<{ children?: ReactNode }>).props.children,
      ).filter(isValidElement);
      const cell = cells[idx];
      return { row, i, v: cell ? parseSortable(nodeText(cell)) : null };
    });
    keyed.sort((a, b) => {
      const c = compareSortable(a.v, b.v, sort.dir);
      return c !== 0 ? c : a.i - b.i; // stable
    });
    return keyed.map((k) => k.row);
  }, [rows, sort, columnIndex]);

  return <>{sorted}</>;
}
