"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { useState } from "react";
import RiskRibbon from "@/components/RiskRibbon";
import { CommandPaletteProvider, CommandPaletteTrigger } from "@/components/ui/CommandPalette";
import { StatusChip } from "@/components/ui/StatusChip";
import { useThemeToggle } from "@/components/ui/ThemeProvider";
import { resolvePageTitle } from "@/lib/utils/commandPaletteItems";
import { formatNavCount, usePipelineCounts } from "@/hooks/usePipelineCounts";
import { MONITOR_NAV, TRADING_NAV, isNavItemActive, TERMINAL_ROUTES, type CommandCenterNavItem } from "@/lib/utils/navRoutes";
import { NavIcon } from "@/components/terminal/institutional/NavIcons";
import { pct, px } from "@/components/terminal/institutional/format";
import { SessionClock } from "@/components/session/SessionClock";
import { KillSwitchIndicator } from "@/components/killswitch/KillSwitchIndicator";
import { KillSwitchPanel } from "@/components/killswitch/KillSwitchPanel";
import { RiskHUD } from "@/components/risk/RiskHUD";

export type M3AppShellProps = {
  children: ReactNode;
  pageTitle?: string;
  price?: number;
  priceChange24hPct?: number;
  showRiskRibbon?: boolean;
  statusChips?: ReactNode;
  breadcrumb?: ReactNode;
  pageActions?: ReactNode;
};

export function M3AppShell({
  children,
  pageTitle: pageTitleProp,
  price,
  priceChange24hPct = 0,
  showRiskRibbon = true,
  statusChips,
  breadcrumb,
  pageActions,
}: M3AppShellProps) {
  const pathname = usePathname();
  const [railCollapsed, setRailCollapsed] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const { theme, toggle: toggleTheme } = useThemeToggle();
  const pageTitle = pageTitleProp ?? resolvePageTitle(pathname);
  const pipelineCounts = usePipelineCounts();

  return (
    <CommandPaletteProvider>
      <div className="m3-app-shell">
        <KillSwitchPanel />
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
            {TRADING_NAV.map((item) => (
              <ShellNavLink
                key={item.href}
                item={item}
                pathname={pathname}
                count={formatNavCount(pipelineCounts, item.countStatus)}
                onNavigate={() => setMobileNavOpen(false)}
              />
            ))}
            <div className="m3-nav-section-label">Monitor</div>
            {MONITOR_NAV.map((item) => (
              <ShellNavLink key={item.href} item={item} pathname={pathname} onNavigate={() => setMobileNavOpen(false)} />
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
          <button type="button" className="m3-mobile-backdrop" aria-label="Close navigation" onClick={() => setMobileNavOpen(false)} />
        ) : null}

        <div className={`m3-main-column ${railCollapsed ? "m3-main-column--collapsed" : ""}`}>
          <header className="m3-top-app-bar" role="banner">
            <button type="button" className="m3-icon-btn m3-mobile-menu-btn" aria-label="Open navigation menu" onClick={() => setMobileNavOpen(true)}>
              <NavIcon name="menu" />
            </button>
            <div>
              {breadcrumb ? <div className="m3-breadcrumb">{breadcrumb}</div> : null}
              <h1 className="m3-top-app-bar__title">{pageTitle}</h1>
              {price != null && price > 0 ? (
                <div className="m3-top-app-bar__metrics">
                  <span className="m3-top-app-bar__price">${px(price)}</span>
                  <span className={priceChange24hPct >= 0 ? "m3-text-profit" : "m3-text-loss"}>{pct(priceChange24hPct)}</span>
                </div>
              ) : null}
            </div>
            <div className="m3-top-app-bar__search">
              <CommandPaletteTrigger />
            </div>
            <div className="m3-top-app-bar__actions">
              <SessionClock />
              <RiskHUD />
              {statusChips}
              <KillSwitchIndicator />
              <button type="button" className="m3-icon-btn" onClick={toggleTheme} aria-label="Toggle theme">
                <NavIcon name={theme === "dark" ? "light" : "dark"} />
              </button>
              {pageActions}
            </div>
          </header>

          {showRiskRibbon ? (
            <div className="m3-risk-ribbon-wrap">
              <RiskRibbon />
            </div>
          ) : null}

          <main className="m3-content-area" id="main-content">
            {children}
          </main>
        </div>
      </div>
    </CommandPaletteProvider>
  );
}

function ShellNavLink({
  item,
  pathname,
  count,
  onNavigate,
}: {
  item: CommandCenterNavItem;
  pathname: string;
  count?: string;
  onNavigate: () => void;
}) {
  const active = isNavItemActive(pathname, {
    href: item.href,
    exactMatch: item.exactMatch ?? false,
  });
  const labelWithCount = count != null ? `${item.label} (${count})` : item.label;
  return (
    <Link
      href={item.href}
      className={`m3-nav-item ${active ? "m3-nav-item--active" : ""}`}
      aria-current={active ? "page" : undefined}
      onClick={onNavigate}
      title={labelWithCount}
    >
      <span className="m3-nav-item__icon"><NavIcon name={item.label} /></span>
      <span className="m3-nav-item__label">
        {item.label}
        {count != null ? <span className="m3-nav-item__count"> ({count})</span> : null}
      </span>
    </Link>
  );
}
