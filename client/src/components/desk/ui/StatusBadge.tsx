"use client";

import { DeskChip } from "./DeskChip";

export type DeskEngineStatus = "live" | "paused" | "degraded" | "syncing" | "cloud-off";

const config: Record<
  DeskEngineStatus,
  { label: string; tone: "success" | "warning" | "error" | "default" | "primary"; dot?: boolean }
> = {
  live: { label: "Live", tone: "success", dot: true },
  paused: { label: "Paused", tone: "warning", dot: true },
  degraded: { label: "Degraded data", tone: "warning" },
  syncing: { label: "Syncing", tone: "primary" },
  "cloud-off": { label: "Cloud sync off", tone: "default" },
};

export function StatusBadge({ status }: { status: DeskEngineStatus }) {
  const c = config[status];
  return (
    <DeskChip tone={c.tone}>
      {c.dot ? (
        <span
          aria-hidden
          style={{
            width: 8,
            height: 8,
            borderRadius: "50%",
            background: "currentColor",
            opacity: status === "paused" ? 0.8 : 1,
          }}
        />
      ) : null}
      {c.label}
    </DeskChip>
  );
}
