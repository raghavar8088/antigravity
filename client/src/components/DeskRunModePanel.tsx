"use client";

import { DeskCard, DeskChip } from "@/components/desk/ui";
import { workerHeartbeatAgeSeconds, isWorkerHeartbeatFresh } from "@/lib/paperDeskWorker/workerLease";

type DeskRunModePanelProps = {
  workerLastPollAt: number | null;
  workerOwner: "browser" | "vps" | null;
  workerMonitorMode: boolean;
};

export function DeskRunModePanel({
  workerLastPollAt,
  workerOwner,
  workerMonitorMode,
}: DeskRunModePanelProps) {
  const isFresh = isWorkerHeartbeatFresh(workerLastPollAt);
  const ageSec = workerHeartbeatAgeSeconds(workerLastPollAt);

  const ownerLabel =
    workerOwner === "vps"
      ? "VPS worker"
      : workerOwner === "browser"
        ? "Browser"
        : "Unknown";

  const heartbeatLabel =
    ageSec === null ? "Never" : ageSec < 60 ? `${ageSec}s ago` : `${Math.round(ageSec / 60)}m ago`;

  return (
    <DeskCard padding="md">
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          flexWrap: "wrap",
          fontSize: 13,
        }}
      >
        <span className="desk-label-sm" style={{ marginRight: 4 }}>
          Execution mode
        </span>
        <DeskChip tone={isFresh ? "success" : "warning"}>
          {isFresh ? `24/7 worker ACTIVE — last tick ${heartbeatLabel}` : "Browser-only — keep tab visible"}
        </DeskChip>
        {workerOwner && (
          <DeskChip tone="default">Owner: {ownerLabel}</DeskChip>
        )}
        {workerMonitorMode && (
          <DeskChip tone="default">Monitor mode — writes owned by worker</DeskChip>
        )}
        {!isFresh && (
          <span className="desk-label-sm" style={{ color: "var(--color-text-secondary)" }}>
            Start worker: <code>cd client && npx tsx scripts/btc-ft-paper-worker.ts</code>
          </span>
        )}
      </div>
    </DeskCard>
  );
}
