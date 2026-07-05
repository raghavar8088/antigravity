"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

type StatusTone = "success" | "warning" | "error" | "info" | "neutral";
/** Reference design system's tone vocabulary — aliases onto the tones above. */
type ReferenceStatusTone = "gain" | "loss" | "warn" | "accent" | "muted";

const toneClass: Record<StatusTone, string> = {
  success: "m3-status-chip--success",
  warning: "m3-status-chip--warning",
  error: "m3-status-chip--error",
  info: "m3-status-chip--info",
  neutral: "m3-status-chip--neutral",
};

const referenceToneAlias: Record<ReferenceStatusTone, StatusTone> = {
  gain: "success",
  loss: "error",
  warn: "warning",
  accent: "info",
  muted: "neutral",
};

function resolveTone(tone: StatusTone | ReferenceStatusTone): StatusTone {
  return tone in referenceToneAlias ? referenceToneAlias[tone as ReferenceStatusTone] : (tone as StatusTone);
}

export function StatusChip({
  label,
  tone = "neutral",
  dot = true,
  /** Animate the dot with a pulse (scale+opacity), for "live" indicators. */
  pulse = false,
  className,
}: {
  label: string;
  tone?: StatusTone | ReferenceStatusTone;
  dot?: boolean;
  pulse?: boolean;
  className?: string;
}) {
  const isOffline = label.toLowerCase() === "offline" || label.toLowerCase().includes("unavailable");
  const resolvedTone = resolveTone(tone);

  return (
    <span className={cn("m3-status-chip", toneClass[resolvedTone], isOffline && "m3-status-chip--offline", className)}>
      {dot ? <span className={cn("m3-status-chip__dot", pulse && "m3-status-chip__dot--pulse")} aria-hidden /> : null}
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

/** Former ui/Badge.tsx variant vocabulary — aliases onto StatusTone, kept for that component's ~9 call sites. */
export type BadgeVariant = "profit" | "loss" | "warning" | "info" | "caution" | "neutral";

const badgeVariantAlias: Record<BadgeVariant, StatusTone> = {
  profit: "success",
  loss: "error",
  warning: "warning",
  info: "info",
  caution: "warning",
  neutral: "neutral",
};

export function Badge({
  label,
  tone,
  variant,
  size = "md",
  dot = false,
  className,
  children,
}: {
  label?: string | number;
  tone?: StatusTone | ReferenceStatusTone;
  /** Legacy ui/Badge.tsx prop name — alias for tone. */
  variant?: BadgeVariant;
  size?: "sm" | "md";
  dot?: boolean;
  className?: string;
  children?: ReactNode;
}) {
  const resolvedTone = tone ? resolveTone(tone) : variant ? badgeVariantAlias[variant] : "neutral";
  return (
    <span className={cn("m3-badge", size === "sm" && "m3-badge--sm", toneClass[resolvedTone], className)}>
      {dot ? <span className="m3-badge__dot" aria-hidden /> : null}
      {label ?? children}
    </span>
  );
}
