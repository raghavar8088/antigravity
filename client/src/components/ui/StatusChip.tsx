"use client";

import { cn } from "./cn";

type StatusTone = "success" | "warning" | "error" | "info" | "neutral";

const toneClass: Record<StatusTone, string> = {
  success: "m3-status-chip--success",
  warning: "m3-status-chip--warning",
  error: "m3-status-chip--error",
  info: "m3-status-chip--info",
  neutral: "m3-status-chip--neutral",
};

export function StatusChip({
  label,
  tone = "neutral",
  dot = true,
  className,
}: {
  label: string;
  tone?: StatusTone;
  dot?: boolean;
  className?: string;
}) {
  return (
    <span className={cn("m3-status-chip", toneClass[tone], className)}>
      {dot ? <span className="m3-status-chip__dot" aria-hidden /> : null}
      {label}
    </span>
  );
}

export function Chip({
  label,
  selected = false,
  onClick,
}: {
  label: string;
  selected?: boolean;
  onClick?: () => void;
}) {
  const Tag = onClick ? "button" : "span";
  return (
    <Tag
      type={onClick ? "button" : undefined}
      className={cn("m3-chip", selected && "m3-chip--selected")}
      onClick={onClick}
    >
      {label}
    </Tag>
  );
}

export function Badge({ label, tone = "neutral" }: { label: string | number; tone?: StatusTone }) {
  return <span className={cn("m3-badge", toneClass[tone])}>{label}</span>;
}
