"use client";

import { useCallback, useEffect, useState } from "react";
import { DeskButton, DeskChip } from "@/components/desk/ui";
import type { ResearchEdgeScore } from "@/lib/ai/researchEdgeScore";
import type { RegimeRosterOutput } from "@/lib/analytics/regimeRosterBuilder";
import type { WalkForwardResult } from "@/lib/analytics/walkForwardValidation";

// ─── API response shape ───────────────────────────────────────────────────────

interface EdgeReportResponse {
  ok: boolean;
  error?: string;
  windowDays: number;
  tradeCount: number;
  generatedAt: string;
  scores: ResearchEdgeScore[];
  regimeRosters: RegimeRosterOutput;
  walkForward: WalkForwardResult;
  topPromote: number[];
  topDisable: number[];
  recommendations: string[];
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

const STATUS_COLOR: Record<ResearchEdgeScore["status"], string> = {
  PROMOTE: "var(--desk-success)",
  KEEP: "var(--desk-primary)",
  WATCH: "var(--desk-warning)",
  DISABLE: "var(--desk-error)",
  INSUFFICIENT: "var(--desk-on-surface-variant)",
};

const STATUS_BG: Record<ResearchEdgeScore["status"], string> = {
  PROMOTE: "var(--desk-success-container)",
  KEEP: "var(--desk-primary-container)",
  WATCH: "var(--desk-warning-container)",
  DISABLE: "var(--desk-error-container)",
  INSUFFICIENT: "var(--desk-surface-container)",
};

const WFE_COLOR: Record<WalkForwardResult["status"], string> = {
  PASS: "var(--desk-success)",
  FAIL: "var(--desk-error)",
  COLLECT_DATA: "var(--desk-on-surface-variant)",
};

function pct(n: number) {
  return `${(n * 100).toFixed(1)}%`;
}
function usd(n: number) {
  return `${n >= 0 ? "+" : ""}$${Math.abs(n).toFixed(2)}`;
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <p
      style={{
        fontSize: "0.625rem",
        fontWeight: 600,
        textTransform: "uppercase",
        letterSpacing: "0.07em",
        color: "var(--desk-on-surface-variant)",
        margin: "0 0 8px",
      }}
    >
      {children}
    </p>
  );
}

function StatusChip({ status }: { status: ResearchEdgeScore["status"] }) {
  return (
    <span
      style={{
        fontSize: "0.625rem",
        fontWeight: 700,
        fontFamily: "var(--desk-font-mono)",
        padding: "1px 6px",
        borderRadius: 4,
        color: STATUS_COLOR[status],
        background: STATUS_BG[status],
        border: `1px solid ${STATUS_COLOR[status]}`,
        letterSpacing: "0.04em",
      }}
    >
      {status}
    </span>
  );
}

function CopyEnvButton({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
      <code
        style={{
          fontSize: "0.625rem",
          fontFamily: "var(--desk-font-mono)",
          background: "var(--desk-surface-dim)",
          border: "1px solid var(--desk-outline)",
          borderRadius: 4,
          padding: "3px 8px",
          color: "var(--desk-primary)",
          flex: 1,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
        title={value}
      >
        {value}
      </code>
      <DeskButton
        variant="outlined"
        style={{ minHeight: 26, fontSize: "0.625rem", padding: "0 8px", flexShrink: 0 }}
        onClick={() => {
          void navigator.clipboard.writeText(value).then(() => {
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
          });
        }}
      >
        {copied ? "Copied!" : `Copy ${label}`}
      </DeskButton>
    </div>
  );
}

// ─── Edge Summary counts ──────────────────────────────────────────────────────

function EdgeSummary({ scores }: { scores: ResearchEdgeScore[] }) {
  const counts: Record<ResearchEdgeScore["status"], number> = {
    PROMOTE: 0, KEEP: 0, WATCH: 0, DISABLE: 0, INSUFFICIENT: 0,
  };
  for (const s of scores) counts[s.status]++;

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "repeat(5, 1fr)",
        gap: 8,
      }}
    >
      {(["PROMOTE", "KEEP", "WATCH", "DISABLE", "INSUFFICIENT"] as const).map((status) => (
        <div
          key={status}
          style={{
            background: STATUS_BG[status],
            border: `1px solid ${STATUS_COLOR[status]}`,
            borderRadius: 8,
            padding: "10px 8px",
            textAlign: "center",
          }}
        >
          <div
            style={{
              fontSize: "1.25rem",
              fontWeight: 700,
              fontFamily: "var(--desk-font-mono)",
              color: STATUS_COLOR[status],
            }}
          >
            {counts[status]}
          </div>
          <div
            style={{
              fontSize: "0.5625rem",
              fontWeight: 600,
              color: STATUS_COLOR[status],
              letterSpacing: "0.05em",
              marginTop: 2,
            }}
          >
            {status}
          </div>
        </div>
      ))}
    </div>
  );
}

