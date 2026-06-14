import { Children, isValidElement, type ReactNode } from "react";

type MetricTone = "neutral" | "positive" | "negative" | "warning";
type MetricSize = "sm" | "md" | "lg";

type MetricProps = {
  label: string;
  value: string;
  tone?: MetricTone;
  delta?: string;
  size?: MetricSize;
  children?: ReactNode;
};

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
  const accentTone = getMetricAccentTone(children);
  const accentClass =
    accentTone === "positive"
      ? "m3-surface-card--tone-positive"
      : accentTone === "negative"
      ? "m3-surface-card--tone-negative"
      : accentTone === "warning"
      ? "m3-surface-card--tone-warning"
      : "";

  return (
    <section className={["m3-surface-card", accentClass, className].filter(Boolean).join(" ")}>
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
  delta,
  size = "md",
  children,
}: MetricProps) {
  const toneClass =
    tone === "positive"
      ? "m3-metric-tile__value--profit"
      : tone === "negative"
      ? "m3-metric-tile__value--loss"
      : tone === "warning"
      ? "m3-metric-tile__value--warning"
      : "";
  const sizeClass = `m3-metric-tile__value--${size}`;
  const deltaToneClass = delta?.trim().startsWith("+")
    ? "m3-metric-tile__delta--positive"
    : delta?.trim().startsWith("-")
    ? "m3-metric-tile__delta--negative"
    : "";

  return (
    <div className="m3-metric-tile">
      <div className="m3-metric-tile__label">{label}</div>
      <div className={["m3-metric-tile__value", sizeClass, toneClass].filter(Boolean).join(" ")}>{value}</div>
      {children ? <div className="mt-1 flex min-h-7 items-end">{children}</div> : null}
      {delta ? <div className={["m3-metric-tile__delta", deltaToneClass].filter(Boolean).join(" ")}>{delta}</div> : null}
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
      const tone = (child.props as Partial<MetricProps>).tone;
      if (tone && tone !== "neutral") tones.add(tone);
    }

    const nestedChildren = (child.props as { children?: ReactNode }).children;
    if (nestedChildren) collectMetricTones(nestedChildren, tones);
  });
}
