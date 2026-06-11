"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { useState } from "react";
import RiskRibbon from "@/components/RiskRibbon";
import { CommandPaletteProvider, CommandPaletteTrigger } from "@/components/ui/CommandPalette";
import { StatusChip } from "@/components/ui/StatusChip";
import { useThemeToggle } from "@/components/ui/ThemeProvider";
import { resolvePageTitle } from "@/lib/commandPaletteItems";
import { COMMAND_CENTER_NAV, isNavItemActive, TERMINAL_ROUTES } from "@/lib/navRoutes";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { terminalAuthorityLabel } from "@/lib/terminal/terminalAuthority";
import { pct, px } from "./format";
import { NavIcon } from "./NavIcons";

export function InstitutionalTerminalShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const snapshot = useTerminalSnapshot();
  const [railCollapsed, setRailCollapsed] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const { theme, toggle: toggleTheme } = useThemeToggle();

  const criticalAlerts = snapshot.alerts.filter((a) => a.severity === "CRITICAL").length;
  const authorityLabel = terminalAuthorityLabel(snapshot);
  const pageTitle = resolvePageTitle(pathname);

  const tradingNav = COMMAND_CENTER_NAV.filter((i) => i.label === "Mock Trading");
  const monitorNav = COMMAND_CENTER_NAV.filter((i) => i.label !== "Mock Trading");

  return (
    <CommandPaletteProvider>
    <div className="m3-app-shell">

      <aside
        className={`m3-nav-rail ${railCollapsed ? "m3-nav-rail--collapsed" : ""} ${mobileNavOpen ? "m3-nav-rail--mobile-open" : ""}`}
        aria-label="Main navigation"
      >
        <div className="m3-nav-rail__brand">
          <Link href={TERMINAL_ROUTES.home} className="m3-nav-rail__brand-link">
            <span className="m3-nav-rail__logo" aria-hidden>ICC</span>
            <span className="m3-nav-rail__brand-text">Institutional Command Center</span>
          </Link>
        </div>

        <div className="m3-nav-rail__scroll">
          <div className="m3-nav-section-label">Trading</div>
          {tradingNav.map((item) => (
            <NavLink key={item.href} item={item} pathname={pathname} onNavigate={() => setMobileNavOpen(false)} />
          ))}

          <div className="m3-nav-section-label">Monitor</div>
          {monitorNav.map((item) => (
            <NavLink key={item.href} item={item} pathname={pathname} onNavigate={() => setMobileNavOpen(false)} />
          ))}
        </div>

        <div className="m3-nav-rail__footer">
          <button
            type="button"
            className="m3-nav-item m3-nav-rail-toggle"
            onClick={() => setRailCollapsed((v) => !v)}
            aria-label={railCollapsed ? "Expand navigation" : "Collapse navigation"}
          >
            <NavIcon name="collapse" />
            <span className="m3-nav-item__label">{railCollapsed ? "Expand" : "Collapse"}</span>
          </button>
        </div>
      </aside>

      {mobileNavOpen ? (
        <button
          type="button"
          className="m3-mobile-backdrop"
          aria-label="Close navigation"
          onClick={() => setMobileNavOpen(false)}
        />
      ) : null}

      <div className={`m3-main-column ${railCollapsed ? "m3-main-column--collapsed" : ""}`}>
        <header className="m3-top-app-bar" role="banner">
          <button
            type="button"
            className="m3-icon-btn m3-mobile-menu-btn"
            aria-label="Open navigation menu"
            onClick={() => setMobileNavOpen(true)}
          >
            <NavIcon name="menu" />
          </button>

          <div>
            <h1 className="m3-top-app-bar__title">{pageTitle}</h1>
            <div className="m3-top-app-bar__metrics">
              <span className="m3-top-app-bar__price">
                {snapshot.hasAuthority && snapshot.price > 0 ? `$${px(snapshot.price)}` : "—"}
              </span>
              <span className={snapshot.priceChange24hPct >= 0 ? "m3-text-profit" : "m3-text-loss"}>
                {snapshot.hasAuthority && snapshot.price > 0 ? pct(snapshot.priceChange24hPct) : "—"}
              </span>
            </div>
          </div>

          <div className="m3-top-app-bar__search">
            <CommandPaletteTrigger />
          </div>

          <div className="m3-top-app-bar__actions">
            <StatusChip
              label={authorityLabel}
              tone={
                snapshot.hasAuthority
                  ? snapshot.authoritySource === "ws"
                    ? "success"
                    : "info"
                  : "error"
              }
            />
            {criticalAlerts > 0 ? (
              <StatusChip label={`${criticalAlerts} critical`} tone="error" />
            ) : null}
            <button type="button" className="m3-icon-btn" onClick={toggleTheme} aria-label="Toggle theme">
              <NavIcon name={theme === "dark" ? "light" : "dark"} />
            </button>
          </div>
        </header>

        <div className="m3-risk-ribbon-wrap">
          <RiskRibbon />
        </div>

        <main className="m3-content-area" id="main-content">
          {children}
        </main>
      </div>
    </div>
    </CommandPaletteProvider>
  );
}

function NavLink({
  item,
  pathname,
  onNavigate,
}: {
  item: (typeof COMMAND_CENTER_NAV)[number];
  pathname: string;
  onNavigate: () => void;
}) {
  const active = isNavItemActive(pathname, {
    href: item.href,
    exactMatch: "exactMatch" in item ? item.exactMatch : false,
  });

  return (
    <Link
      href={item.href}
      className={`m3-nav-item ${active ? "m3-nav-item--active" : ""}`}
      aria-current={active ? "page" : undefined}
      onClick={onNavigate}
    >
      <span className="m3-nav-item__icon">
        <NavIcon name={item.label} />
      </span>
      <span className="m3-nav-item__label">{item.label}</span>
    </Link>
  );
}