// ─── Strategy table ───────────────────────────────────────────────────────────

function StrategyTable({ scores }: { scores: ResearchEdgeScore[] }) {
  if (scores.length === 0) {
    return (
      <p style={{ fontSize: "0.6875rem", color: "var(--desk-on-surface-variant)", margin: 0 }}>
        No strategy data yet — capture a research edge report after accumulating ≥5 trades.
      </p>
    );
  }

  return (
    <div style={{ overflowX: "auto" }}>
      <table
        style={{
          width: "100%",
          fontSize: "0.6875rem",
          borderCollapse: "collapse",
          fontFamily: "var(--desk-font-body)",
        }}
      >
        <thead>
          <tr
            style={{
              borderBottom: "2px solid var(--desk-outline)",
              color: "var(--desk-on-surface-variant)",
              textAlign: "left",
            }}
          >
            {["Strategy", "Trades", "Expectancy", "Fee/Gross", "PF", "Win Rate", "Conf.", "Status"].map(
              (h) => (
                <th key={h} style={{ padding: "4px 8px 6px", fontWeight: 600, fontSize: "0.625rem", textTransform: "uppercase", letterSpacing: "0.05em" }}>
                  {h}
                </th>
              ),
            )}
            <th style={{ padding: "4px 8px 6px", fontWeight: 600, fontSize: "0.625rem", textTransform: "uppercase", letterSpacing: "0.05em" }}>Reason</th>
          </tr>
        </thead>
        <tbody>
          {scores.map((s) => (
            <tr
              key={s.strategyId}
              style={{ borderTop: "1px solid var(--desk-outline-variant)" }}
            >
              <td style={{ padding: "5px 8px", color: "var(--desk-on-surface)", fontWeight: 500 }}>
                <span style={{ fontFamily: "var(--desk-font-mono)", fontSize: "0.625rem", color: "var(--desk-on-surface-variant)" }}>
                  #{s.strategyId}
                </span>{" "}
                {s.strategyName}
              </td>
              <td style={{ padding: "5px 8px", color: "var(--desk-on-surface)", fontFamily: "var(--desk-font-mono)" }}>
                {s.trades}
              </td>
              <td
                style={{
                  padding: "5px 8px",
                  fontFamily: "var(--desk-font-mono)",
                  color: s.expectancy >= 0 ? "var(--desk-success)" : "var(--desk-error)",
                  fontWeight: 600,
                }}
              >
                {usd(s.expectancy)}
              </td>
              <td
                style={{
                  padding: "5px 8px",
                  fontFamily: "var(--desk-font-mono)",
                  color: s.feePctOfAbsGross > 100 ? "var(--desk-error)" : s.feePctOfAbsGross > 50 ? "var(--desk-warning)" : "var(--desk-on-surface)",
                }}
              >
                {s.feePctOfAbsGross.toFixed(0)}%
              </td>
              <td
                style={{
                  padding: "5px 8px",
                  fontFamily: "var(--desk-font-mono)",
                  color: s.profitFactor >= 1.1 ? "var(--desk-success)" : "var(--desk-on-surface-variant)",
                }}
              >
                {s.profitFactor >= 9.9 ? "∞" : s.profitFactor.toFixed(2)}
              </td>
              <td style={{ padding: "5px 8px", fontFamily: "var(--desk-font-mono)", color: "var(--desk-on-surface)" }}>
                {pct(s.winRate)}
              </td>
              <td style={{ padding: "5px 8px", fontFamily: "var(--desk-font-mono)", color: "var(--desk-on-surface-variant)" }}>
                {pct(s.confidence)}
              </td>
              <td style={{ padding: "5px 8px" }}>
                <StatusChip status={s.status} />
              </td>
              <td
                style={{
                  padding: "5px 8px",
                  fontSize: "0.625rem",
                  color: "var(--desk-on-surface-variant)",
                  maxWidth: 260,
                  lineHeight: 1.4,
                }}
              >
                {s.reason}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Regime Rosters section ───────────────────────────────────────────────────

function RegimeRosters({ rosters }: { rosters: RegimeRosterOutput }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      {[
        { label: "Chop roster", ids: rosters.chopRoster, envLine: rosters.envLines.chop },
        { label: "Trend roster", ids: rosters.trendRoster, envLine: rosters.envLines.trend },
        { label: "All-weather roster", ids: rosters.highVolRoster, envLine: rosters.envLines.allWeather },
      ].map(({ label, ids, envLine }) => (
        <div key={label}>
          <p style={{ fontSize: "0.6875rem", fontWeight: 600, color: "var(--desk-on-surface)", margin: "0 0 4px" }}>
            {label}{" "}
            <span style={{ fontFamily: "var(--desk-font-mono)", fontSize: "0.625rem", color: "var(--desk-on-surface-variant)" }}>
              ({ids.length} strategies)
            </span>
          </p>
          {ids.length > 0 ? (
            <CopyEnvButton label={label} value={envLine} />
          ) : (
            <p style={{ fontSize: "0.625rem", color: "var(--desk-on-surface-variant)", margin: 0 }}>
              No qualifying strategies yet.
            </p>
          )}
        </div>
      ))}
      {rosters.disabledIds.length > 0 && (
        <div>
          <p style={{ fontSize: "0.6875rem", fontWeight: 600, color: "var(--desk-error)", margin: "0 0 4px" }}>
            Recommended to disable ({rosters.disabledIds.length})
          </p>
          <p style={{ fontSize: "0.625rem", fontFamily: "var(--desk-font-mono)", color: "var(--desk-on-surface-variant)", margin: 0 }}>
            {rosters.disabledIds.join(", ")}
          </p>
        </div>
      )}
    </div>
  );
}

// ─── Walk-forward section ─────────────────────────────────────────────────────

function WalkForwardSection({ result }: { result: WalkForwardResult }) {
  const color = WFE_COLOR[result.status];
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span
          style={{
            fontSize: "0.75rem",
            fontWeight: 700,
            fontFamily: "var(--desk-font-mono)",
            color,
          }}
        >
          {result.status}
        </span>
        <span style={{ fontSize: "0.6875rem", color: "var(--desk-on-surface-variant)" }}>
          Aggregate WFE: {(result.aggregateWFE * 100).toFixed(0)}%
        </span>
        <span style={{ fontSize: "0.6875rem", color: "var(--desk-on-surface-variant)" }}>
          · {result.windows.length} window{result.windows.length !== 1 ? "s" : ""}
        </span>
      </div>
      <p style={{ fontSize: "0.6875rem", color: "var(--desk-on-surface)", margin: 0, lineHeight: 1.5 }}>
        {result.reason}
      </p>
      {result.windows.length > 0 && (
        <div style={{ overflowX: "auto" }}>
          <table style={{ width: "100%", fontSize: "0.625rem", borderCollapse: "collapse", fontFamily: "var(--desk-font-mono)" }}>
            <thead>
              <tr style={{ color: "var(--desk-on-surface-variant)", borderBottom: "1px solid var(--desk-outline)" }}>
                <th style={{ padding: "3px 6px", textAlign: "left" }}>Test window</th>
                <th style={{ padding: "3px 6px" }}>Train trades</th>
                <th style={{ padding: "3px 6px" }}>Test trades</th>
                <th style={{ padding: "3px 6px" }}>Train E[pnl]</th>
                <th style={{ padding: "3px 6px" }}>Test E[pnl]</th>
                <th style={{ padding: "3px 6px" }}>WFE</th>
                <th style={{ padding: "3px 6px" }}>Pass</th>
              </tr>
            </thead>
            <tbody>
              {result.windows.map((w, i) => (
                <tr key={i} style={{ borderTop: "1px solid var(--desk-outline-variant)" }}>
                  <td style={{ padding: "3px 6px", color: "var(--desk-on-surface-variant)" }}>
                    {w.testStart.slice(0, 10)}
                  </td>
                  <td style={{ padding: "3px 6px", textAlign: "center" }}>{w.trainTrades}</td>
                  <td style={{ padding: "3px 6px", textAlign: "center" }}>{w.testTrades}</td>
                  <td style={{ padding: "3px 6px", textAlign: "right", color: w.trainExpectancy >= 0 ? "var(--desk-success)" : "var(--desk-error)" }}>
                    {usd(w.trainExpectancy)}
                  </td>
                  <td style={{ padding: "3px 6px", textAlign: "right", color: w.testExpectancy >= 0 ? "var(--desk-success)" : "var(--desk-error)" }}>
                    {usd(w.testExpectancy)}
                  </td>
                  <td style={{ padding: "3px 6px", textAlign: "right", color: w.pass ? "var(--desk-success)" : "var(--desk-error)" }}>
                    {(w.walkForwardEfficiency * 100).toFixed(0)}%
                  </td>
                  <td style={{ padding: "3px 6px", textAlign: "center" }}>
                    {w.pass ? "✓" : "✗"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// ─── Main panel ───────────────────────────────────────────────────────────────

export function EdgeLabPanel() {
  const [report, setReport] = useState<EdgeReportResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [windowDays, setWindowDays] = useState(30);
  const [copiedRecs, setCopiedRecs] = useState(false);

  const fetchReport = useCallback(
    async (days: number) => {
      setLoading(true);
      setError(null);
      try {
        const res = await fetch(`/api/research-edge-report?window_days=${days}`);
        const data = (await res.json()) as EdgeReportResponse;
        if (data.ok) setReport(data);
        else setError(data.error ?? "Failed to load report");
      } catch {
        setError("Network error fetching edge report");
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    void fetchReport(windowDays);
  }, [fetchReport, windowDays]);

  const copyRecommendations = useCallback(() => {
    if (!report) return;
    void navigator.clipboard
      .writeText(report.recommendations.join("\n"))
      .then(() => {
        setCopiedRecs(true);
        setTimeout(() => setCopiedRecs(false), 1500);
      });
  }, [report]);

  return (
    <div
      style={{
        border: "1px solid var(--desk-outline)",
        borderRadius: "var(--desk-radius-card)",
        background: "var(--desk-surface)",
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
        boxShadow: "var(--desk-elevation-1)",
      }}
    >
      {/* Header */}
      <div
        style={{
          padding: "var(--desk-space-4)",
          borderBottom: "1px solid var(--desk-outline)",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: "var(--desk-space-3)",
          flexWrap: "wrap",
          background: "var(--desk-surface-container)",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <span
            style={{
              fontFamily: "var(--desk-font-display)",
              fontSize: "0.9375rem",
              fontWeight: 600,
              color: "var(--desk-on-surface)",
            }}
          >
            Edge Lab
          </span>
          <DeskChip tone="warning">Research only</DeskChip>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ fontSize: "0.6875rem", color: "var(--desk-on-surface-variant)" }}>
            Window:
          </span>
          {[7, 14, 30, 60].map((d) => (
            <DeskButton
              key={d}
              variant={windowDays === d ? "tonal" : "outlined"}
              style={{ minHeight: 28, fontSize: "0.6875rem", padding: "0 10px" }}
              onClick={() => setWindowDays(d)}
            >
              {d}d
            </DeskButton>
          ))}
          <DeskButton
            variant="outlined"
            style={{ minHeight: 28, fontSize: "0.6875rem", padding: "0 10px" }}
            onClick={() => void fetchReport(windowDays)}
            disabled={loading}
          >
            {loading ? "Loading…" : "Refresh"}
          </DeskButton>
        </div>
      </div>

      {/* Disclaimer banner */}
      <div
        style={{
          padding: "8px var(--desk-space-4)",
          background: "var(--desk-warning-container)",
          borderBottom: "1px solid var(--desk-outline)",
          fontSize: "0.6875rem",
          color: "var(--desk-warning)",
          fontWeight: 500,
        }}
      >
        ⚠ Research recommendations only. Past paper performance does not guarantee future results.
        No live orders. No threshold changes. No gate bypassing.
      </div>

      <div
        style={{
          padding: "var(--desk-space-4)",
          display: "flex",
          flexDirection: "column",
          gap: "var(--desk-space-4)",
        }}
      >
        {error && (
          <p style={{ fontSize: "0.75rem", color: "var(--desk-error)", margin: 0 }}>{error}</p>
        )}

        {loading && !report && (
          <p style={{ fontSize: "0.6875rem", color: "var(--desk-on-surface-variant)", margin: 0 }}>
            Loading edge report…
          </p>
        )}

        {report && (
          <>
            {/* Meta row */}
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: 12,
                fontSize: "0.6875rem",
                color: "var(--desk-on-surface-variant)",
                flexWrap: "wrap",
              }}
            >
              <span>
                Window: <b style={{ color: "var(--desk-on-surface)" }}>{report.windowDays}d</b>
              </span>
              <span>
                Trades: <b style={{ color: "var(--desk-on-surface)" }}>{report.tradeCount}</b>
              </span>
              <span>
                Strategies:{" "}
                <b style={{ color: "var(--desk-on-surface)" }}>{report.scores.length}</b>
              </span>
              <span style={{ marginLeft: "auto", fontSize: "0.625rem" }}>
                {new Date(report.generatedAt).toLocaleTimeString()}
              </span>
            </div>

            {/* Edge Summary */}
            <div>
              <SectionLabel>Edge Summary</SectionLabel>
              <EdgeSummary scores={report.scores} />
            </div>

            {/* Recommendations */}
            {report.recommendations.length > 0 && (
              <div>
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    marginBottom: 8,
                  }}
                >
                  <SectionLabel>Recommendations</SectionLabel>
                  <DeskButton
                    variant="text"
                    style={{ minHeight: 22, fontSize: "0.625rem", padding: "0 6px" }}
                    onClick={copyRecommendations}
                  >
                    {copiedRecs ? "Copied!" : "Copy all"}
                  </DeskButton>
                </div>
                <ul style={{ margin: 0, paddingLeft: 16 }}>
                  {report.recommendations.map((rec, i) => (
                    <li
                      key={i}
                      style={{
                        fontSize: "0.6875rem",
                        color: "var(--desk-on-surface)",
                        lineHeight: 1.5,
                        marginBottom: 4,
                      }}
                    >
                      {rec}
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {/* Strategy table */}
            <div>
              <SectionLabel>Strategy Edge Table ({report.scores.length})</SectionLabel>
              <StrategyTable scores={report.scores} />
            </div>

            {/* Regime Rosters */}
            <div>
              <SectionLabel>Regime Rosters</SectionLabel>
              <RegimeRosters rosters={report.regimeRosters} />
            </div>

            {/* Walk-Forward */}
            <div>
              <SectionLabel>Walk-Forward Validation</SectionLabel>
              <WalkForwardSection result={report.walkForward} />
            </div>
          </>
        )}
      </div>
    </div>
  );
}
