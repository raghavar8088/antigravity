"use client";

/**
 * AttributionPanel
 * Collapsible panel showing performance attribution breakdown.
 * Additive only. No new state or fetches.
 */
import { useState } from "react";
import type { AttributionReport, AttributionBucket } from "@/lib/futuresAttribution";

interface Props {
  report: AttributionReport | null;
}

const BAR_MAX_WIDTH = 80;

function MiniBar({
  value,
  max,
  color,
}: {
  value: number;
  max: number;
  color: string;
}) {
  const width = max > 0 ? Math.abs(value / max) * BAR_MAX_WIDTH : 0;
  return (
    <div
      style={{
        display: "inline-block",
        width,
        height: 8,
        background: color,
        borderRadius: 2,
        verticalAlign: "middle",
        marginLeft: 4,
      }}
    />
  );
}

function BucketTable({
  title,
  buckets,
  maxAbs,
}: {
  title: string;
  buckets: AttributionBucket[];
  maxAbs: number;
}) {
  if (buckets.length === 0) return null;
  return (
    <div style={{ marginBottom: 10 }}>
      <div style={{ fontWeight: 600, color: "#8b949e", marginBottom: 4 }}>{title}</div>
      {buckets
        .filter((b) => b.trades > 0)
        .map((b) => (
          <div
            key={b.label}
            style={{
              display: "grid",
              gridTemplateColumns: "70px 30px 60px 1fr",
              gap: 4,
              fontSize: 10,
              marginBottom: 2,
              color: b.avgNetPnl >= 0 ? "#3fb950" : "#f85149",
            }}
          >
            <span
              style={{
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
                color: "#e6edf3",
              }}
            >
              {b.label}
            </span>
            <span style={{ color: "#8b949e" }}>{b.trades}t</span>
            <span>${b.avgNetPnl.toFixed(2)}</span>
            <MiniBar
              value={b.avgNetPnl}
              max={maxAbs}
              color={b.avgNetPnl >= 0 ? "#3fb950" : "#f85149"}
            />
          </div>
        ))}
    </div>
  );
}

export function AttributionPanel({ report }: Props) {
  const [expanded, setExpanded] = useState(false);

  if (!report || report.totalAnalyzed < 10) {
    return (
      <div
        style={{
          fontSize: 11,
          color: "#8b949e",
          padding: "4px 0",
          marginTop: 6,
        }}
      >
        Attribution: awaiting 10+ production trades
      </div>
    );
  }

  const allBuckets = [
    ...report.byHoldDuration,
    ...report.byHourOfDay,
    ...report.byTemplateFamily,
    ...report.bySide,
  ];
  const maxAbs = Math.max(...allBuckets.map((b) => Math.abs(b.avgNetPnl)), 1);

  return (
    <div
      style={{
        border: "1px solid #21262d",
        borderRadius: 8,
        padding: "8px 12px",
        fontSize: 11,
        color: "#e6edf3",
        marginTop: 8,
        background: "#0d1117",
      }}
    >
      <div
        style={{
          cursor: "pointer",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
        onClick={() => setExpanded((e) => !e)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setExpanded((v) => !v);
          }
        }}
        role="button"
        tabIndex={0}
      >
        <span style={{ fontWeight: 700, color: "#bc8cff" }}>
          📊 Performance Attribution ({report.totalAnalyzed} trades)
        </span>
        <span style={{ color: "#8b949e" }}>{expanded ? "▲" : "▼"}</span>
      </div>

      {!expanded ? (
        <div style={{ color: "#8b949e", marginTop: 2, fontSize: 10 }}>
          Best hold: {report.bestHoldBucket ?? "N/A"}
          {" · "}
          Best hour: {report.bestHour != null ? `${report.bestHour}:00 UTC` : "N/A"}
        </div>
      ) : null}

      {expanded ? (
        <div style={{ marginTop: 8 }}>
          <BucketTable title="By Hold Duration" buckets={report.byHoldDuration} maxAbs={maxAbs} />
          <BucketTable title="By Side" buckets={report.bySide} maxAbs={maxAbs} />
          <BucketTable
            title="By Template Family"
            buckets={report.byTemplateFamily}
            maxAbs={maxAbs}
          />
          <BucketTable title="By Exit Reason" buckets={report.byExitReason} maxAbs={maxAbs} />
          <BucketTable
            title="By Hour (UTC, min 3 trades)"
            buckets={report.byHourOfDay.filter((b) => b.trades >= 3)}
            maxAbs={maxAbs}
          />
          <div style={{ color: "#8b949e", fontSize: 10, marginTop: 4 }}>
            Computed: {new Date(report.computedAt).toLocaleTimeString()}
          </div>
        </div>
      ) : null}
    </div>
  );
}
