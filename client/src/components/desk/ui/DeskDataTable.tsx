"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

export type DeskColumn<T> = {
  id: string;
  header: ReactNode;
  align?: "left" | "right" | "center";
  cell: (row: T, index: number) => ReactNode;
  width?: string;
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
}: DeskDataTableProps<T>) {
  if (rows.length === 0 && empty) {
    return <>{empty}</>;
  }

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
            {columns.map((col) => (
              <th
                key={col.id}
                className="desk-label-md"
                style={{
                  padding: "10px 14px",
                  textAlign: col.align ?? "left",
                  borderBottom: "1px solid var(--desk-outline)",
                  width: col.width,
                  fontWeight: 500,
                }}
              >
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
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
