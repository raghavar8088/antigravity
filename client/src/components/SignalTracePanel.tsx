"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { DeskButton, DeskChip, DeskBanner, DeskCard, DeskSectionHeader } from "@/components/desk/ui";
import type { StrategySignalTraceRow, SignalTraceSummary } from "@/lib/ai/strategySignalTrace";
import { closestSignalRows, signalTraceRatio } from "@/lib/ai/strategySignalTrace";
import type { EntryFunnelSnapshot } from "@/lib/trading/deskEntryFunnelSnapshot";
import { diagnoseNoTradeRootCause, type NoTradeRootCauseResult } from "@/lib/risk/noTradeRootCause";

type TraceApiResponse = {
  ok: boolean;
  ageSeconds?: number | null;
  mode?: string;
  symbol?: string;
  summary?: SignalTraceSummary | null;
  rows?: StrategySignalTraceRow[];
  message?: string;
  error?: string;
};

type FunnelApiResponse = {
  ok: boolean;
  snapshot?: EntryFunnelSnapshot | null;
  ageSeconds?: number | null;
  healthy?: boolean;
  error?: string;
};

type WorkerHealthResponse = {
  workerLastPollAt?: number | null;
  owner?: string | null;
  stale?: boolean;
  source?: string;
  buildSha?: string | null;
};

const STATUS_COLOR: Record<string, string> = {
  OPENED: "#3fb950",
  CANDIDATE: "#58a6ff",
  FIRED: "#d29922",
  REJECTED: "#f85149",
  EVALUATED: "#8b949e",
};

const POLL_INTERVAL_MS = 5_000;

