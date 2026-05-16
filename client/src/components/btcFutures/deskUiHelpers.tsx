"use client";

import { DeskChip } from "@/components/desk/ui";

export function parseExitReasonSummary(summary: string): { reason: string; count: number; avg: string }[] {
  if (!summary || summary === "—") return [];
  return summary.split(" · ").map((part) => {
    const m = part.match(/^(\w+)×(\d+)\s+(avg[+-]?\d+\.\d+)$/);
    if (!m) return { reason: part, count: 0, avg: "" };
    return { reason: m[1], count: Number(m[2]), avg: m[3] };
  });
}

export function exitReasonChipTone(reason: string): "success" | "error" | "warning" | "primary" | "default" {
  if (reason === "TP" || reason === "PROFIT_LOCK") return reason === "TP" ? "success" : "primary";
  if (reason === "SL") return "error";
  if (reason === "TIME") return "default";
  return "default";
}

export function ExitReasonChip({ reason }: { reason: string }) {
  return <DeskChip tone={exitReasonChipTone(reason)}>{reason}</DeskChip>;
}

export function SideChip({ side }: { side: "LONG" | "SHORT" }) {
  return <DeskChip tone={side === "LONG" ? "success" : "error"}>{side}</DeskChip>;
}

export function StrategyStatusChip({
  status,
  disabled,
}: {
  status: string;
  disabled?: boolean;
}) {
  if (disabled) return <DeskChip tone="default">Disabled</DeskChip>;
  const label = status.replace(/_/g, " ");
  if (status === "IN_POSITION") return <DeskChip tone="primary">{label}</DeskChip>;
  if (status === "COOLING") return <DeskChip tone="warning">{label}</DeskChip>;
  if (status === "READY") return <DeskChip tone="success">{label}</DeskChip>;
  return <DeskChip>{label}</DeskChip>;
}
