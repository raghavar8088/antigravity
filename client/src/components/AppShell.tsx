"use client";

/**
 * AppShell — app-wide navigation shell, modeled on the TradingAI terminal.
 *
 * Robust flex layout: a sticky 250px sidebar as a flex item (never overlaps
 * content — the earlier fixed+padding version let the sidebar cover the
 * page). Palette follows TradingAI: white surfaces, violet primary (logo +
 * active nav pill), amber reserved for CTAs. Below lg the sidebar collapses
 * to a slide-out drawer. Auth screens render bare.
 */

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";

type NavItem = { href: string; label: string; icon: ReactNode; external?: boolean };
type NavSection = { title: string; items: NavItem[] };

const sora = { fontFamily: "var(--font-sora), system-ui, sans-serif" };

const ic = {
  monitor: (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="2" y="3" width="20" height="14" rx="2" /><path d="M8 21h8M12 17v4" />
    </svg>
  ),
  trend: (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M22 7l-8.5 8.5-5-5L2 17" /><path d="M16 7h6v6" />
    </svg>
  ),
  bolt: (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />
    </svg>
  ),
  layers: (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
    </svg>
  ),
  code: (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M16 18l6-6-6-6M8 6l-6 6 6 6" />
    </svg>
  ),
  phone: (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="5" y="2" width="14" height="20" rx="2" /><path d="M12 18h.01" />
    </svg>
  ),
  shield: (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    </svg>
  ),
  target: (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="9" /><circle cx="12" cy="12" r="5" /><circle cx="12" cy="12" r="1" />
    </svg>
  ),
  live: (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 2v8" /><path d="M18.4 6.6a9 9 0 1 1-12.8 0" />
    </svg>
  ),
};

const PRIMARY_NAV: NavItem[] = [
  { href: "/terminal", label: "Command Center", icon: ic.monitor },
  { href: "/live-engine", label: "Live Engine", icon: ic.live },
  // Directly below the Live Engine, because it is the same desk against the
  // demo venue: same layout, same controls, same fee and bracket handling,
  // separate process and separate credentials so the two wallets can never be
  // confused for one another.
  { href: "/live-demo-engine", label: "Live Demo Engine", icon: ic.live },
  // Directly below the Live Engine because it answers the question that page
  // cannot: the Live Engine shows what real capital is doing, this shows
  // whether those same strategies deserve it. Same allow-list, same Delta
  // prices, same taker fees, same margin rules — paper money only.
  { href: "/live-engine-paper", label: "Live Engine Paper Desk", icon: ic.trend },
  // Closes the Live Engine group. It is the Crypto Scalp Desk's strategies on
  // the highest-volume currencies only — a separate engine process on a
  // fourteen-symbol universe, so those streams compete for fill slots with each
  // other rather than with ~220 thinly-traded perpetuals. Placed here rather
  // than beside the scalp desk because it is a candidate feeder for the live
  // desk, and the scalp desk stays where it is as the control arm.
  { href: "/high-volume-crypto", label: "High Volume Crypto Trading", icon: ic.bolt },
  // Sits directly below Live Engine: it is the paper counterpart to that desk —
  // same Delta chain, same margin reality, no real money.
  // Gold sits with the trading desks rather than under the crypto group: it
  // runs on the same venue and the same engine, but it is a different asset and
  // its results must not be read as part of the crypto leaderboard.
  { href: "/gold-desk", label: "Gold Desk", icon: ic.target },
  { href: "/crypto-fno", label: "Crypto F&O", icon: ic.layers },
  { href: "/btc-pre-live", label: "BTC Pre-Live Engine", icon: ic.trend },
  { href: "/scalp-desk", label: "Crypto Scalp Desk", icon: ic.bolt },
  { href: "/options-selling-desk", label: "Crypto Options Selling", icon: ic.shield },
  { href: "/options-buying-desk", label: "Crypto Options Buying", icon: ic.target },
  { href: "/mock-trading", label: "Mock Trading", icon: ic.layers },
];

const SYSTEM_NAV: NavItem[] = [{ href: "/mobile", label: "Mobile Emergency", icon: ic.phone }];

/** Raw JSON debug endpoints — grouped into collapsed disclosures so they
 * don't outweigh the six real nav destinations above. */
