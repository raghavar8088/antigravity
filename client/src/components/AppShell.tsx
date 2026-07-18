"use client";

/**
 * AppShell — app-wide TradingAI-style navigation shell.
 *
 * Desktop (lg+): permanent 250px left sidebar — brand block, grouped module
 * sections with icons, active route highlighted as a violet pill. Content
 * (including the global risk ribbon) flows in the column to its right.
 * Below lg: the sidebar becomes a slide-out drawer behind a floating button.
 * Auth screens render bare. Fully additive: page internals are untouched.
 */

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";

type NavItem = {
  href: string;
  label: string;
  icon: ReactNode;
  external?: boolean;
};

type NavSection = { title: string; items: NavItem[] };

const ic = {
  monitor: (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="2" y="3" width="20" height="14" rx="2" />
      <path d="M8 21h8M12 17v4" />
    </svg>
  ),
  trend: (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M22 7l-8.5 8.5-5-5L2 17" />
      <path d="M16 7h6v6" />
    </svg>
  ),
  bolt: (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />
    </svg>
  ),
  layers: (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
    </svg>
  ),
  code: (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M16 18l6-6-6-6M8 6l-6 6 6 6" />
    </svg>
  ),
  phone: (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="5" y="2" width="14" height="20" rx="2" />
      <path d="M12 18h.01" />
    </svg>
  ),
};

const SECTIONS: NavSection[] = [
  {
    title: "Trading",
    items: [
      { href: "/terminal", label: "Command Center", icon: ic.monitor },
      { href: "/btc-pre-live", label: "BTC Pre-Live Engine", icon: ic.trend },
      { href: "/scalp-desk", label: "Crypto Scalp Desk", icon: ic.bolt },
      { href: "/mock-trading", label: "Mock Trading", icon: ic.layers },
    ],
  },
  {
    title: "Scalp Lab · API",
    items: [
      { href: "/api/scalp/scalp/leaderboard", label: "Leaderboard JSON", icon: ic.code, external: true },
      { href: "/api/scalp/scalp/stats", label: "Desk Totals JSON", icon: ic.code, external: true },
      { href: "/api/scalp/scalp/trades?n=100", label: "Recent Trades JSON", icon: ic.code, external: true },
      { href: "/api/scalp/scalp/health", label: "Engine Health JSON", icon: ic.code, external: true },
    ],
  },
  {
    title: "System",
    items: [{ href: "/mobile", label: "Mobile Emergency", icon: ic.phone }],
  },
];

function SidebarBody({ pathname }: { pathname: string }) {
  return (
    <>
      <div className="flex items-center gap-2.5 px-5 py-5">
        <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-violet-600 text-white">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M22 7l-8.5 8.5-5-5L2 17" />
            <path d="M16 7h6v6" />
          </svg>
        </span>
        <div className="text-[16px] font-extrabold tracking-tight text-zinc-900">Antigravity</div>
      </div>

      <nav className="flex-1 overflow-y-auto px-3 pb-4">
        {SECTIONS.map((sec) => (
          <div key={sec.title} className="mb-5">
            <div className="px-3 pb-1.5 text-[10px] font-bold uppercase tracking-[0.16em] text-zinc-400">
              {sec.title}
            </div>
            <ul className="flex flex-col gap-0.5">
              {sec.items.map((item) => {
                const active = !item.external && pathname.startsWith(item.href);
                const cls = `flex items-center gap-3 rounded-xl px-3 py-2 text-[13.5px] font-semibold transition-colors ${
                  active
                    ? "bg-violet-100 text-violet-800"
                    : "text-zinc-500 hover:bg-zinc-50 hover:text-zinc-900"
                }`;
                const body = (
                  <>
                    <span className={active ? "text-violet-700" : "text-zinc-400"}>{item.icon}</span>
                    <span className="flex-1">{item.label}</span>
                    {item.external && <span className="text-[10px] font-bold text-zinc-300">↗</span>}
                  </>
                );
                return (
                  <li key={item.href}>
                    {item.external ? (
                      <a href={item.href} target="_blank" rel="noreferrer" className={cls}>
                        {body}
                      </a>
                    ) : (
                      <Link href={item.href} className={cls}>
                        {body}
                      </Link>
                    )}
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>

      <div className="border-t border-zinc-100 px-5 py-3 text-[10px] leading-relaxed text-zinc-400">
        Scalp Desk is paper-only. Real money requires the pre-registered gate — never leaderboard position alone.
      </div>
    </>
  );
}

export default function AppShell({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const pathname = usePathname();

  useEffect(() => setOpen(false), [pathname]);
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  if (pathname === "/login" || pathname === "/sign-in") return <>{children}</>;

  return (
    <div className="min-h-screen">
      {/* Desktop: permanent sidebar */}
      <aside className="fixed inset-y-0 left-0 z-40 hidden w-[250px] flex-col border-r border-zinc-200 bg-white lg:flex">
        <SidebarBody pathname={pathname} />
      </aside>

      {/* Mobile: floating launcher + drawer */}
      <button
        type="button"
        aria-label="Open module menu"
        onClick={() => setOpen(true)}
        className="fixed left-3 top-3 z-40 flex h-9 w-9 items-center justify-center rounded-full border border-zinc-200 bg-white text-zinc-700 shadow-sm lg:hidden"
      >
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M2 4h12M2 8h12M2 12h12" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
        </svg>
      </button>
      {open && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            type="button"
            aria-label="Close module menu"
            onClick={() => setOpen(false)}
            className="absolute inset-0 h-full w-full cursor-default bg-zinc-900/40"
          />
          <aside className="absolute left-0 top-0 flex h-full w-[270px] flex-col overflow-y-auto border-r border-zinc-200 bg-white shadow-2xl">
            <SidebarBody pathname={pathname} />
          </aside>
        </div>
      )}

      {/* Content column */}
      <div className="lg:pl-[250px]">{children}</div>
    </div>
  );
}
