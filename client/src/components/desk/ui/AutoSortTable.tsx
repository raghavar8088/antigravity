"use client";

import {
  Children,
  cloneElement,
  isValidElement,
  useMemo,
  useState,
  type ReactElement,
  type ReactNode,
} from "react";
import { compareSortable, nodeText, parseSortable } from "./sortValue";

/**
 * Makes an existing hand-written <table> sortable by wrapping it.
 *
 * Dozens of tables in this app predate DeskDataTable and build their own
 * thead/tbody with bespoke styling. Rewriting each into a column model would be
 * a large, risky edit per file for no visual gain, and adding a comparator per
 * column is the scheme that ends up working on the three tables somebody
 * remembered to wire up.
 *
 * So this takes the table as CHILDREN and rewrites it structurally: every <th>
 * in the head gains a sort control, and the body's <tr> elements are reordered
 * by the text of the corresponding cell. The caller's markup, styles and class
 * names are preserved by cloning rather than replacing.
 *
 * Reordering React ELEMENTS, never the DOM. These pages refresh every 15-20
 * seconds and React rebuilds the rows each time, so a DOM-level sort would be
 * silently undone about four times a minute — a control that appears to work and
 * does not.
 *
 * Two limitations worth stating rather than discovering:
 *   - Body rows should be keyed. React reuses unkeyed children by position, so a
 *     reordered list can show the right values against the wrong row.
 *   - Cells spanning columns (colSpan) shift the mapping between a header and
 *     its cells. Tables using them should sort with the explicit components
 *     instead.
 */
export function AutoSortTable({
  children,
  defaultSort,
}: {
  children: ReactNode;
  /** Optional initial ordering, by column index. */
  defaultSort?: { index: number; dir: "asc" | "desc" };
}) {
  const [sort, setSort] = useState<{ index: number; dir: "asc" | "desc" } | null>(defaultSort ?? null);

  const toggle = (index: number) =>
    setSort((cur) => {
      // First click DESCENDING: on these boards the interesting end is the top.
      if (!cur || cur.index !== index) return { index, dir: "desc" };
      if (cur.dir === "desc") return { index, dir: "asc" };
      return null; // third click restores the caller's ordering
    });

  const rendered = useMemo(() => transform(children, sort, toggle), [children, sort]);
  return <>{rendered}</>;
}

type AnyProps = { children?: ReactNode; [k: string]: unknown };

/** Recursively find the thead/tbody inside whatever wrappers the caller used. */
function transform(
  node: ReactNode,
  sort: { index: number; dir: "asc" | "desc" } | null,
  toggle: (i: number) => void,
): ReactNode {
  if (Array.isArray(node)) return Children.map(node, (n) => transform(n, sort, toggle));
  if (!isValidElement(node)) return node;

  const el = node as ReactElement<AnyProps>;
  const type = el.type;

  if (type === "thead") {
    return cloneElement(el, undefined, transformHead(el.props.children, sort, toggle));
  }
  if (type === "tbody") {
    return cloneElement(el, undefined, sortBody(el.props.children, sort));
  }
  if (el.props?.children !== undefined) {
    return cloneElement(el, undefined, transform(el.props.children, sort, toggle));
  }
  return el;
}

/** Give every <th> in the header row a sort button, preserving its styling. */
function transformHead(
  node: ReactNode,
  sort: { index: number; dir: "asc" | "desc" } | null,
  toggle: (i: number) => void,
): ReactNode {
  return Children.map(node, (row) => {
    if (!isValidElement(row)) return row;
    const rowEl = row as ReactElement<AnyProps>;
    if (rowEl.type !== "tr") {
      return cloneElement(rowEl, undefined, transformHead(rowEl.props.children, sort, toggle));
    }
    let i = -1;
    const cells = Children.map(rowEl.props.children, (cell) => {
      if (!isValidElement(cell)) return cell;
      const cellEl = cell as ReactElement<AnyProps>;
      if (cellEl.type !== "th") return cell;
      i += 1;
      const index = i;
      const active = sort?.index === index;
      const style = (cellEl.props.style ?? {}) as { textAlign?: string };
      const align = style.textAlign === "right" ? "flex-end" : style.textAlign === "center" ? "center" : "flex-start";
      return cloneElement(
        cellEl,
        { "aria-sort": active ? (sort?.dir === "asc" ? "ascending" : "descending") : "none" } as AnyProps,
        <button
          type="button"
          onClick={() => toggle(index)}
          title={active ? `Sorted ${sort?.dir === "asc" ? "ascending" : "descending"} — click to change` : "Click to sort"}
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 4,
            justifyContent: align,
            width: "100%",
            background: "transparent",
            border: "none",
            padding: 0,
            font: "inherit",
            color: active ? "var(--desk-primary, #d4a017)" : "inherit",
            fontWeight: active ? 700 : undefined,
            cursor: "pointer",
            textAlign: "inherit",
          }}
        >
          {cellEl.props.children}
          <span aria-hidden style={{ fontSize: 8, lineHeight: 1, opacity: active ? 1 : 0.5 }}>
            {active ? (sort?.dir === "desc" ? "▼" : "▲") : "▲▼"}
          </span>
        </button>,
      );
    });
    return cloneElement(rowEl, undefined, cells);
  });
}

/**
 * Reorder body rows by the text of the sorted column's cell.
 *
 * Exported for tests: the reordering is the part that can silently do nothing —
 * rows that are Fragments rather than <tr>, or a column index past the end —
 * and those failures are invisible in a rendered page.
 */
export function sortBody(node: ReactNode, sort: { index: number; dir: "asc" | "desc" } | null): ReactNode {
  const rows = Children.toArray(node);
  if (!sort) return rows;
  const keyed = rows.map((row, i) => {
    if (!isValidElement(row)) return { row, i, v: null };
    const cells = Children.toArray((row as ReactElement<AnyProps>).props.children).filter(isValidElement);
    const cell = cells[sort.index];
    return { row, i, v: cell ? parseSortable(nodeText(cell)) : null };
  });
  keyed.sort((a, b) => {
    const c = compareSortable(a.v, b.v, sort.dir);
    return c !== 0 ? c : a.i - b.i; // stable
  });
  return keyed.map((k) => k.row);
}