const API_GROUPS: NavSection[] = [
  {
    title: "Scalp Lab",
    items: [
      { href: "/api/scalp/scalp/leaderboard", label: "Leaderboard JSON", icon: ic.code, external: true },
      { href: "/api/scalp/scalp/stats", label: "Desk Totals JSON", icon: ic.code, external: true },
      { href: "/api/scalp/scalp/trades?n=100", label: "Trade History JSON", icon: ic.code, external: true },
      { href: "/api/scalp/scalp/health", label: "Engine Health JSON", icon: ic.code, external: true },
    ],
  },
  {
    // The same four endpoints on the high-volume process. Separate group rather
    // than extra rows in Scalp Lab: the two desks answer with the same JSON
    // shape from different universes, and a reader who mistakes one for the
    // other has no way to notice from the payload alone.
    title: "High Volume",
    items: [
      { href: "/api/scalp-highvol/scalp/leaderboard", label: "Leaderboard JSON", icon: ic.code, external: true },
      { href: "/api/scalp-highvol/scalp/stats", label: "Desk Totals JSON", icon: ic.code, external: true },
      { href: "/api/scalp-highvol/scalp/trades?n=100", label: "Trade History JSON", icon: ic.code, external: true },
      { href: "/api/scalp-highvol/scalp/health", label: "Engine Health JSON", icon: ic.code, external: true },
    ],
  },
  {
    title: "Options Selling",
    items: [
      { href: "/api/options-selling/strategies", label: "Strategies JSON", icon: ic.code, external: true },
      { href: "/api/options-selling/stats", label: "Desk Totals JSON", icon: ic.code, external: true },
      { href: "/api/options-selling/trades", label: "Trade History JSON", icon: ic.code, external: true },
      { href: "/api/options-selling/positions", label: "Open Positions JSON", icon: ic.code, external: true },
    ],
  },
  {
    title: "Options Buying",
    items: [
      { href: "/api/options-buying/strategies", label: "Strategies JSON", icon: ic.code, external: true },
      { href: "/api/options-buying/stats", label: "Desk Totals JSON", icon: ic.code, external: true },
      { href: "/api/options-buying/trades", label: "Trade History JSON", icon: ic.code, external: true },
      { href: "/api/options-buying/positions", label: "Open Positions JSON", icon: ic.code, external: true },
    ],
  },
];

function SectionLabel({ children }: { children: ReactNode }) {
  return (
    <div
      className="px-3 pb-2 pt-1 text-[10.5px] font-bold uppercase tracking-[0.16em]"
      style={{ color: "var(--desk-on-surface-variant)", opacity: 0.75 }}
    >
      {children}
    </div>
  );
}

function NavRow({ item, active }: { item: NavItem; active: boolean }) {
  const cls = `app-shell-nav-row flex items-center gap-3 rounded-xl px-3 py-2.5 text-[13.5px] font-semibold transition-colors ${
    active ? "" : "app-shell-nav-link"
  }`;
  const style: React.CSSProperties = active
    ? {
        background: "var(--desk-primary-container)",
        color: "var(--desk-on-primary-container)",
        boxShadow: "inset 3px 0 0 var(--desk-primary)",
      }
    : { color: "var(--desk-on-surface-variant)" };
  const iconWrapStyle: React.CSSProperties = {
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    width: 26,
    height: 26,
    borderRadius: 8,
    flexShrink: 0,
    color: active ? "var(--desk-primary)" : "var(--desk-on-surface-variant)",
    background: active ? "var(--desk-surface)" : "transparent",
    opacity: active ? 1 : 0.75,
  };
  const body = (
    <>
      <span style={iconWrapStyle}>{item.icon}</span>
      <span className="flex-1 truncate">{item.label}</span>
      {item.external && (
        <span aria-hidden style={{ fontSize: 10, fontWeight: 700, color: "var(--desk-on-surface-variant)", opacity: 0.6 }}>
          ↗
        </span>
      )}
    </>
  );
  return (
    <li>
      {item.external ? (
        <a href={item.href} target="_blank" rel="noreferrer" className={cls} style={style}>{body}</a>
      ) : (
        <Link href={item.href} className={cls} style={style}>{body}</Link>
      )}
    </li>
  );
}

