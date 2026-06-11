import type { ReactNode } from "react";

export function TerminalCard({
  title,
  subtitle,
  actions,
  children,
  className = "",
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`m3-surface-card ${className}`}>
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

export function Metric({
  label,
  value,
  tone = "neutral",
}: {
  label: string;
  value: string;
  tone?: "neutral" | "positive" | "negative" | "warning";
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
      <div className={`m3-metric-tile__value ${toneClass}`}>{value}</div>
    </div>
  );
}
