"use client";

import { useCallback, useEffect, useState } from "react";
import { DeskButton, DeskChip } from "@/components/desk/ui";
import type { AiAppTrackerReport } from "@/lib/aiAppTracker/types";

type Props = {
  workerStatus: string;
  dominantBlocker: string;
};

const SEVERITY_COLOR: Record<string, string> = {
  info: "#3fb950",
  warning: "#d29922",
  danger: "#f85149",
};

const SEVERITY_TONE: Record<string, "success" | "warning" | "error" | "default"> = {
  info: "success",
  warning: "warning",
  danger: "error",
};

export function AiAppTrackerPanel({ workerStatus, dominantBlocker }: Props) {
  const [report, setReport] = useState<AiAppTrackerReport | null>(null);
  const [loading, setLoading] = useState(false);
  const [capturing, setCapturing] = useState(false);
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchLatest = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/ai-app-tracker/latest");
      if (res.ok) {
        const data = await res.json() as { ok: boolean; report?: AiAppTrackerReport | null };
        if (data.ok) setReport(data.report ?? null);
        else setReport(null);
      } else if (res.status !== 404) {
        setError("Failed to load report");
      }
    } catch {
      setError("Network error fetching report");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void fetchLatest(); }, [fetchLatest]);

  const captureNow = useCallback(async () => {
    setCapturing(true);
    setError(null);
    try {
      const res = await fetch("/api/ai-app-tracker/capture", { method: "POST" });
      const data = await res.json() as { ok: boolean; error?: string };
      if (data.ok) await fetchLatest();
      else setError(data.error ?? "Capture failed");
    } catch {
      setError("Network error during capture");
    } finally {
      setCapturing(false);
    }
  }, [fetchLatest]);

  const copyAiContext = useCallback(() => {
    const lines: string[] = [
      `[BTC Paper Desk AI Context — ${new Date().toISOString()}]`,
      `Worker: ${workerStatus}`,
      `Blocker: ${dominantBlocker}`,
    ];
    if (report) {
      lines.push(`Last report (${report.severity.toUpperCase()}): ${report.summary}`);
      if (report.recommendations[0]) lines.push(`Action: ${report.recommendations[0]}`);
      lines.push(`Warnings: ${report.snapshot.warnings.length > 0 ? report.snapshot.warnings.join(" | ") : "none"}`);
    } else {
      lines.push("No tracker report yet.");
    }
    lines.push("Mind map: client/docs/AI_APPLICATION_MINDMAP.md");
    lines.push("JSON twin: client/docs/ai-application-mindmap.json");
    lines.push("Key files: client/src/lib/deskEntryFunnelSnapshot.ts | client/src/lib/futuresDeskPolicy.ts | client/src/hooks/useBTCFuturesScalperEngine.ts");

    void navigator.clipboard.writeText(lines.join("\n")).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }, [report, workerStatus, dominantBlocker]);

  const reportAgeMin = report
    ? Math.floor((Date.now() - new Date(report.created_at).getTime()) / 60_000)
    : null;

  const snap = report?.snapshot;

  return (
    <div
      style={{
        border: "1px solid #21262d",
        borderRadius: 8,
        padding: "10px 12px",
        background: "#161b22",
        fontSize: 11,
        display: "flex",
        flexDirection: "column",
        gap: 8,
      }}
    >
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
        <span style={{ fontWeight: 600, color: "#58a6ff" }}>AI App Tracker</span>
        <div style={{ display: "flex", gap: 6 }}>
          <DeskButton
            variant="outlined"
            style={{ minHeight: 26, fontSize: "0.65rem", padding: "0 8px" }}
            onClick={() => void captureNow()}
            disabled={capturing}
          >
            {capturing ? "Capturing…" : "Capture report now"}
          </DeskButton>
          <DeskButton
            variant="outlined"
            style={{ minHeight: 26, fontSize: "0.65rem", padding: "0 8px" }}
            onClick={copyAiContext}
          >
            {copied ? "Copied!" : "Copy AI context"}
          </DeskButton>
        </div>
      </div>

      {error && <p style={{ fontSize: 10, color: "#f85149", margin: 0 }}>{error}</p>}

      {/* Worker / blocker row */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: 6, alignItems: "center" }}>
        <span style={{ fontSize: 10, color: "#8b949e" }}>Worker:</span>
        <span style={{ fontSize: 10, color: "#c9d1d9" }}>{workerStatus}</span>
        <span style={{ fontSize: 10, color: "#8b949e", marginLeft: 8 }}>Blocker:</span>
        <code style={{ fontSize: 10, color: "#d29922", fontFamily: "var(--desk-font-mono, monospace)" }}>
          {dominantBlocker}
        </code>
      </div>

      {/* Latest report */}
      {loading && (
        <p style={{ fontSize: 10, color: "#8b949e", margin: 0 }}>Loading latest report…</p>
      )}
      {!loading && !report && !error && (
        <p style={{ fontSize: 10, color: "#8b949e", margin: 0 }}>
          No tracker report yet. Click &quot;Capture report now&quot; to create one.
        </p>
      )}
      {report && snap && (
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          {/* Severity + age */}
          <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
            <DeskChip tone={SEVERITY_TONE[report.severity] ?? "default"}>
              {report.severity.toUpperCase()}
            </DeskChip>
            <span style={{ fontSize: 9, color: "#8b949e" }}>
              {reportAgeMin != null ? `${reportAgeMin}m ago` : ""} · {report.report_id.slice(0, 8)}
            </span>
          </div>

          {/* Summary */}
          <p
            style={{
              fontSize: 10,
              color: SEVERITY_COLOR[report.severity] ?? "#c9d1d9",
              margin: 0,
              lineHeight: 1.5,
            }}
          >
            {report.summary}
          </p>

          {/* Recommendations */}
          {report.recommendations.length > 0 && (
            <div>
              <p style={{ fontSize: 9, color: "#8b949e", margin: "0 0 4px" }}>Recommendations:</p>
              <ul style={{ margin: 0, paddingLeft: 16 }}>
                {report.recommendations.slice(0, 3).map((rec, i) => (
                  <li key={i} style={{ fontSize: 9, color: "#c9d1d9", marginBottom: 2 }}>
                    {rec}
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Warnings */}
          {snap.warnings.length > 0 && (
            <div>
              <p style={{ fontSize: 9, color: "#8b949e", margin: "0 0 2px" }}>Active warnings:</p>
              <ul style={{ margin: 0, paddingLeft: 16 }}>
                {snap.warnings.slice(0, 3).map((w, i) => (
                  <li key={i} style={{ fontSize: 9, color: "#d29922", marginBottom: 2 }}>{w}</li>
                ))}
              </ul>
            </div>
          )}

          {/* Snapshot quick-stats using nested fields */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: "2px 12px",
              fontSize: 9,
              color: "#8b949e",
              marginTop: 2,
            }}
          >
            <span>Open positions: <b style={{ color: "#c9d1d9" }}>{snap.paperState.openPositions}</b></span>
            <span>Worker: <b style={{ color: snap.worker.stale ? "#f85149" : "#3fb950" }}>
              {snap.worker.stale ? `STALE (${snap.worker.ageSeconds}s)` : "LIVE"}
            </b></span>
            <span>Active strats: <b style={{ color: "#c9d1d9" }}>{snap.entryFunnel.activeStrategies ?? "?"}</b></span>
            {snap.paperState.balanceDriftUsd != null && (
              <span>
                Balance drift:{" "}
                <b
                  style={{
                    color: snap.paperState.balanceDriftUsd < -20
                      ? "#f85149"
                      : snap.paperState.balanceDriftUsd > 20
                        ? "#3fb950"
                        : "#c9d1d9",
                  }}
                >
                  {snap.paperState.balanceDriftUsd >= 0 ? "+" : ""}
                  ${snap.paperState.balanceDriftUsd.toFixed(2)}
                </b>
              </span>
            )}
          </div>

          {/* Env flags */}
          <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
            {Object.entries(snap.env).map(([k, v]) =>
              v ? (
                <span
                  key={k}
                  style={{
                    fontSize: 8,
                    border: "1px solid #30363d",
                    borderRadius: 4,
                    padding: "1px 5px",
                    color: "#58a6ff",
                    fontFamily: "var(--desk-font-mono, monospace)",
                  }}
                >
                  {k}
                </span>
              ) : null,
            )}
          </div>
        </div>
      )}

      {/* Pointer to docs */}
      <p style={{ fontSize: 9, color: "#8b949e", margin: 0 }}>
        Mind map:{" "}
        <code style={{ fontFamily: "var(--desk-font-mono, monospace)" }}>
          client/docs/AI_APPLICATION_MINDMAP.md
        </code>
        {" · "}
        CLI:{" "}
        <code style={{ fontFamily: "var(--desk-font-mono, monospace)" }}>npm run ai:summary</code>
      </p>
    </div>
  );
}
