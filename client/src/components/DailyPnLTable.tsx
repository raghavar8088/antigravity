"use client";

import { type CSSProperties, type ReactNode, useEffect, useMemo, useState } from "react";
import { TerminalCard } from "@/components/terminal/institutional/TerminalCard";
import { SkeletonBlock } from "@/components/ui/EmptyState";

const REFRESH_MS = 60_000;
const DISPLAY_TIMEZONE = "Asia/Kolkata";

type RangeFilter = "7d" | "30d" | "all";

interface DailyPnlRow {
  date: string;
  dateIst: string;
  realizedPnl: number;
  tradeCount: number;
  winCount: number;
  lossCount: number;
  winRate: number;
  startingEquity: number;
  endingEquity: number;
  pnlPct: number;
  bestTrade: number;
  worstTrade: number;
  strategies: string[];
}

interface DailyPnlSummary {
  totalDaysTraded: number;
  bestDayPnl: number;
  worstDayPnl: number;
  avgDailyPnl: number;
  avgDailyPct: number;
  currentStreak: number;
  streakType: "winning" | "losing";
}

interface DailyPnlApiResponse {
  ok: boolean;
  days?: DailyPnlRow[];
  summary?: DailyPnlSummary | null;
  error?: string;
}

interface DailyPnLTableProps {
  accountKey?: string;
  className?: string;
}

const tableStyle: CSSProperties = {
  width: "100%",
  borderCollapse: "separate",
  borderSpacing: 0,
  fontSize: 13,
  minWidth: 760,
};

const thStyle: CSSProperties = {
  padding: "12px 12px",
  fontSize: 11,
  fontWeight: 700,
  textTransform: "uppercase",
  letterSpacing: "0.06em",
  color: "var(--text-muted)",
  borderBottom: "1px solid var(--border-subtle, var(--border))",
  background: "var(--surface-2)",
  whiteSpace: "nowrap",
};

const tdStyle: CSSProperties = {
  padding: "13px 12px",
  verticalAlign: "middle",
  color: "var(--text-secondary)",
  borderBottom: "1px solid var(--border-subtle, var(--border))",
};

const monoCellStyle: CSSProperties = {
  fontFamily: "var(--font-mono)",
  fontVariantNumeric: "tabular-nums",
  whiteSpace: "nowrap",
};

function pnlColor(value: number): string {
  if (value > 0) return "var(--green)";
  if (value < 0) return "var(--red)";
  return "var(--text-muted)";
}

function fmtSignedUsd(value: number): string {
  if (!Number.isFinite(value)) return "—";
  const sign = value > 0 ? "+" : value < 0 ? "-" : "";
  return `${sign}$${Math.abs(value).toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}

function fmtSignedPct(value: number): string {
  if (!Number.isFinite(value)) return "—";
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(2)}%`;
}

function fmtDisplayDate(dateIst: string): string {
  // dateIst is YYYY-MM-DD; parse as a calendar date (no further TZ shifting needed for display).
  const [year, month, day] = dateIst.split("-").map(Number);
  if (!year || !month || !day) return dateIst;
  const d = new Date(Date.UTC(year, month - 1, day));
  return new Intl.DateTimeFormat("en-IN", {
    month: "short",
    day: "2-digit",
    timeZone: "UTC",
  }).format(d);
}

function rowBackground(index: number) {
  return index % 2 === 1 ? "#fbfdff" : "#ffffff";
}

function rangeCutoffMs(range: RangeFilter): number | null {
  if (range === "all") return null;
  const days = range === "7d" ? 7 : 30;
  return Date.now() - days * 24 * 60 * 60 * 1000;
}

function isWithinRange(row: DailyPnlRow, cutoffMs: number | null): boolean {
  if (cutoffMs == null) return true;
  const [year, month, day] = row.dateIst.split("-").map(Number);
  if (!year || !month || !day) return true;
  return Date.UTC(year, month - 1, day) >= cutoffMs;
}

