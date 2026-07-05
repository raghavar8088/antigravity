"use client";

import { Children, isValidElement, type ReactNode } from "react";
import { cn } from "./cn";
import { Sparkline } from "./Sparkline";

export type CardVariant = "default" | "elevated" | "danger" | "warning";
/** "lavender" mirrors the reference design system's tinted-panel variant (canvas-edge bg, no border). */
export type CardTint = "white" | "lavender";

type CardProps = {
  title: string;
  subtitle?: string;
  /** Alias for subtitle, matching the reference GlassPanel's `note` prop naming. */
  note?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  /** Visual variant controlling border and background tint */
  variant?: CardVariant;
  /** Panel background tint — "lavender" matches the reference design system's tinted-panel look. */
  tint?: CardTint;
  /**
   * When true, a left accent border is auto-derived from any child
   * <Metric tone="..."> — negative wins over positive over warning. Ported
   * from the former TerminalCard component, which always did this — default
   * is false here to preserve plain Card's pre-merge behavior for existing callers.
   */
  autoTone?: boolean;
};

const variantClass: Record<CardVariant, string> = {
  default:  "m3-surface-card",
  elevated: "m3-surface-card border-[var(--color-border)] shadow-[var(--shadow-elevated)]",
  danger:   "m3-surface-card border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.04)]",
  warning:  "m3-surface-card border-[rgba(249,115,22,0.3)] bg-[rgba(249,115,22,0.04)]",
};

export function Card({
  title,
  subtitle,
  note,
  actions,
  children,
  className,
  variant = "default",
  tint = "white",
  autoTone = false,
}: CardProps) {
  const accentTone = autoTone ? getMetricAccentTone(children) : "neutral";
  const accentClass =
    accentTone === "positive"
      ? "m3-surface-card--tone-positive"
      : accentTone === "negative"
      ? "m3-surface-card--tone-negative"
      : accentTone === "warning"
      ? "m3-surface-card--tone-warning"
      : "";

  return (
    <section
      className={cn(variantClass[variant], accentClass, tint === "lavender" && "m3-surface-card--tint-lavender", className)}
    >
      <header className="m3-surface-card__header">
        <div>
          <h2 className="m3-surface-card__title">{title}</h2>
          {(subtitle ?? note) ? <p className="m3-surface-card__subtitle">{subtitle ?? note}</p> : null}
        </div>
        {actions}
      </header>
      <div className="m3-surface-card__body">{children}</div>
    </section>
  );
}

type MetricTone = "neutral" | "positive" | "negative" | "warning";
type MetricSize = "sm" | "md" | "lg";

export function Metric({
  label,
  value,
  tone = "neutral",
  delta,
  size = "md",
  /** Optional sparkline series rendered under the value, matching the reference StatCard. */
  sparkline,
  children,
}: {
  label: string;
  value: string;
  tone?: MetricTone;
  delta?: string;
  size?: MetricSize;
  sparkline?: number[];
  children?: ReactNode;
}) {
  const toneClass =
    tone === "positive"
      ? "m3-metric-tile__value--profit"
      : tone === "negative"
      ? "m3-metric-tile__value--loss"
      : tone === "warning"
      ? "m3-metric-tile__value--warning"
      : "";
  const deltaToneClass = delta?.trim().startsWith("+")
    ? "m3-metric-tile__delta--positive"
    : delta?.trim().startsWith("-")
    ? "m3-metric-tile__delta--negative"
    : "";
  const sparklineColor =
    tone === "positive" ? "var(--gain, var(--color-profit))" : tone === "negative" ? "var(--loss, var(--color-loss))" : undefined;

  return (
    <div className="m3-metric-tile">
      <div className="m3-metric-tile__label">{label}</div>
      <div className={cn("m3-metric-tile__value", `m3-metric-tile__value--${size}`, toneClass)}>{value}</div>
      {children ? <div className="mt-1 flex min-h-7 items-end">{children}</div> : null}
      {sparkline && sparkline.length > 1 ? (
        <div className="mt-1 flex min-h-7 items-end">
          <Sparkline data={sparkline} color={sparklineColor} />
        </div>
      ) : null}
      {delta ? <div className={cn("m3-metric-tile__delta", deltaToneClass)}>{delta}</div> : null}
    </div>
  );
}

function getMetricAccentTone(children: ReactNode): MetricTone {
  const tones = new Set<MetricTone>();
  collectMetricTones(children, tones);

  if (tones.has("negative")) return "negative";
  if (tones.has("positive")) return "positive";
  if (tones.has("warning")) return "warning";
  return "neutral";
}

function collectMetricTones(node: ReactNode, tones: Set<MetricTone>) {
  Children.forEach(node, (child) => {
    if (!isValidElement(child)) return;

    if (child.type === Metric) {
      const tone = (child.props as Partial<{ tone: MetricTone }>).tone;
      if (tone && tone !== "neutral") tones.add(tone);
    }

    const nestedChildren = (child.props as { children?: ReactNode }).children;
    if (nestedChildren) collectMetricTones(nestedChildren, tones);
  });
}
