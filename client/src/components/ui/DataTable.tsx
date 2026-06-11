"use client";

import { useMemo, useState, type ReactNode } from "react";
import { EmptyState } from "./EmptyState";
import { Skeleton } from "./Skeleton";
import { TextField } from "./FormControls";
import { cn } from "./cn";

export type DataTableColumn<T> = {
  id: string;
  header: ReactNode;
  cell: (row: T, index: number) => ReactNode;
  align?: "left" | "right" | "center";
  sortable?: boolean;
  sortValue?: (row: T) => string | number;
  width?: string;
};

type SortDir = "asc" | "desc";

export function DataTable<T>({
  columns,
  rows,
  getRowKey,
  loading = false,
  searchable = false,
  searchPlaceholder = "Search…",
  searchFilter,
  pageSize: initialPageSize = 25,
  pageSizeOptions = [25, 50, 100],
  emptyTitle = "No data",
  emptySubtitle,
  density = "comfortable",
  stickyHeader = true,
}: {
  columns: DataTableColumn<T>[];
  rows: T[];
  getRowKey: (row: T, index: number) => string;
  loading?: boolean;
  searchable?: boolean;
  searchPlaceholder?: string;
  searchFilter?: (row: T, query: string) => boolean;
  pageSize?: number;
  pageSizeOptions?: number[];
  emptyTitle?: string;
  emptySubtitle?: string;
  density?: "comfortable" | "compact" | "ultra-compact";
  stickyHeader?: boolean;
}) {
  const [query, setQuery] = useState("");
  const [sortCol, setSortCol] = useState<string | null>(null);
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(initialPageSize);

  const filtered = useMemo(() => {
    let result = rows;
    if (searchable && query.trim() && searchFilter) {
      result = result.filter((row) => searchFilter(row, query.trim().toLowerCase()));
    }
    if (sortCol) {
      const col = columns.find((c) => c.id === sortCol);
      if (col?.sortValue) {
        result = [...result].sort((a, b) => {
          const av = col.sortValue!(a);
          const bv = col.sortValue!(b);
          const cmp = av < bv ? -1 : av > bv ? 1 : 0;
          return sortDir === "asc" ? cmp : -cmp;
        });
      }
    }
    return result;
  }, [rows, query, searchable, searchFilter, sortCol, sortDir, columns]);

  const pageCount = Math.max(1, Math.ceil(filtered.length / pageSize));
  const pageRows = filtered.slice(page * pageSize, (page + 1) * pageSize);

  const toggleSort = (colId: string) => {
    if (sortCol === colId) setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    else { setSortCol(colId); setSortDir("asc"); }
    setPage(0);
  };

  if (loading) {
    return (
      <div className="m3-data-table m3-data-table--loading" aria-busy="true">
        <Skeleton width="100%" height={32} />
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} width="100%" height={density === "ultra-compact" ? 24 : 36} className="m3-data-table__skeleton-row" />
        ))}
      </div>
    );
  }

  return (
    <div className={cn("m3-data-table", `m3-data-table--${density}`)}>
      {searchable ? (
        <div className="m3-data-table__toolbar">
          <TextField
            placeholder={searchPlaceholder}
            value={query}
            onChange={(e) => { setQuery(e.target.value); setPage(0); }}
            aria-label="Search table"
          />
          <span className="m3-data-table__count">{filtered.length} rows</span>
        </div>
      ) : null}

      <div className="m3-data-table__scroll">
        <table>
          <thead className={stickyHeader ? "m3-data-table__head--sticky" : undefined}>
            <tr>
              {columns.map((col) => (
                <th key={col.id} style={{ width: col.width, textAlign: col.align ?? "left" }}>
                  {col.sortable ? (
                    <button type="button" className="m3-data-table__sort-btn" onClick={() => toggleSort(col.id)}>
                      {col.header}
                      {sortCol === col.id ? (sortDir === "asc" ? " ↑" : " ↓") : null}
                    </button>
                  ) : col.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {pageRows.length === 0 ? (
              <tr>
                <td colSpan={columns.length}>
                  <EmptyState title={emptyTitle} subtitle={emptySubtitle} />
                </td>
              </tr>
            ) : (
              pageRows.map((row, i) => (
                <tr key={getRowKey(row, page * pageSize + i)} className="m3-data-table__row">
                  {columns.map((col) => (
                    <td key={col.id} style={{ textAlign: col.align ?? "left" }} className="m3-data-table__cell--tabular">
                      {col.cell(row, page * pageSize + i)}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {filtered.length > pageSize ? (
        <footer className="m3-data-table__footer">
          <label className="m3-data-table__page-size">
            Rows
            <select value={pageSize} onChange={(e) => { setPageSize(Number(e.target.value)); setPage(0); }}>
              {pageSizeOptions.map((n) => <option key={n} value={n}>{n}</option>)}
            </select>
          </label>
          <div className="m3-data-table__pagination">
            <button type="button" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>Previous</button>
            <span>{page + 1} / {pageCount}</span>
            <button type="button" disabled={page >= pageCount - 1} onClick={() => setPage((p) => p + 1)}>Next</button>
          </div>
        </footer>
      ) : null}
    </div>
  );
}