export function SignalTracePanel({ accountKey }: { accountKey?: string | null }) {
  const [data, setData] = useState<TraceApiResponse | null>(null);
  const [funnelData, setFunnelData] = useState<FunnelApiResponse | null>(null);
  const [healthData, setHealthData] = useState<WorkerHealthResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [gateFilter, setGateFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [search, setSearch] = useState("");
  const [firedOnly, setFiredOnly] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);

  const isActiveRef = useRef(true);

  const fetchTrace = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (accountKey) params.set("account_key", accountKey);
      if (gateFilter) params.set("gate", gateFilter);
      if (statusFilter) params.set("status", statusFilter);
      params.set("limit", "500");

      const [traceRes, funnelRes, healthRes] = await Promise.all([
        fetch(`/api/strategy-signal-trace?${params.toString()}`),
        fetch(`/api/desk-entry-funnel${accountKey ? `?account_key=${encodeURIComponent(accountKey)}` : ""}`),
        fetch(`/api/health/desk-worker`),
      ]);

      const traceJson = await traceRes.json() as TraceApiResponse;
      const funnelJson = await funnelRes.json() as FunnelApiResponse;
      const healthJson = await healthRes.json() as WorkerHealthResponse;

      setData(traceJson);
      setFunnelData(funnelJson.ok ? funnelJson : null);
      setHealthData(healthJson);
    } catch {
      setError("Network error fetching signal trace");
    } finally {
      setLoading(false);
    }
  }, [accountKey, gateFilter, statusFilter]);

  // Poll every 5s only when tab is visible
  useEffect(() => {
    isActiveRef.current = true;
    void fetchTrace();
    const id = setInterval(() => { if (isActiveRef.current) void fetchTrace(); }, POLL_INTERVAL_MS);
    return () => {
      isActiveRef.current = false;
      clearInterval(id);
    };
  }, [fetchTrace]);

  const summary = data?.summary ?? null;
  const allRows = data?.rows ?? [];

  const rows = useMemo(() => {
    let r = allRows;
    if (firedOnly) r = r.filter((row) => row.status === "FIRED" || row.status === "CANDIDATE" || row.status === "OPENED");
    if (search.trim()) {
      const q = search.trim().toLowerCase();
      r = r.filter((row) => row.strategyName.toLowerCase().includes(q) || String(row.strategyId).includes(q));
    }
    return r;
  }, [allRows, firedOnly, search]);

  const gateOptions = useMemo(() => {
    const gates = new Set(allRows.map((r) => r.gate));
    return ["", ...Array.from(gates).sort()];
  }, [allRows]);

  const closestSignals = useMemo(() => closestSignalRows(allRows, 10), [allRows]);

  // ── Derived diagnostic state (PART 2 / PART 8) ─────────────────────────────
  const funnel = funnelData?.snapshot ?? null;
  const workerStale = healthData?.stale ?? true;
  const traceAge = data?.ageSeconds ?? null;
  const traceEmpty = !data?.rows || data.rows.length === 0;

  const rootCauseResult: NoTradeRootCauseResult | null = useMemo(() => {
    if (!funnel && !data?.summary && workerStale) return null;
    return diagnoseNoTradeRootCause({
      funnel,
      signalTrace: { summary: data?.summary ?? null, rows: data?.rows ?? null, ageSeconds: traceAge },
      workerHealth: { stale: workerStale, workerLastPollAt: healthData?.workerLastPollAt ?? null, ageSeconds: healthData?.workerLastPollAt ? Math.floor((Date.now() - healthData.workerLastPollAt) / 1000) : null },
    });
  }, [funnel, data?.summary, data?.rows, traceAge, workerStale, healthData]);

  const workerStatus = workerStale ? "STALE" : "ACTIVE";
  const traceStatus = traceEmpty ? (workerStale ? "EMPTY (worker stale)" : "EMPTY") : (traceAge != null && traceAge > 30 ? "STALE" : "FRESH");

  const copyDebugContext = useCallback(() => {
    const ctx = [
      "Signal Trace Debug",
      `Worker: ${workerStatus}`,
      `Trace age: ${traceAge != null ? traceAge + "s" : "unknown"}`,
      `Funnel blocker: ${funnel?.dominantBlocker ?? "none"}`,
      `Root cause: ${rootCauseResult?.rootCause ?? "unknown"}`,
      `Evaluated: ${data?.summary?.totalEvaluated ?? 0}`,
      `Fired: ${data?.summary?.fired ?? 0}`,
      `Candidates: ${data?.summary?.candidates ?? 0}`,
      `Opened: ${data?.summary?.opened ?? 0}`,
      `Top gate: ${data?.summary?.topRejectedGate ?? "none"}`,
      `Closest: ${closestSignals.length > 0 ? `${closestSignals[0].strategyName} ${(signalTraceRatio(closestSignals[0]) * 100).toFixed(0)}%` : "none"}`,
      `Recommended: ${rootCauseResult?.safeFix ?? "Check worker logs"}`,
      "Relevant files: runPaperDeskPollTick.ts, strategySignalTrace.ts, SignalTracePanel.tsx",
    ].join("\n");
    void navigator.clipboard.writeText(ctx);
  }, [workerStatus, traceAge, funnel, rootCauseResult, data, closestSignals]);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12, fontSize: 11 }}>
      {/* PART 1: Title / Subtitle / Helper */}
      <div>
        <div style={{ fontSize: 15, fontWeight: 700, color: "#c9d1d9" }}>Signal Trade Lab</div>
        <div style={{ fontSize: 10, color: "#8b949e" }}>
          End-to-end strategy evaluation: signal → confirmation → gates → candidate → paper open
        </div>
        <div style={{ fontSize: 9, color: "#8b949e", marginTop: 2 }}>
          This panel explains why a signal did or did not become a paper position. It does not force trades.
        </div>
      </div>

      {/* PART 2: Always-visible Diagnostic Header */}
      <DeskCard>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 8, alignItems: "center", padding: "2px 4px" }}>
          <DeskChip tone={workerStale ? "error" : "success"}>Worker: {workerStatus}</DeskChip>
          <DeskChip tone={traceStatus.includes("STALE") || traceStatus.includes("EMPTY") ? "warning" : "success"}>Trace: {traceStatus}</DeskChip>
          {traceAge != null && <DeskChip tone="default">{traceAge}s ago</DeskChip>}
          {funnel && (
            <>
              <DeskChip tone="default">Active: {funnel.activeStrategies}</DeskChip>
              <DeskChip tone="default">Evaluated: {funnel.evaluatedStrategies}</DeskChip>
              <DeskChip tone="default">Candidates: {funnel.candidateCount}</DeskChip>
              <DeskChip tone="default">Opened: {funnel.opened}</DeskChip>
              <DeskChip tone={funnel.dominantBlocker === "none" ? "success" : "warning"}>Blocker: {funnel.dominantBlocker}</DeskChip>
            </>
          )}
          <DeskButton
            variant="outlined"
            style={{ minHeight: 24, fontSize: "0.65rem", padding: "0 8px", marginLeft: "auto" }}
            onClick={() => void fetchTrace()}
            disabled={loading}
          >
            {loading ? "…" : "Refresh"}
          </DeskButton>
          <DeskButton variant="outlined" onClick={copyDebugContext} style={{ minHeight: 24, fontSize: "0.65rem", padding: "0 8px" }}>
            Copy Debug
          </DeskButton>
        </div>
      </DeskCard>

      {error && <p style={{ fontSize: 10, color: "#f85149", margin: 0 }}>{error}</p>}

      {/* PART 8 + 9: Root Cause Banner + Freshness Warning */}
      {rootCauseResult && (
        <DeskBanner
          variant={rootCauseResult.rootCause === "WORKER_STALE" || rootCauseResult.rootCause === "STATE_DIRTY" ? "error" : rootCauseResult.rootCause.includes("BLOCKING") ? "warning" : "info"}
          title={`Current root cause: ${rootCauseResult.rootCause}`}
        >
          <div style={{ fontSize: 10 }}>
            {rootCauseResult.evidence.join(" · ")}
            <div style={{ marginTop: 4, color: "#8b949e" }}>{rootCauseResult.safeFix}</div>
          </div>
        </DeskBanner>
      )}

      {healthData && !healthData.stale && traceAge != null && traceAge > 30 && (
        <DeskBanner variant="warning" title="Worker Trace Freshness Warning">
          Worker heartbeat is fresh, but signal trace is {traceAge}s old. The worker may not be persisting `signal_trace_latest`.
        </DeskBanner>
      )}

      {/* PART 3: Professional Empty State when no trace */}
      {!data?.summary && !loading && traceEmpty && (
        <DeskCard padding="md">
          <DeskSectionHeader title="No signal trace captured yet" />
          <div style={{ fontSize: 10, color: "#8b949e", marginBottom: 8 }}>
            Likely causes:
          </div>
          <ul style={{ fontSize: 10, margin: "0 0 12px 16px", color: "#c9d1d9" }}>
            <li>Worker heartbeat stale (VPS pm2 down or lease lost)</li>
            <li>Entry funnel missing (worker never completed a tick)</li>
            <li>Worker not writing `signal_trace_latest` to paper_state</li>
            <li>Browser in monitor mode but trace not yet synced</li>
            <li>Roster empty / invalid DESK_WORKER_STRATEGY_IDS</li>
            <li>Market data unavailable (Delta API issue)</li>
          </ul>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
            <DeskButton variant="outlined" onClick={() => void fetchTrace()}>Refresh</DeskButton>
            <DeskButton variant="outlined" onClick={copyDebugContext}>Copy Signal Debug Context</DeskButton>
            <a href="/api/health/desk-worker" target="_blank" rel="noreferrer" style={{ fontSize: 10, color: "#58a6ff" }}>Open health JSON</a>
            <a href="/api/desk-entry-funnel" target="_blank" rel="noreferrer" style={{ fontSize: 10, color: "#58a6ff" }}>Open entry funnel JSON</a>
          </div>
          <div style={{ marginTop: 8, fontSize: 9, color: "#8b949e" }}>
            Restart command: <code>pm2 restart btc-ft-worker && pm2 logs btc-ft-worker --lines 50</code>
          </div>
        </DeskCard>
      )}

      {/* Filters */}
      {allRows.length > 0 && (
        <div style={{ display: "flex", flexWrap: "wrap", gap: 6, alignItems: "center" }}>
          <select
            value={gateFilter}
            onChange={(e) => setGateFilter(e.target.value)}
            style={{ fontSize: 10, background: "#161b22", color: "#c9d1d9", border: "1px solid #30363d", borderRadius: 4, padding: "2px 6px" }}
          >
            {gateOptions.map((g) => <option key={g} value={g}>{g || "All gates"}</option>)}
          </select>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            style={{ fontSize: 10, background: "#161b22", color: "#c9d1d9", border: "1px solid #30363d", borderRadius: 4, padding: "2px 6px" }}
          >
            {["", "OPENED", "CANDIDATE", "FIRED", "REJECTED", "EVALUATED"].map((s) => (
              <option key={s} value={s}>{s || "All statuses"}</option>
            ))}
          </select>
          <input
            type="text"
            placeholder="Search strategy…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ fontSize: 10, background: "#161b22", color: "#c9d1d9", border: "1px solid #30363d", borderRadius: 4, padding: "2px 6px", width: 140 }}
          />
          <label style={{ fontSize: 10, color: "#8b949e", display: "flex", alignItems: "center", gap: 4, cursor: "pointer" }}>
            <input type="checkbox" checked={firedOnly} onChange={(e) => setFiredOnly(e.target.checked)} />
            Fired/candidates only
          </label>
          <span style={{ fontSize: 9, color: "#8b949e", marginLeft: "auto" }}>{rows.length} rows</span>
        </div>
      )}

      {closestSignals.length > 0 && (
        <div style={{ overflowX: "auto", border: "1px solid #30363d", borderRadius: 6 }}>
          <div style={{ padding: "5px 6px", color: "#c9d1d9", fontWeight: 700 }}>
            Closest signals
          </div>
          <table style={{ width: "100%", fontSize: 9, borderCollapse: "collapse", fontFamily: "var(--desk-font-mono, monospace)" }}>
            <thead>
              <tr style={{ color: "#8b949e", textAlign: "left", borderTop: "1px solid #21262d", borderBottom: "1px solid #21262d" }}>
                <th style={{ padding: "3px 6px" }}>Strategy</th>
                <th style={{ padding: "3px 6px" }}>Score</th>
                <th style={{ padding: "3px 6px" }}>Threshold</th>
                <th style={{ padding: "3px 6px" }}>Ratio</th>
                <th style={{ padding: "3px 6px" }}>Confirm</th>
                <th style={{ padding: "3px 6px" }}>Regime</th>
                <th style={{ padding: "3px 6px" }}>Gate</th>
                <th style={{ padding: "3px 6px" }}>Reason</th>
              </tr>
            </thead>
            <tbody>
              {closestSignals.map((row) => (
                <tr key={`closest-${row.traceId}`} style={{ borderTop: "1px solid #21262d" }}>
                  <td style={{ padding: "3px 6px", color: "#c9d1d9" }}>#{row.strategyId} {row.strategyName}</td>
                  <td style={{ padding: "3px 6px", color: "#c9d1d9" }}>{row.signalScore.toFixed(1)}</td>
                  <td style={{ padding: "3px 6px", color: "#c9d1d9" }}>{row.requiredThreshold}</td>
                  <td style={{ padding: "3px 6px", color: signalTraceRatio(row) >= 1 ? "#3fb950" : "#d29922" }}>
                    {signalTraceRatio(row).toFixed(2)}
                  </td>
                  <td style={{ padding: "3px 6px", color: row.confirmPassed ? "#3fb950" : "#f85149" }}>
                    {row.confirmPassed ? "pass" : "fail"}
                  </td>
                  <td style={{ padding: "3px 6px", color: row.regimeAllowed ? "#3fb950" : "#f85149" }}>{row.regime}</td>
                  <td style={{ padding: "3px 6px", color: "#c9d1d9" }}>{row.gate}</td>
                  <td style={{ padding: "3px 6px", color: "#8b949e" }}>{row.reason}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Table */}
      {rows.length > 0 && (
        <div style={{ overflowX: "auto" }}>
          <table style={{ width: "100%", fontSize: 9, borderCollapse: "collapse", fontFamily: "var(--desk-font-mono, monospace)" }}>
            <thead>
              <tr style={{ color: "#8b949e", textAlign: "left", borderBottom: "1px solid #21262d" }}>
                <th style={{ padding: "3px 6px" }}>Strategy</th>
                <th style={{ padding: "3px 6px" }}>Side</th>
                <th style={{ padding: "3px 6px" }}>Status</th>
                <th style={{ padding: "3px 6px" }}>Gate</th>
                <th style={{ padding: "3px 6px" }}>Score / Req</th>
                <th style={{ padding: "3px 6px" }}>Confirm</th>
                <th style={{ padding: "3px 6px" }}>Regime</th>
                <th style={{ padding: "3px 6px" }}>ATR/Fee</th>
                <th style={{ padding: "3px 6px" }}>Reason</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                const rowKey = row.traceId;
                const isExpanded = expanded === rowKey;
                const color = STATUS_COLOR[row.status] ?? "#8b949e";
                return (
                  <>
                    <tr
                      key={rowKey}
                      style={{ borderTop: "1px solid #21262d", cursor: "pointer", background: isExpanded ? "#0d1117" : undefined }}
                      onClick={() => setExpanded(isExpanded ? null : rowKey)}
                    >
                      <td style={{ padding: "3px 6px", color: "#c9d1d9" }}>
                        #{row.strategyId} {row.strategyName}
                      </td>
                      <td style={{ padding: "3px 6px", color: row.side === "LONG" ? "#3fb950" : row.side === "SHORT" ? "#f85149" : "#8b949e" }}>
                        {row.side ?? "—"}
                      </td>
                      <td style={{ padding: "3px 6px" }}>
                        <span style={{ color, border: `1px solid ${color}`, borderRadius: 3, padding: "0 4px", fontSize: 8 }}>{row.status}</span>
                      </td>
                      <td style={{ padding: "3px 6px", color: "#c9d1d9" }}>{row.gate}</td>
                      <td style={{ padding: "3px 6px", color: "#c9d1d9" }}>
                        {row.signalScore.toFixed(1)} / {row.requiredThreshold}
                      </td>
                      <td style={{ padding: "3px 6px", color: row.confirmPassed ? "#3fb950" : "#f85149" }}>
                        {row.confirmPassed ? "✓" : "✗"}
                      </td>
                      <td style={{ padding: "3px 6px", color: row.regimeAllowed ? "#3fb950" : "#f85149" }}>
                        {row.regime}
                      </td>
                      <td style={{ padding: "3px 6px", color: row.feeHurdlePassed === false ? "#f85149" : row.feeHurdlePassed ? "#3fb950" : "#8b949e" }}>
                        {row.atrPct != null ? `${(row.atrPct * 100).toFixed(3)}%` : "—"}
                      </td>
                      <td style={{ padding: "3px 6px", color: "#8b949e", maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                        {row.reason}
                      </td>
                    </tr>
                    {isExpanded && (
                      <tr key={`${rowKey}-detail`} style={{ background: "#0d1117" }}>
                        <td colSpan={9} style={{ padding: "6px 12px" }}>
                          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                            <p style={{ margin: 0, color: "#c9d1d9" }}><b>Reason:</b> {row.reason}</p>
                            {row.openedPositionId && (
                              <p style={{ margin: 0, color: "#3fb950" }}><b>Position ID:</b> {row.openedPositionId}</p>
                            )}
                            {row.candidateRank != null && (
                              <p style={{ margin: 0, color: "#58a6ff" }}><b>Candidate rank:</b> #{row.candidateRank}</p>
                            )}
                            {row.qualityScore != null && (
                              <p style={{ margin: 0, color: "#8b949e" }}><b>Quality score:</b> {row.qualityScore.toFixed(1)}</p>
                            )}
                            {row.mtfScore != null && (
                              <p style={{ margin: 0, color: "#8b949e" }}><b>MTF score:</b> {row.mtfScore.toFixed(1)}</p>
                            )}
                            {row.spreadPct != null && (
                              <p style={{ margin: 0, color: "#8b949e" }}><b>Spread:</b> {(row.spreadPct * 100).toFixed(4)}%</p>
                            )}
                            {row.contributions && row.contributions.length > 0 && (
                              <div>
                                <p style={{ margin: "0 0 2px", color: "#8b949e" }}><b>Signal contributions:</b></p>
                                <ul style={{ margin: 0, paddingLeft: 16 }}>
                                  {row.contributions.map((c, i) => (
                                    <li key={i} style={{ color: c.pts >= 0 ? "#3fb950" : "#f85149" }}>
                                      {c.pts >= 0 ? "+" : ""}{c.pts} {c.reason}
                                    </li>
                                  ))}
                                </ul>
                              </div>
                            )}
                          </div>
                        </td>
                      </tr>
                    )}
                  </>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Reject breakdown */}
      {summary && Object.keys(summary.rejectedByGate).length > 0 && (
        <div>
          <p style={{ fontSize: 9, color: "#8b949e", margin: "0 0 4px" }}>Rejections by gate:</p>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
            {Object.entries(summary.rejectedByGate)
              .sort(([, a], [, b]) => b - a)
              .map(([gate, count]) => (
                <span key={gate} style={{ fontSize: 8, border: "1px solid #f85149", borderRadius: 3, padding: "0 5px", color: "#f85149" }}>
                  {gate}: {count}
                </span>
              ))}
          </div>
        </div>
      )}
    </div>
  );
}
