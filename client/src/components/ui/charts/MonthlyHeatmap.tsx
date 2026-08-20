"use client";

import { AutoSortTable } from "@/components/desk/ui";

export interface MonthlyHeatmapEntry {
  year: number;
  month: number;
  return_pct: number;
}

export interface MonthlyHeatmapProps {
  data: MonthlyHeatmapEntry[];
}

const MONTH_LABELS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

function cellColor(pct: number) {
  if (Math.abs(pct) < 0.05) return "rgba(20,20,30,0.04)";
  const t = Math.min(1, Math.abs(pct) / 10);
  const alpha = 0.1 + t * 0.5;
  return pct > 0 ? `rgba(14,159,110,${alpha})` : `rgba(217,45,63,${alpha})`;
}

export function MonthlyHeatmap({ data }: MonthlyHeatmapProps) {
  if (data.length === 0) return null;
  const years = Array.from(new Set(data.map((d) => d.year))).sort();
  const byYearMonth = new Map(data.map((d) => [`${d.year}-${d.month}`, d.return_pct]));

  return (
    <div style={{ overflowX: "auto" }}>
      <AutoSortTable><table style={{ borderCollapse: "separate", borderSpacing: 2, width: "100%" }}>
        <thead>
          <tr>
            <th style={{ fontSize: 10.5, color: "var(--text-faint, var(--text-muted))", fontFamily: "var(--font-mono)", padding: "6px 2px" }} />
            {MONTH_LABELS.map((m) => (
              <th key={m} style={{ fontSize: 10.5, color: "var(--text-faint, var(--text-muted))", fontFamily: "var(--font-mono)", padding: "6px 2px", minWidth: 40 }}>
                {m}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {years.map((year) => (
            <tr key={year}>
              <td style={{ fontSize: 10.5, color: "var(--text-muted)", fontFamily: "var(--font-mono)", padding: "6px 2px" }}>{year}</td>
              {MONTH_LABELS.map((_, monthIdx) => {
                const pct = byYearMonth.get(`${year}-${monthIdx + 1}`);
                return (
                  <td
                    key={monthIdx}
                    title={pct != null ? `${year}-${String(monthIdx + 1).padStart(2, "0")}: ${pct.toFixed(2)}%` : "No data"}
                    style={{
                      fontSize: 10.5,
                      fontFamily: "var(--font-mono)",
                      fontVariantNumeric: "tabular-nums",
                      textAlign: "center",
                      borderRadius: 4,
                      padding: "6px 2px",
                      minWidth: 40,
                      background: pct != null ? cellColor(pct) : "transparent",
                      color: "var(--text, var(--text-primary))",
                    }}
                  >
                    {pct != null ? `${pct > 0 ? "+" : ""}${pct.toFixed(1)}` : "—"}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table></AutoSortTable>
    </div>
  );
}
