"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { terminalAuthorityLabel } from "@/lib/terminal/terminalAuthority";
import { pct, px, usd } from "./format";

const nav = [
  { href: "/terminal/execution", label: "Execution" },
  { href: "/terminal/risk", label: "Risk" },
  { href: "/terminal/research", label: "Research" },
  { href: "/terminal/strategies", label: "Strategies" },
  { href: "/terminal/analytics", label: "Analytics" },
  { href: "/terminal/portfolio", label: "Portfolio" },
  { href: "/terminal/events", label: "Events" },
  { href: "/terminal/journal", label: "Journal" },
];

export function InstitutionalTerminalShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const snapshot = useTerminalSnapshot();
  const criticalAlerts = snapshot.alerts.filter((a) => a.severity === "CRITICAL").length;
  const authorityLabel = terminalAuthorityLabel(snapshot);
  const equityDisplay = snapshot.risk.netExposureUsd > 0
    ? usd(snapshot.risk.netExposureUsd + (snapshot.risk.grossExposureUsd > 0 ? 0 : 0), { compact: true })
    : snapshot.updatedAt
      ? usd(snapshot.risk.grossExposureUsd, { compact: true })
      : "—";

  return (
    <div className="min-h-screen bg-[#080b10] text-zinc-100">
      <header className="sticky top-0 z-40 border-b border-zinc-800 bg-[#0b0f16]/95 backdrop-blur">
        <div className="flex min-h-[54px] flex-wrap items-center gap-3 px-3 lg:px-4">
          <Link href="/terminal/execution" className="flex items-center gap-2">
            <span className="flex h-8 w-8 items-center justify-center rounded-md border border-amber-500/30 bg-amber-500/10 font-mono text-sm font-bold text-amber-300">
              BTC
            </span>
            <span className="hidden text-xs font-semibold uppercase tracking-[0.18em] text-zinc-400 sm:inline">
              Institutional Terminal
            </span>
          </Link>
          <div className="h-7 w-px bg-zinc-800" />
          <div className="font-mono text-lg font-semibold text-zinc-50">
            {snapshot.price > 0 ? `$${px(snapshot.price)}` : "—"}
          </div>
          <div className={snapshot.priceChange24hPct >= 0 ? "font-mono text-xs text-emerald-400" : "font-mono text-xs text-rose-400"}>
            {snapshot.price > 0 ? pct(snapshot.priceChange24hPct) : "—"}
          </div>
          <div className="hidden gap-3 text-[11px] text-zinc-400 md:flex">
            <span>Spread <b className="font-mono text-zinc-200">{snapshot.spreadBps > 0 ? `${snapshot.spreadBps.toFixed(1)}bps` : "—"}</b></span>
            <span>Funding <b className="font-mono text-amber-300">{snapshot.fundingRate !== 0 ? pct(snapshot.fundingRate * 100, 4) : "—"}</b></span>
            <span>Regime <b className="text-sky-300">{snapshot.regime || "—"}</b></span>
            <span>Heat <b className="font-mono text-emerald-300">{snapshot.risk.heatPct > 0 ? `${snapshot.risk.heatPct.toFixed(1)}%` : "—"}</b></span>
            <span>Exposure <b className="font-mono text-zinc-200">{equityDisplay}</b></span>
          </div>
          <div className="ml-auto flex items-center gap-2">
            <span className={`rounded-full px-2 py-1 text-[10px] font-semibold uppercase ${
              snapshot.hasAuthority
                ? snapshot.authoritySource === "ws"
                  ? "bg-emerald-500/10 text-emerald-300"
                  : "bg-sky-500/10 text-sky-300"
                : "bg-rose-500/10 text-rose-300"
            }`}>
              {authorityLabel}
            </span>
            {criticalAlerts > 0 ? (
              <span className="rounded-full bg-rose-500/15 px-2 py-1 text-[10px] font-semibold uppercase text-rose-300">
                {criticalAlerts} Critical
              </span>
            ) : null}
          </div>
        </div>
        <nav className="flex gap-1 overflow-x-auto border-t border-zinc-900 px-2 py-1 lg:px-4">
          {nav.map((item) => {
            const active = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`rounded-md px-3 py-2 text-xs font-medium uppercase tracking-[0.12em] transition ${
                  active ? "bg-sky-500/15 text-sky-200" : "text-zinc-500 hover:bg-zinc-900 hover:text-zinc-200"
                }`}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>
      </header>
      <main className="p-2 lg:p-4">{children}</main>
    </div>
  );
}
