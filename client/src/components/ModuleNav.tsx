"use client";

/**
 * ModuleNav — global module switcher (jio-finance-style slide-out sidebar).
 *
 * A fixed launcher button (top-left, below the risk ribbon) opens a drawer
 * listing every live module grouped by area, with the active route
 * highlighted. Rendered as an overlay so dense pages (terminal) keep their
 * full-width layouts. Fully additive: no page markup is touched.
 */

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";

type NavItem = {
  href: string;
  label: string;
  hint?: string;
  external?: boolean;
};

type NavSection = { title: string; items: NavItem[] };

const SECTIONS: NavSection[] = [
  {
    title: "Crypto Desks",
    items: [
      { href: "/terminal", label: "Command Center", hint: "ICC terminal" },
      { href: "/btc-pre-live", label: "BTC Pre-Live Engine", hint: "49 qualified · paper week" },
      { href: "/scalp-desk", label: "Crypto Scalp Desk", hint: "800 paper streams · 1m" },
      { href: "/mock-trading", label: "Mock Trading", hint: "validation desk" },
    ],
  },
  {
    title: "Scalp Desk API",
    items: [
      { href: "/api/scalp/scalp/leaderboard", label: "Leaderboard JSON", external: true },
      { href: "/api/scalp/scalp/stats", label: "Desk Totals JSON", external: true },
      { href: "/api/scalp/scalp/trades?n=100", label: "Recent Trades JSON", external: true },
      { href: "/api/scalp/scalp/health", label: "Engine Health JSON", external: true },
    ],
  },
  {
    title: "System",
    items: [{ href: "/mobile", label: "Mobile Emergency View", hint: "kill switch · flat all" }],
  },
];

export default function ModuleNav() {
  const [open, setOpen] = useState(false);
  const pathname = usePathname();

  // close the drawer on every route change and on Escape
  useEffect(() => setOpen(false), [pathname]);
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  // hide entirely on auth screens
  if (pathname === "/login" || pathname === "/sign-in") return null;

  return (
    <>
      <button
        type="button"
        aria-label="Open module menu"
        onClick={() => setOpen(true)}
        className="fixed left-3 top-3 z-40 flex h-9 w-9 items-center justify-center rounded-full border border-zinc-200 bg-white text-zinc-700 shadow-sm transition-transform hover:scale-105 hover:text-zinc-900"
      >
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M2 4h12M2 8h12M2 12h12" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
        </svg>
      </button>

      {open && (
        <div className="fixed inset-0 z-50">
          <button
            type="button"
            aria-label="Close module menu"
            onClick={() => setOpen(false)}
            className="absolute inset-0 h-full w-full cursor-default bg-zinc-900/40"
          />
          <aside className="absolute left-0 top-0 flex h-full w-[280px] flex-col overflow-y-auto border-r border-zinc-200 bg-white shadow-2xl">
            {/* Brand */}
            <div className="flex items-center gap-2.5 border-b border-zinc-100 px-5 py-4">
              <span className="flex h-8 w-8 items-center justify-center rounded-xl bg-zinc-900 text-[13px] font-extrabold text-white">
                A
              </span>
              <div>
                <div className="text-[14px] font-extrabold leading-tight text-zinc-900">Antigravity</div>
                <div className="text-[10px] uppercase tracking-[0.18em] text-zinc-400">Trading Modules</div>
              </div>
            </div>

            {/* Sections */}
            <nav className="flex-1 px-3 py-3">
              {SECTIONS.map((sec) => (
                <div key={sec.title} className="mb-4">
                  <div className="px-2 pb-1.5 text-[10px] font-bold uppercase tracking-[0.18em] text-zinc-400">
                    {sec.title}
                  </div>
                  <ul className="flex flex-col gap-0.5">
                    {sec.items.map((item) => {
                      const active = !item.external && pathname === item.href;
                      const cls = `flex items-center justify-between gap-2 rounded-lg px-3 py-2 text-[13px] font-semibold transition-colors ${
                        active
                          ? "bg-violet-50 text-violet-800"
                          : "text-zinc-600 hover:bg-zinc-50 hover:text-zinc-900"
                      }`;
                      const body = (
                        <>
                          <span className="flex flex-col">
                            {item.label}
                            {item.hint && (
                              <span className="text-[10px] font-medium text-zinc-400">{item.hint}</span>
                            )}
                          </span>
                          {item.external ? (
                            <span className="text-[10px] font-bold text-zinc-300">↗</span>
                          ) : (
                            active && <span className="h-1.5 w-1.5 rounded-full bg-violet-500" />
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

            <div className="border-t border-zinc-100 px-5 py-3 text-[10px] leading-relaxed text-zinc-400">
              Scalp Desk is paper-only. Real money requires the pre-registered gate — never leaderboard position alone.
            </div>
          </aside>
        </div>
      )}
    </>
  );
}