function ApiGroup({ title, items }: NavSection) {
  return (
    <details className="app-shell-api-group">
      <summary
        className="flex cursor-pointer list-none items-center gap-2 rounded-lg px-3 py-2 text-[11.5px] font-bold"
        style={{ color: "var(--desk-on-surface-variant)" }}
      >
        <svg
          className="app-shell-api-group__chevron"
          width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3"
          strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"
          style={{ flexShrink: 0, transition: "transform 0.15s ease" }}
        >
          <path d="M9 6l6 6-6 6" />
        </svg>
        <span className="flex-1 truncate">{title}</span>
        <span
          className="desk-mono"
          style={{ fontSize: 10, fontWeight: 700, opacity: 0.6, padding: "1px 6px", borderRadius: 999, background: "var(--desk-surface-container)" }}
        >
          {items.length}
        </span>
      </summary>
      <ul className="flex flex-col gap-0.5 py-1 pl-2">
        {items.map((item) => (
          <li key={item.href}>
            <a
              href={item.href}
              target="_blank"
              rel="noreferrer"
              className="app-shell-nav-link flex items-center gap-2 rounded-lg px-3 py-1.5 text-[12.5px] font-medium"
              style={{ color: "var(--desk-on-surface-variant)" }}
            >
              <span style={{ opacity: 0.6, display: "flex" }}>{item.icon}</span>
              <span className="flex-1 truncate">{item.label}</span>
              <span aria-hidden style={{ fontSize: 10, fontWeight: 700, opacity: 0.5 }}>↗</span>
            </a>
          </li>
        ))}
      </ul>
    </details>
  );
}

function SidebarBody({ pathname }: { pathname: string }) {
  return (
    <>
      <Link
        href="/terminal"
        className="flex items-center gap-3 px-6 pb-5 pt-6"
        style={{ borderBottom: "1px solid var(--desk-outline-variant)" }}
      >
        <span
          className="flex h-10 w-10 items-center justify-center rounded-2xl shadow-sm"
          style={{ background: "var(--desk-primary)", color: "var(--desk-on-primary)" }}
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M22 7l-8.5 8.5-5-5L2 17" /><path d="M16 7h6v6" />
          </svg>
        </span>
        <span className="text-[19px] font-extrabold tracking-tight" style={{ ...sora, color: "var(--desk-on-surface)" }}>
          Antigravity
        </span>
      </Link>

      <nav className="flex-1 overflow-y-auto px-4 pb-4 pt-4">
        <SectionLabel>Trading</SectionLabel>
        <ul className="flex flex-col gap-0.5">
          {PRIMARY_NAV.map((item) => (
            <NavRow key={item.href} item={item} active={pathname.startsWith(item.href)} />
          ))}
        </ul>

        <div className="my-4" style={{ borderTop: "1px solid var(--desk-outline-variant)" }} />

        <SectionLabel>Developer · Raw JSON</SectionLabel>
        <div className="flex flex-col gap-0.5">
          {API_GROUPS.map((sec) => (
            <ApiGroup key={sec.title} title={sec.title} items={sec.items} />
          ))}
        </div>

        <div className="my-4" style={{ borderTop: "1px solid var(--desk-outline-variant)" }} />

        <SectionLabel>System</SectionLabel>
        <ul className="flex flex-col gap-0.5">
          {SYSTEM_NAV.map((item) => (
            <NavRow key={item.href} item={item} active={pathname.startsWith(item.href)} />
          ))}
        </ul>
      </nav>

      <div
        className="mx-4 mb-5 rounded-2xl px-4 py-3.5 text-[10.5px] leading-relaxed"
        style={{ background: "var(--desk-surface-container)", color: "var(--desk-on-surface-variant)" }}
      >
        <span className="font-bold" style={{ color: "var(--desk-on-surface)" }}>Paper only.</span> Real money requires
        the pre-registered gate — never leaderboard position alone.
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
    <div className="flex min-h-screen" style={{ background: "var(--desk-surface-dim)" }}>
      {/* Desktop: sticky sidebar as a flex item — cannot overlap content */}
      <aside
        className="app-shell-sidebar sticky top-0 hidden h-screen w-[250px] shrink-0 flex-col lg:flex"
        style={{ borderRight: "1px solid var(--desk-outline)", background: "var(--desk-surface)" }}
      >
        <SidebarBody pathname={pathname} />
      </aside>

      {/* Mobile: floating launcher + drawer */}
      <button
        type="button"
        aria-label="Open module menu"
        onClick={() => setOpen(true)}
        className="fixed left-3 top-3 z-40 flex h-10 w-10 items-center justify-center rounded-full shadow-md lg:hidden"
        style={{ border: "1px solid var(--desk-outline)", background: "var(--desk-surface)", color: "var(--desk-on-surface-variant)" }}
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
            className="absolute inset-0 h-full w-full cursor-default"
            style={{ background: "rgba(0,0,0,0.4)" }}
          />
          <aside
            className="app-shell-sidebar absolute left-0 top-0 flex h-full w-[280px] flex-col overflow-y-auto shadow-2xl"
            style={{ borderRight: "1px solid var(--desk-outline)", background: "var(--desk-surface)" }}
          >
            <SidebarBody pathname={pathname} />
          </aside>
        </div>
      )}

      {/* Content column */}
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}
