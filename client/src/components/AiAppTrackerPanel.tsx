"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { DeskButton, DeskChip } from "@/components/desk/ui";
import type { AiAppTrackerReport } from "@/lib/aiAppTracker/types";
import { recommendHealingActions, type DeskHealingAction } from "@/lib/trading/deskSelfHealing";

type Props = {
  workerStatus: string;
  dominantBlocker: string;
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
        const data = (await res.json()) as { ok: boolean; report?: AiAppTrackerReport | null };
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

  useEffect(() => {
    void fetchLatest();
  }, [fetchLatest]);

  const captureNow = useCallback(async () => {
    setCapturing(true);
    setError(null);
    try {
      const res = await fetch("/api/ai-app-tracker/capture", { method: "POST" });
      const data = (await res.json()) as { ok: boolean; error?: string };
      if (data.ok) await fetchLatest();
      else setError(data.error ?? "Capture failed");
    } catch {
      setError("Network error during capture");
    } finally {
      setCapturing(false);
    }
  }, [fetchLatest]);

  const snap = report?.snapshot;
  const healingActions: DeskHealingAction[] = useMemo(
    () => (snap ? recommendHealingActions(snap) : []),
    [snap],
  );

  const [copiedCmd, setCopiedCmd] = useState<string | null>(null);
  const copyCmd = useCallback((cmd: string) => {
    void navigator.clipboard.writeText(cmd).then(() => {
      setCopiedCmd(cmd);
      setTimeout(() => setCopiedCmd((v) => (v === cmd ? null : v)), 1500);
    });
  }, []);

  const copyAiContext = useCallback(() => {
    const lines: string[] = [
      `[BTC Paper Desk AI Context — ${new Date().toISOString()}]`,
      `Worker: ${workerStatus}`,
      `Blocker: ${dominantBlocker}`,
    ];
    if (report) {
      lines.push(`Last report (${report.severity.toUpperCase()}): ${report.summary}`);
      if (report.recommendations[0]) lines.push(`Action: ${report.recommendations[0]}`);
      const topHeal = healingActions[0];
      if (topHeal && topHeal.type !== "NO_ACTION") {
        lines.push(`Healing: [${topHeal.type}] ${topHeal.title} → ${topHeal.operatorAction}`);
      }
      lines.push(
        `Warnings: ${report.snapshot.warnings.length > 0 ? report.snapshot.warnings.join(" | ") : "none"}`,
      );
    } else {
      lines.push("No tracker report yet.");
    }
    lines.push("Mind map: client/docs/AI_APPLICATION_MINDMAP.md");
    lines.push("JSON twin: client/docs/ai-application-mindmap.json");
    lines.push(
      "Key files: client/src/lib/deskEntryFunnelSnapshot.ts | client/src/lib/futuresDeskPolicy.ts | client/src/hooks/useBTCFuturesScalperEngine.ts",
    );
    void navigator.clipboard.writeText(lines.join("\n")).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }, [report, workerStatus, dominantBlocker, healingActions]);

  const reportAgeMin =
    report ? Math.floor((Date.now() - new Date(report.created_at).getTime()) / 60_000) : null;

  const severity = report?.severity ?? "info";
  const activeHealActions = healingActions.filter((a) => a.type !== "NO_ACTION");

  return (
    <div className="ai-tracker-panel">
      {/* Severity strip */}
      <div className={`ai-tracker-severity-strip ai-tracker-severity-strip--${severity}`} />

      <div className="ai-tracker-body">
        {/* ── Header ── */}
        <div className="ai-tracker-header">
          <div className="ai-tracker-header__left">
            <span className="ai-tracker-header__title">AI App Tracker</span>
            {report && (
              <DeskChip tone={SEVERITY_TONE[severity] ?? "default"}>
                {severity.toUpperCase()}
              </DeskChip>
            )}
            {reportAgeMin != null && (
              <span className="ai-tracker-header__age">{reportAgeMin}m ago</span>
            )}
          </div>
          <div className="ai-tracker-header__actions">
            <DeskButton
              variant="outlined"
              style={{ minHeight: 30, fontSize: "0.6875rem", padding: "0 10px" }}
              onClick={() => void captureNow()}
              disabled={capturing}
            >
              {capturing ? "Capturing…" : "Capture now"}
            </DeskButton>
            <DeskButton
              variant="outlined"
              style={{ minHeight: 30, fontSize: "0.6875rem", padding: "0 10px" }}
              onClick={copyAiContext}
            >
              {copied ? "Copied!" : "Copy AI context"}
            </DeskButton>
          </div>
        </div>

        {/* ── Worker / blocker status bar ── */}
        <div className="ai-tracker-status-bar">
          <span className="ai-tracker-status-bar__label">Worker</span>
          <span className="ai-tracker-status-bar__value">{workerStatus}</span>
          <span className="ai-tracker-status-bar__sep">·</span>
          <span className="ai-tracker-status-bar__label">Blocker</span>
          <code
            style={{
              fontFamily: "var(--desk-font-mono)",
              fontSize: "0.6875rem",
              color: "var(--desk-warning)",
              fontWeight: 500,
            }}
          >
            {dominantBlocker}
          </code>
          {snap && (
            <>
              <span className="ai-tracker-status-bar__sep">·</span>
              <span className="ai-tracker-status-bar__label">VPS</span>
              <span
                className={`ai-tracker-status-bar__value ai-tracker-status-bar__value--${snap.worker.stale ? "stale" : "live"}`}
              >
                {snap.worker.stale ? `STALE (${snap.worker.ageSeconds ?? "?"}s)` : "LIVE"}
              </span>
            </>
          )}
        </div>

        {/* ── Errors / loading ── */}
        {error && (
          <p style={{ fontSize: "0.75rem", color: "var(--desk-error)", margin: 0 }}>{error}</p>
        )}
        {loading && (
          <p style={{ fontSize: "0.6875rem", color: "var(--desk-on-surface-variant)", margin: 0 }}>
            Loading…
          </p>
        )}
        {!loading && !report && !error && (
          <p style={{ fontSize: "0.6875rem", color: "var(--desk-on-surface-variant)", margin: 0 }}>
            No tracker report yet — click Capture now to create one.
          </p>
        )}

        {/* ── Report body ── */}
        {report && snap && (
          <>
            {/* Summary */}
            <p className={`ai-tracker-summary ai-tracker-summary--${severity}`}>
              {report.summary}
            </p>

            {/* Recommendations */}
            {report.recommendations.length > 0 && (
              <div>
                <p className="ai-tracker-section-label">Recommendations</p>
                <ul style={{ margin: 0, paddingLeft: 16 }}>
                  {report.recommendations.slice(0, 3).map((rec, i) => (
                    <li
                      key={i}
                      style={{
                        fontSize: "0.6875rem",
                        color: "var(--desk-on-surface)",
                        marginBottom: 3,
                        lineHeight: 1.45,
                      }}
                    >
                      {rec}
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {/* Self-healing actions */}
            {activeHealActions.length > 0 && (
              <div>
                <p className="ai-tracker-section-label">
                  Self-healing actions ({activeHealActions.length})
                </p>
                <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  {activeHealActions.slice(0, 3).map((act, i) => (
                    <div
                      key={i}
                      className={`ai-tracker-heal-card ai-tracker-heal-card--${act.severity}`}
                    >
                      <div className="ai-tracker-heal-card__title-row">
                        <span className="ai-tracker-heal-card__title">{act.title}</span>
                        <span className="ai-tracker-heal-card__type">{act.type}</span>
                        {act.safeToAutomate && (
                          <span className="ai-tracker-heal-card__safe-badge">
                            safe-to-automate
                          </span>
                        )}
                      </div>
                      <p className="ai-tracker-heal-card__reason">{act.reason}</p>
                      <p className="ai-tracker-heal-card__action">→ {act.operatorAction}</p>
                      {act.copyableCommand && (
                        <div className="ai-tracker-cmd-block">
                          <code
                            className="ai-tracker-cmd-block__code"
                            title={act.copyableCommand}
                          >
                            {act.copyableCommand}
                          </code>
                          <DeskButton
                            variant="outlined"
                            style={{ minHeight: 24, fontSize: "0.625rem", padding: "0 8px", flexShrink: 0 }}
                            onClick={() => copyCmd(act.copyableCommand!)}
                          >
                            {copiedCmd === act.copyableCommand ? "Copied" : "Copy"}
                          </DeskButton>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Auto-heal execution log */}
            {report.healingExecuted && report.healingExecuted.length > 0 && (
              <div>
                <p className="ai-tracker-section-label">
                  Auto-heal log (
                  {report.healingExecuted.filter((h) => h.status === "executed").length}/
                  {report.healingExecuted.length} executed)
                </p>
                <div style={{ display: "flex", flexDirection: "column", gap: 5 }}>
                  {report.healingExecuted.slice(0, 4).map((h, i) => (
                    <div
                      key={i}
                      className={`ai-tracker-heal-log-item ai-tracker-heal-log-item--${h.status}`}
                    >
                      <div
                        style={{
                          display: "flex",
                          alignItems: "center",
                          gap: 6,
                          flexWrap: "wrap",
                        }}
                      >
                        <span
                          style={{
                            fontSize: "0.6875rem",
                            fontWeight: 600,
                            color: "var(--desk-on-surface)",
                          }}
                        >
                          [{h.actionType}] {h.title}
                        </span>
                        <span
                          style={{
                            fontSize: "0.625rem",
                            fontFamily: "var(--desk-font-mono)",
                            border: "1px solid var(--desk-outline)",
                            borderRadius: 3,
                            padding: "0 5px",
                            color:
                              h.status === "executed"
                                ? "var(--desk-success)"
                                : h.status === "failed"
                                  ? "var(--desk-error)"
                                  : "var(--desk-on-surface-variant)",
                          }}
                        >
                          {h.status}
                        </span>
                        {h.durationMs != null && (
                          <span
                            style={{
                              fontSize: "0.625rem",
                              color: "var(--desk-on-surface-variant)",
                            }}
                          >
                            {h.durationMs}ms
                          </span>
                        )}
                      </div>
                      {h.reason && (
                        <p
                          style={{
                            fontSize: "0.625rem",
                            color: "var(--desk-on-surface-variant)",
                            margin: 0,
                            lineHeight: 1.4,
                          }}
                        >
                          {h.reason}
                        </p>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Active warnings */}
            {snap.warnings.length > 0 && (
              <div>
                <p className="ai-tracker-section-label">Active warnings</p>
                <ul style={{ margin: 0, paddingLeft: 16 }}>
                  {snap.warnings.slice(0, 3).map((w, i) => (
                    <li
                      key={i}
                      style={{
                        fontSize: "0.6875rem",
                        color: "var(--desk-warning)",
                        marginBottom: 3,
                        lineHeight: 1.4,
                      }}
                    >
                      {w}
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {/* Quick-stats grid */}
            <div className="ai-tracker-stats-grid">
              <span>
                Open positions: <b>{snap.paperState.openPositions}</b>
              </span>
              <span>
                Active strats: <b>{snap.entryFunnel.activeStrategies ?? "?"}</b>
              </span>
              {snap.paperState.balanceDriftUsd != null && (
                <span>
                  Balance drift:{" "}
                  <b
                    style={{
                      color:
                        snap.paperState.balanceDriftUsd < -20
                          ? "var(--desk-error)"
                          : snap.paperState.balanceDriftUsd > 20
                            ? "var(--desk-success)"
                            : "var(--desk-on-surface)",
                    }}
                  >
                    {snap.paperState.balanceDriftUsd >= 0 ? "+" : ""}$
                    {snap.paperState.balanceDriftUsd.toFixed(2)}
                  </b>
                </span>
              )}
              {snap.paperState.pauseEntries && (
                <span style={{ color: "var(--desk-warning)" }}>
                  Entries: <b>PAUSED</b>
                </span>
              )}
            </div>

            {/* Env flags */}
            {Object.values(snap.env).some(Boolean) && (
              <div className="ai-tracker-env-chips">
                {Object.entries(snap.env).map(([k, v]) =>
                  v ? (
                    <span key={k} className="ai-tracker-env-chip">
                      {k}
                    </span>
                  ) : null,
                )}
              </div>
            )}
          </>
        )}

        {/* ── Footer / docs ── */}
        <p className="ai-tracker-footer">
          Mind map:{" "}
          <code style={{ fontFamily: "var(--desk-font-mono)" }}>
            client/docs/AI_APPLICATION_MINDMAP.md
          </code>
          {" · "}CLI:{" "}
          <code style={{ fontFamily: "var(--desk-font-mono)" }}>npm run ai:summary</code>
        </p>
      </div>
    </div>
  );
}
