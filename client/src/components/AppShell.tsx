"use client";

/**
 * AppShell — app-wide navigation shell.
 *
 * Structure follows the TradingAI reference (permanent 250px left sidebar,
 * grouped module sections, active pill); the visual language follows
 * JioFinance (white surfaces, generous spacing, warm gold accent, heavy
 * display type). Desktop lg+: permanent sidebar. Below lg: slide-out
 * drawer behind a floating launcher. Auth screens render bare.
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

const sora = { fontFamily: "var(--font-sora), system-ui, sans-serif" };

const ic = {
  monitor: (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="2" y="3" width="20" height="14" rx="2" />
      <path d="M8 21h8M12 17v4" />
    </svg>
  ),
  trend: (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M22 7l-8.5 8.5-5-5L2 17" />
      <path d="M16 7h6v6" />
    </svg>
  ),
  bolt: (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />
    </svg>
  ),
  layers: (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
    </svg>
  ),
  code: (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M16 18l6-6-6-6M8 6l-6 6 6 6" />
    </svg>
  ),
  phone: (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
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
      <Link href="/terminal" className="flex items-center gap-3 px-6 pb-6 pt-7">
        <span className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-amber-400 to-amber-600 text-white shadow-sm">
          <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M22 7l-8.5 8.5-5-5L2 17" />
            <path d="M16 7h6v6" />
          </svg>
        </span>
        <div>
          <div className="text-[17px] font-extrabold leading-tight tracking-tight text-zinc-900" style={sora}>
            Antigravity
          </div>
          <div className="text-[10px] font-semibold uppercase tracking-[0.2em] text-amber-700/70">Trading Suite</div>
        </div>
      </Link>

      <nav className="flex-1 overflow-y-auto px-4 pb-4">
        {SECTIONS.map((sec) => (
          <div key={sec.title} className="mb-6">
            <div className="px-3 pb-2 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-400">{sec.title}</div>
            <ul className="flex flex-col gap-1">
              {sec.items.map((item) => {
                const active = !item.external && pathname.startsWith(item.href);
                const cls = `flex items-center gap-3 rounded-full px-4 py-2.5 text-[13.5px] font-semibold transition-colors ${
                  active ? "bg-amber-50 text-amber-900" : "text-zinc-500 hover:bg-zinc-50 hover:text-zinc-900"
                }`;
                const body = (
                  <>
                    <span className={active ? "text-amber-600" : "text-zinc-400"}>{item.icon}</span>
                    <span className="flex-1">{item.label}</span>
                    {item.external ? (
                      <span className="text-[10px] font-bold text-zinc-300">↗</span>
                    ) : (
                      active && <span className="h-1.5 w-1.5 rounded-full bg-amber-500" />
                    )}
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

      <div className="mx-4 mb-5 rounded-2xl bg-zinc-50 px-4 py-3.5 text-[10.5px] leading-relaxed text-zinc-500">
        <span className="font-bold text-zinc-700">Paper only.</span> Real money requires the pre-registered gate — never
        leaderboard position alone.
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
    <div className="min-h-screen bg-white">
      {/* Desktop: permanent sidebar */}
      <aside className="fixed inset-y-0 left-0 z-40 hidden w-[250px] flex-col border-r border-zinc-100 bg-white lg:flex">
        <SidebarBody pathname={pathname} />
      </aside>

      {/* Mobile: floating launcher + drawer */}
      <button
        type="button"
        aria-label="Open module menu"
        onClick={() => setOpen(true)}
        className="fixed left-3 top-3 z-40 flex h-10 w-10 items-center justify-center rounded-full border border-zinc-200 bg-white text-zinc-700 shadow-md lg:hidden"
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
          <aside className="absolute left-0 top-0 flex h-full w-[280px] flex-col overflow-y-auto border-r border-zinc-100 bg-white shadow-2xl">
            <SidebarBody pathname={pathname} />
          </aside>
        </div>
      )}

      {/* Content column */}
      <div className="lg:pl-[250px]">{children}</div>
    </div>
  );
}
