"use client";

import { useCallback, useEffect, useState } from "react";
import { DeskButton, DeskChip, DeskCard, DeskSectionHeader } from "@/components/desk/ui";
import type { VerificationTrackEvent } from "@/lib/verificationTrack/types";
import { AutoSortTable } from "@/components/desk/ui";

export function VerificationTrackPanel({ accountKey }: { accountKey?: string | null }) {
  const [events, setEvents] = useState<VerificationTrackEvent[]>([]);
  const [summary, setSummary] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [evRes, sumRes] = await Promise.all([
        fetch(`/api/verification-track/events?limit=30${accountKey ? `&account_key=${accountKey}` : ""}`),
        fetch(`/api/verification-track/summary?window_minutes=60`),
      ]);
      const evJson = await evRes.json();
      const sumJson = await sumRes.json();
      setEvents(evJson.events ?? []);
      setSummary(sumJson);
    } catch {
      // non-fatal
    } finally {
      setLoading(false);
    }
  }, [accountKey]);

  useEffect(() => {
    void fetchData();
    const id = setInterval(() => void fetchData(), 30_000);
    return () => clearInterval(id);
  }, [fetchData]);

  const copyAiContext = async () => {
    const res = await fetch(`/api/verification-track/ai-context?window_minutes=120`);
    const json = await res.json();
    await navigator.clipboard.writeText(json.text);
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <DeskSectionHeader title="Verification Track" subtitle="Mongo-backed observability for Cursor AI debugging" />

      {summary && (
        <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
          <DeskChip tone="default">Ticks: {summary.countsByType?.WORKER_TICK ?? 0}</DeskChip>
          <DeskChip tone="default">Fired: {summary.countsByType?.SIGNAL_FIRED ?? 0}</DeskChip>
          <DeskChip tone="warning">Rejected: {summary.countsByType?.SIGNAL_REJECTED ?? 0}</DeskChip>
          <DeskChip tone="primary">Candidates: {summary.countsByType?.CANDIDATE_BUILT ?? 0}</DeskChip>
          <DeskChip tone="success">Opened: {summary.opened}</DeskChip>
          <DeskChip tone="default">Closed: {summary.closed}</DeskChip>
          {summary.latestRootCause && <DeskChip tone="error">Root: {summary.latestRootCause.slice(0, 40)}</DeskChip>}
        </div>
      )}

      <div style={{ display: "flex", gap: 8 }}>
        <DeskButton variant="outlined" onClick={fetchData} disabled={loading}>Refresh</DeskButton>
        <DeskButton variant="outlined" onClick={copyAiContext}>Copy AI Context</DeskButton>
        <a href="/api/verification-track/ai-context" target="_blank" style={{ fontSize: 10, color: "#58a6ff" }}>Open raw JSON</a>
      </div>

      <DeskCard padding="md">
        <div style={{ fontSize: 10, color: "#8b949e", marginBottom: 6 }}>Latest events (newest first)</div>
        <div style={{ maxHeight: 320, overflow: "auto" }}>
          <AutoSortTable><table style={{ width: "100%", fontSize: 9, fontFamily: "var(--desk-font-mono, monospace)" }}>
            <thead>
              <tr style={{ color: "#8b949e", textAlign: "left", borderBottom: "1px solid #30363d" }}>
                <th>Time</th><th>Type</th><th>Strategy</th><th>Gate/Blocker</th><th>Summary</th>
              </tr>
            </thead>
            <tbody>
              {events.map(e => (
                <tr key={e.event_id} style={{ borderTop: "1px solid #21262d" }}>
                  <td style={{ color: "#8b949e" }}>{new Date(e.created_at).toLocaleTimeString()}</td>
                  <td><span style={{ color: e.severity === "danger" ? "#f85149" : e.severity === "warning" ? "#d29922" : "#3fb950" }}>{e.type}</span></td>
                  <td style={{ color: "#c9d1d9" }}>{e.strategy_name ?? e.dominant_blocker ?? "—"}</td>
                  <td style={{ color: "#8b949e" }}>{e.gate ?? e.dominant_blocker ?? "—"}</td>
                  <td style={{ color: "#8b949e", maxWidth: 260, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{e.summary}</td>
                </tr>
              ))}
              {events.length === 0 && <tr><td colSpan={5} style={{ color: "#8b949e" }}>No verification events yet.</td></tr>}
            </tbody>
          </table></AutoSortTable>
        </div>
      </DeskCard>
    </div>
  );
}
