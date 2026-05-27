"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { DeskButton, DeskChip } from "@/components/desk/ui";
import type { StrategySignalTraceRow, SignalTraceSummary } from "@/lib/strategySignalTrace";

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
      const res = await fetch(`/api/strategy-signal-trace?${params.toString()}`);
      const json = await res.json() as TraceApiResponse;
      setData(json);
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

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10, fontSize: 11 }}>
      {/* Summary chips */}
      {summary && (
        <div style={{ display: "flex", flexWrap: "wrap", gap: 6, alignItems: "center" }}>
          <DeskChip tone="default">Evaluated: {summary.totalEvaluated}</DeskChip>
          <DeskChip tone={summary.fired > 0 ? "warning" : "default"}>Fired: {summary.fired}</DeskChip>
          <DeskChip tone={summary.candidates > 0 ? "primary" : "default"}>Candidates: {summary.candidates}</DeskChip>
          <DeskChip tone={summary.opened > 0 ? "success" : "default"}>Opened: {summary.opened}</DeskChip>
          {summary.topRejectedGate && (
            <DeskChip tone="error">Top reject: {summary.topRejectedGate}</DeskChip>
          )}
          {data?.ageSeconds != null && (
            <span style={{ fontSize: 9, color: "#8b949e" }}>
              {data.ageSeconds}s ago · {data.mode ?? "?"} · {data.symbol ?? ""}
            </span>
          )}
          <DeskButton
            variant="outlined"
            style={{ minHeight: 24, fontSize: "0.65rem", padding: "0 8px", marginLeft: "auto" }}
            onClick={() => void fetchTrace()}
            disabled={loading}
          >
            {loading ? "…" : "Refresh"}
          </DeskButton>
        </div>
      )}

      {error && <p style={{ fontSize: 10, color: "#f85149", margin: 0 }}>{error}</p>}

      {!summary && !loading && (
        <p style={{ fontSize: 10, color: "#8b949e", margin: 0 }}>
          {data?.message ?? "No signal trace yet. Keep worker running or wait for next poll tick."}
        </p>
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
