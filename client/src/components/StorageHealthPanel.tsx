"use client";

import { useCallback, useEffect, useState } from "react";
import { DeskCard } from "./desk/ui/DeskCard";
import { DeskMetricTile } from "./desk/ui/DeskMetricTile";
import { DeskSectionHeader } from "./desk/ui/DeskSectionHeader";
import { DeskButton } from "./desk/ui/DeskButton";

// ── Types ──────────────────────────────────────────────────────────────────

interface StorageHealthData {
  ok: boolean;
  serverTime: string;
  mongo: {
    configured: boolean;
    pingOk: boolean;
    db: string;
    status: "ONLINE" | "UNREACHABLE" | "NOT_CONFIGURED";
  };
  localStorage: {
    ok: boolean;
    dataRoot: string;
    totalMb: number;
    filesWrittenToday: number;
    backupCount: number;
    lastBackupAt: string | null;
    auditFileCount: number;
    writeFailures: number;
    recentFailures: { label: string; error: string; ts: string }[];
    queueStats: { queued: number; succeeded: number; failed: number; retried: number; queueDepth: number };
    uptimeMs: number;
    status: "HEALTHY" | "DEGRADED";
  };
  syncStatus: {
    mongoOnline: boolean;
    localStorageHealthy: boolean;
    dualPersistenceActive: boolean;
    failedWrites: number;
  };
}

interface BackupResult {
  ok: boolean;
  period: string;
  filename: string;
  sizeBytes: number;
  createdAt: string;
  error?: string;
}

// ── Helpers ────────────────────────────────────────────────────────────────

function fmtBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1048576).toFixed(1)} MB`;
}

function fmtUptime(ms: number): string {
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
}

function StatusDot({ status }: { status: "ONLINE" | "HEALTHY" | "DEGRADED" | "UNREACHABLE" | "NOT_CONFIGURED" | boolean }) {
  const ok = status === "ONLINE" || status === "HEALTHY" || status === true;
  const warn = status === "UNREACHABLE" || status === "DEGRADED";
  const color = ok ? "#22c55e" : warn ? "#f59e0b" : "#ef4444";
  const label = ok ? "●" : warn ? "◐" : "○";
  return <span style={{ color, fontWeight: 700, marginRight: 6 }} title={String(status)}>{label}</span>;
}

// ── Main component ─────────────────────────────────────────────────────────

export function StorageHealthPanel() {
  const [health, setHealth] = useState<StorageHealthData | null>(null);
  const [loading, setLoading] = useState(false);
  const [backupLoading, setBackupLoading] = useState(false);
  const [lastBackupResults, setLastBackupResults] = useState<BackupResult[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const fetchHealth = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/storage/health");
      const data = await res.json();
      setHealth(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fetch failed");
    } finally {
      setLoading(false);
    }
  }, []);

  const triggerBackup = useCallback(async () => {
    setBackupLoading(true);
    try {
      const res = await fetch("/api/storage/backup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ period: "daily", includeReports: true }),
      });
      const data = await res.json();
      setLastBackupResults(data.results ?? []);
      void fetchHealth();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Backup failed");
    } finally {
      setBackupLoading(false);
    }
  }, [fetchHealth]);

  // Auto-refresh every 30 seconds
  useEffect(() => {
    void fetchHealth();
    const id = setInterval(() => void fetchHealth(), 30_000);
    return () => clearInterval(id);
  }, [fetchHealth]);

  return (
    <DeskCard elevation={1} padding="lg">
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "var(--desk-space-4)" }}>
        <DeskSectionHeader title="Data Storage Health" />
        <div style={{ display: "flex", gap: "var(--desk-space-2)" }}>
          <DeskButton
            variant="outlined"
            onClick={() => void fetchHealth()}
            disabled={loading}
          >
            {loading ? "Refreshing…" : "Refresh"}
          </DeskButton>
          <DeskButton
            variant="filled"
            onClick={() => void triggerBackup()}
            disabled={backupLoading}
          >
            {backupLoading ? "Backing up…" : "Backup Now"}
          </DeskButton>
        </div>
      </div>

      {error && (
        <div style={{ color: "#ef4444", background: "rgba(239,68,68,0.1)", borderRadius: 6, padding: "var(--desk-space-3)", marginBottom: "var(--desk-space-4)", fontSize: 13 }}>
          {error}
        </div>
      )}

      {/* Status row */}
      {health && (
        <>
          <div style={{ display: "flex", gap: "var(--desk-space-2)", marginBottom: "var(--desk-space-4)", flexWrap: "wrap" }}>
            <StatusChip
              label="MongoDB"
              status={health.mongo.status}
              detail={health.mongo.db}
            />
            <StatusChip
              label="Local Storage"
              status={health.localStorage.status}
              detail={health.localStorage.dataRoot.split(/[/\\]/).slice(-3).join("/")}
            />
            <StatusChip
              label="Dual Persistence"
              status={health.syncStatus.dualPersistenceActive ? "ONLINE" : "DEGRADED"}
              detail={health.syncStatus.dualPersistenceActive ? "Active" : "Partial"}
            />
          </div>

          {/* Metric tiles */}
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(140px, 1fr))", gap: "var(--desk-space-3)", marginBottom: "var(--desk-space-4)" }}>
            <DeskMetricTile
              compact
              label="Storage Used"
              value={fmtBytes(health.localStorage.totalMb * 1024 * 1024)}
              detail="local disk"
            />
            <DeskMetricTile
              compact
              label="Files Today"
              value={health.localStorage.filesWrittenToday}
              detail="writes queued"
            />
            <DeskMetricTile
              compact
              label="Backups"
              value={health.localStorage.backupCount}
              detail="daily+weekly+monthly"
            />
            <DeskMetricTile
              compact
              label="Last Backup"
              value={health.localStorage.lastBackupAt
                ? new Date(health.localStorage.lastBackupAt).toLocaleTimeString()
                : "—"}
              detail={health.localStorage.lastBackupAt
                ? new Date(health.localStorage.lastBackupAt).toLocaleDateString()
                : "never"}
            />
            <DeskMetricTile
              compact
              label="Audit Files"
              value={health.localStorage.auditFileCount}
              detail="NDJSON logs"
            />
            <DeskMetricTile
              compact
              label="Write Failures"
              value={health.localStorage.writeFailures}
              valueClassName={health.localStorage.writeFailures > 0 ? "desk-pnl-negative" : "desk-pnl-positive"}
              detail="since start"
            />
            <DeskMetricTile
              compact
              label="Queue Depth"
              value={health.localStorage.queueStats.queueDepth}
              detail={`${health.localStorage.queueStats.succeeded} ok / ${health.localStorage.queueStats.retried} retry`}
            />
            <DeskMetricTile
              compact
              label="Uptime"
              value={fmtUptime(health.localStorage.uptimeMs)}
              detail="persistence svc"
            />
          </div>

          {/* Recent failures */}
          {health.localStorage.recentFailures.length > 0 && (
            <div style={{ marginBottom: "var(--desk-space-4)" }}>
              <span className="desk-label-md" style={{ display: "block", marginBottom: "var(--desk-space-2)", color: "#f59e0b" }}>
                Recent write failures
              </span>
              <div style={{ background: "var(--desk-surface-container)", borderRadius: 6, padding: "var(--desk-space-3)" }}>
                {health.localStorage.recentFailures.map((f, i) => (
                  <div key={i} style={{ fontSize: 12, fontFamily: "monospace", marginBottom: 4, color: "#ef4444" }}>
                    <span style={{ color: "var(--desk-on-surface-dim)" }}>{new Date(f.ts).toLocaleTimeString()} </span>
                    [{f.label}] {f.error}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Last backup results */}
          {lastBackupResults && (
            <div style={{ marginBottom: "var(--desk-space-4)" }}>
              <span className="desk-label-md" style={{ display: "block", marginBottom: "var(--desk-space-2)" }}>
                Last backup results
              </span>
              {lastBackupResults.map((r, i) => (
                <div key={i} style={{ fontSize: 12, fontFamily: "monospace", marginBottom: 4, color: r.ok ? "#22c55e" : "#ef4444" }}>
                  <StatusDot status={r.ok} />
                  {r.period} — {r.filename} ({fmtBytes(r.sizeBytes)})
                  {r.error && ` — ${r.error}`}
                </div>
              ))}
            </div>
          )}

          <div style={{ fontSize: 11, color: "var(--desk-on-surface-dim)", marginTop: "var(--desk-space-2)" }}>
            Refreshed {new Date(health.serverTime).toLocaleTimeString()} · Auto-refresh 30s
          </div>
        </>
      )}

      {!health && !loading && (
        <div style={{ color: "var(--desk-on-surface-dim)", fontSize: 13 }}>
          Click Refresh to load storage status.
        </div>
      )}
    </DeskCard>
  );
}

// ── Status chip ────────────────────────────────────────────────────────────

function StatusChip({
  label,
  status,
  detail,
}: {
  label: string;
  status: string;
  detail: string;
}) {
  const ok = status === "ONLINE" || status === "HEALTHY";
  const warn = status === "UNREACHABLE" || status === "DEGRADED";
  const bg = ok ? "rgba(34,197,94,0.12)" : warn ? "rgba(245,158,11,0.12)" : "rgba(239,68,68,0.12)";
  const fg = ok ? "#22c55e" : warn ? "#f59e0b" : "#ef4444";

  return (
    <div style={{ background: bg, borderRadius: 8, padding: "6px 14px", display: "flex", flexDirection: "column", gap: 2, minWidth: 140 }}>
      <span style={{ fontSize: 11, color: "var(--desk-on-surface-dim)" }}>{label}</span>
      <span style={{ fontSize: 13, fontWeight: 700, color: fg }}>
        <StatusDot status={status as Parameters<typeof StatusDot>[0]["status"]} />
        {status.replace(/_/g, " ")}
      </span>
      <span style={{ fontSize: 10, color: "var(--desk-on-surface-dim)", fontFamily: "monospace" }} title={detail}>
        {detail.length > 24 ? "…" + detail.slice(-22) : detail}
      </span>
    </div>
  );
}