function DailyPnLSkeleton() {
  return (
    <div style={{ overflowX: "auto" }} aria-busy="true" aria-label="Loading daily PnL">
      <table style={tableStyle}>
        <thead>
          <tr>
            {["DATE", "PNL ($)", "PNL (%)", "TRADES", "W/L", "BEST / WORST"].map((header, index) => (
              <th key={header} style={{ ...thStyle, textAlign: index === 0 ? "left" : "right" }}>
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {Array.from({ length: 3 }).map((_, rowIndex) => (
            <tr key={rowIndex} style={{ background: rowBackground(rowIndex), borderBottom: 0 }}>
              {Array.from({ length: 6 }).map((__, cellIndex) => (
                <td key={cellIndex} style={{ ...tdStyle, textAlign: cellIndex === 0 ? "left" : "right" }}>
                  <SkeletonBlock width={cellIndex === 0 ? 72 : 64} height={12} rounded={3} />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function summaryLine(summary: DailyPnlSummary): string {
  const dayLabel = summary.totalDaysTraded === 1 ? "day" : "days";
  const avgPnl = fmtSignedUsd(summary.avgDailyPnl);
  const avgPct = fmtSignedPct(summary.avgDailyPct);
  const streakIcon = summary.streakType === "winning" ? "\u{1F525}" : "❄️";
  const streakLabel = summary.streakType === "winning" ? "win streak" : "losing streak";
  const streakText = summary.currentStreak > 0 ? ` · ${streakIcon} ${summary.currentStreak}-day ${streakLabel}` : "";
  return `${summary.totalDaysTraded} ${dayLabel} traded · Avg ${avgPnl}/day · ${avgPct}/day${streakText}`;
}

const RANGE_OPTIONS: { value: RangeFilter; label: string }[] = [
  { value: "7d", label: "Last 7d" },
  { value: "30d", label: "30d" },
  { value: "all", label: "All" },
];

function rangeButtonStyle(active: boolean): CSSProperties {
  return {
    padding: "5px 10px",
    borderRadius: 6,
    border: "1px solid var(--border-subtle, var(--border))",
    background: active ? "var(--amber)" : "var(--surface)",
    color: active ? "#111" : "var(--text-secondary)",
    fontWeight: 700,
    fontSize: 11,
    cursor: "pointer",
  };
}

export function DailyPnLTable({ accountKey, className }: DailyPnLTableProps) {
  const [days, setDays] = useState<DailyPnlRow[]>([]);
  const [summary, setSummary] = useState<DailyPnlSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdatedAt, setLastUpdatedAt] = useState<number | null>(null);
  const [range, setRange] = useState<RangeFilter>("all");
  const [retryToken, setRetryToken] = useState(0);

  useEffect(() => {
    let cancelled = false;
    let inFlight = false;

    const fetchDailyPnl = async () => {
      if (inFlight) return;
      inFlight = true;
      try {
        const params = new URLSearchParams();
        if (accountKey) params.set("account_key", accountKey);
        const qs = params.toString();
        const response = await fetch(`/api/mock-trading/daily-pnl-summary${qs ? `?${qs}` : ""}`, { cache: "no-store" });
        const data: DailyPnlApiResponse = await response.json();
        if (cancelled) return;
        if (!response.ok || !data.ok) {
          setError(data.error ?? `HTTP ${response.status}`);
          return;
        }
        setDays(data.days ?? []);
        setSummary(data.summary ?? null);
        setError(null);
        setLastUpdatedAt(Date.now());
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Unknown error");
      } finally {
        inFlight = false;
        if (!cancelled) setLoading(false);
      }
    };

    setLoading(true);
    void fetchDailyPnl();
    const timer = window.setInterval(() => void fetchDailyPnl(), REFRESH_MS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [accountKey, retryToken]);

  const filteredDays = useMemo(() => {
    const cutoff = rangeCutoffMs(range);
    return days.filter((row) => isWithinRange(row, cutoff));
  }, [days, range]);

  const lastUpdatedLabel = useMemo(() => {
    if (lastUpdatedAt == null) return null;
    return new Intl.DateTimeFormat("en-IN", {
      hour: "2-digit",
      minute: "2-digit",
      hour12: true,
      timeZone: DISPLAY_TIMEZONE,
    }).format(lastUpdatedAt);
  }, [lastUpdatedAt]);

  const actions = (
    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
      {lastUpdatedLabel ? (
        <span style={{ color: "var(--text-muted)", fontSize: 10, whiteSpace: "nowrap" }}>
          Updated {lastUpdatedLabel} IST
        </span>
      ) : null}
      <div style={{ display: "flex", gap: 6 }}>
        {RANGE_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            type="button"
            style={rangeButtonStyle(range === opt.value)}
            onClick={() => setRange(opt.value)}
          >
            {opt.label}
          </button>
        ))}
      </div>
    </div>
  );

  let body: ReactNode;
  if (loading) {
    body = <DailyPnLSkeleton />;
  } else if (error) {
    body = (
      <div className="m3-terminal-empty-state">
        <div className="m3-terminal-empty-state__title">Unable to load daily PnL</div>
        <div className="m3-terminal-empty-state__subtitle">{error}</div>
        <button
          type="button"
          onClick={() => setRetryToken((t) => t + 1)}
          style={{
            marginTop: 10,
            border: "1px solid var(--border-subtle, var(--border))",
            borderRadius: 8,
            background: "#fff",
            color: "var(--text-primary)",
            cursor: "pointer",
            fontSize: 12,
            fontWeight: 700,
            padding: "6px 12px",
          }}
        >
          Retry
        </button>
      </div>
    );
  } else if (filteredDays.length === 0) {
    body = (
      <div className="m3-terminal-empty-state">
        <div className="m3-terminal-empty-state__title">No closed trades yet</div>
        <div className="m3-terminal-empty-state__subtitle">Daily PnL will appear here once trades close.</div>
      </div>
    );
  } else {
    body = (
      <div style={{ overflowX: "auto" }}>
        <table style={tableStyle}>
          <thead>
            <tr>
              {["DATE", "PNL ($)", "PNL (%)", "TRADES", "W/L", "BEST / WORST"].map((header, index) => (
                <th key={header} style={{ ...thStyle, textAlign: index === 0 ? "left" : "right" }}>
                  {header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filteredDays.map((row, index) => {
              const baseBg = rowBackground(index);
              const borderColor = row.realizedPnl > 0 ? "var(--green)" : row.realizedPnl < 0 ? "var(--red)" : "transparent";
              return (
                <tr
                  key={row.dateIst}
                  style={{
                    background: baseBg,
                    borderBottom: 0,
                    borderLeft: `3px solid ${borderColor}`,
                  }}
                >
                  <td style={{ ...tdStyle, ...monoCellStyle, color: "var(--text-primary)", fontWeight: 600 }}>
                    {fmtDisplayDate(row.dateIst)}
                  </td>
                  <td
                    style={{
                      ...tdStyle,
                      ...monoCellStyle,
                      textAlign: "right",
                      color: pnlColor(row.realizedPnl),
                      fontWeight: 700,
                    }}
                  >
                    {fmtSignedUsd(row.realizedPnl)}
                  </td>
                  <td
                    style={{
                      ...tdStyle,
                      ...monoCellStyle,
                      textAlign: "right",
                      color: pnlColor(row.pnlPct),
                      fontWeight: 600,
                    }}
                  >
                    {fmtSignedPct(row.pnlPct)}
                  </td>
                  <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>
                    {row.tradeCount} trade{row.tradeCount === 1 ? "" : "s"}
                  </td>
                  <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>
                    <span style={{ color: "var(--green)", fontWeight: 700 }}>{row.winCount}W</span>
                    {" / "}
                    <span style={{ color: "var(--red)", fontWeight: 700 }}>{row.lossCount}L</span>
                  </td>
                  <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>
                    <span style={{ color: "var(--green)" }}>{fmtSignedUsd(row.bestTrade)}</span>
                    {" / "}
                    <span style={{ color: "var(--red)" }}>{fmtSignedUsd(row.worstTrade)}</span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    );
  }

  return (
    <TerminalCard
      title="Daily PnL"
      subtitle={!loading && !error && summary ? summaryLine(summary) : undefined}
      actions={actions}
      className={className}
    >
      {body}
    </TerminalCard>
  );
}
