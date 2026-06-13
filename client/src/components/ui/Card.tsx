"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

export type CardVariant = "default" | "elevated" | "danger" | "warning";

type CardProps = {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  /** Visual variant controlling border and background tint */
  variant?: CardVariant;
};

const variantClass: Record<CardVariant, string> = {
  default:  "m3-surface-card",
  elevated: "m3-surface-card border-[var(--color-border)] shadow-[var(--shadow-elevated)]",
  danger:   "m3-surface-card border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.04)]",
  warning:  "m3-surface-card border-[rgba(249,115,22,0.3)] bg-[rgba(249,115,22,0.04)]",
};

export function Card({ title, subtitle, actions, children, className, variant = "default" }: CardProps) {
  return (
    <section className={cn(variantClass[variant], className)}>
      <header className="m3-surface-card__header">
        <div>
          <h2 className="m3-surface-card__title">{title}</h2>
          {subtitle ? <p className="m3-surface-card__subtitle">{subtitle}</p> : null}
        </div>
        {actions}
      </header>
      <div className="m3-surface-card__body">{children}</div>
    </section>
  );
}

type MetricTone = "neutral" | "positive" | "negative" | "warning";

export function Metric({
  label,
  value,
  tone = "neutral",
}: {
  label: string;
  value: string;
  tone?: MetricTone;
}) {
  const toneClass =
    tone === "positive"
      ? "m3-metric-tile__value--profit"
      : tone === "negative"
      ? "m3-metric-tile__value--loss"
      : tone === "warning"
      ? "m3-metric-tile__value--warning"
      : "";

  return (
    <div className="m3-metric-tile">
      <div className="m3-metric-tile__label">{label}</div>
      <div className={cn("m3-metric-tile__value", toneClass)}>{value}</div>
    </div>
  );
}
