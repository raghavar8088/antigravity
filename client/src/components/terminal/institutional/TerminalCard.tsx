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
    <section className={`rounded-xl border border-zinc-800 bg-[#0d1118] ${className}`}>
      <header className="flex min-h-10 items-center justify-between gap-3 border-b border-zinc-800 px-3 py-2">
        <div>
          <h2 className="text-[11px] font-semibold uppercase tracking-[0.16em] text-zinc-200">{title}</h2>
          {subtitle ? <p className="mt-0.5 text-[10px] text-zinc-500">{subtitle}</p> : null}
        </div>
        {actions}
      </header>
      <div className="p-3">{children}</div>
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
  const cls =
    tone === "positive"
      ? "text-emerald-300"
      : tone === "negative"
      ? "text-rose-300"
      : tone === "warning"
      ? "text-amber-300"
      : "text-zinc-100";
  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-950/40 px-3 py-2">
      <div className="text-[10px] uppercase tracking-[0.12em] text-zinc-500">{label}</div>
      <div className={`mt-1 font-mono text-sm font-semibold ${cls}`}>{value}</div>
    </div>
  );
}
