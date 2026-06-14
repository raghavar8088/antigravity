"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { MONITOR_NAV, TRADING_NAV, isNavItemActive, TERMINAL_ROUTES } from "@/lib/utils/navRoutes";

type NavItem = {
  href: string;
  label: string;
  icon: React.ReactNode;
  badge?: string | number;
  countStatus?: import("@/lib/strategyAuthority/types").StrategyStatus;
  exactMatch?: boolean;
  routeMatcher?: (pathname: string) => boolean;
};

type NavSection = {
  label?: string;
  items: NavItem[];
};

function IconCommand() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1" y="1" width="6" height="6" rx="1.5" />
      <rect x="9" y="1" width="6" height="6" rx="1.5" />
      <rect x="1" y="9" width="6" height="6" rx="1.5" />
      <rect x="9" y="9" width="6" height="6" rx="1.5" />
    </svg>
  );
}

function IconExecution() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
      <path d="M4.5 6.5h7M4.5 9.5h4.5" />
      <path d="M11.5 9.5l1.5 1.5 2.5-3.5" />
    </svg>
  );
}

function IconStrategy() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="8" cy="8" r="6.5" />
      <line x1="8" y1="4" x2="8" y2="8" />
      <line x1="8" y1="8" x2="11" y2="10" />
      <circle cx="8" cy="8" r="1" fill="currentColor" stroke="none" />
    </svg>
  );
}

function IconAnalytics() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1" y="9" width="3" height="6" rx="0.5" />
      <rect x="6" y="5" width="3" height="10" rx="0.5" />
      <rect x="11" y="1" width="4" height="14" rx="0.5" />
    </svg>
  );
}

function IconRisk() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 1.5L14.5 13H1.5L8 1.5Z" />
      <line x1="8" y1="6" x2="8" y2="9.5" />
      <circle cx="8" cy="11.5" r="0.75" fill="currentColor" stroke="none" />
    </svg>
  );
}

function IconPortfolio() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1.5" y="4.5" width="13" height="9.5" rx="1.5" />
      <path d="M5 4.5V3a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v1.5" />
      <line x1="1.5" y1="8.5" x2="14.5" y2="8.5" />
    </svg>
  );
}

function IconResearch() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="6.5" cy="6.5" r="5" />
      <line x1="10.5" y1="10.5" x2="14.5" y2="14.5" />
      <line x1="4.5" y1="6.5" x2="8.5" y2="6.5" />
      <line x1="6.5" y1="4.5" x2="6.5" y2="8.5" />
    </svg>
  );
}

function IconEvents() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 4h12v9H2z" />
      <path d="M5 2v2M11 2v2M2 7h12" />
    </svg>
  );
}

function IconHealth() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 13s-5-3.5-5-7a3 3 0 0 1 5-2 3 3 0 0 1 5 2c0 3.5-5 7-5 7z" />
    </svg>
  );
}

function IconDiagnostics() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="1,8 4,8 5.5,4 7,12 9,6 10.5,10 12,8 15,8" />
    </svg>
  );
}

function IconSettings() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="8" cy="8" r="2.5" />
      <path d="M8 1v1.5M8 13.5V15M1 8h1.5M13.5 8H15M3.05 3.05l1.06 1.06M11.89 11.89l1.06 1.06M3.05 12.95l1.06-1.06M11.89 4.11l1.06-1.06" />
    </svg>
  );
}

const ICONS: Record<string, React.ReactNode> = {
  "Command Center": <IconCommand />,
  Execution: <IconExecution />,
  Strategies: <IconStrategy />,
  Portfolio: <IconPortfolio />,
  Risk: <IconRisk />,
  Analytics: <IconAnalytics />,
  Research: <IconResearch />,
  Events: <IconEvents />,
  Health: <IconHealth />,
  Diagnostics: <IconDiagnostics />,
  Settings: <IconSettings />,
  "Trade Engine": <IconResearch />,
};

const NAV_SECTIONS: NavSection[] = [
  {
    label: "Trading",
    items: TRADING_NAV.map((item) => ({
      href: item.href,
      label: item.label,
      icon: ICONS[item.label] ?? <IconResearch />,
      exactMatch: item.exactMatch ?? false,
      countStatus: item.countStatus,
    })),
  },
  {
    label: "Monitor",
    items: MONITOR_NAV.map((item) => ({
      href: item.href,
      label: item.label,
      icon: ICONS[item.label] ?? <IconCommand />,
      exactMatch: item.exactMatch ?? false,
    })),
  },
];

type SidebarProps = {
  liveConnected?: boolean;
  openPositions?: number;
  mobileOpen?: boolean;
  onNavigate?: () => void;
};

export function Sidebar({
  liveConnected = false,
  openPositions,
  mobileOpen = false,
  onNavigate,
}: SidebarProps) {
  const pathname = usePathname();

  return (
    <nav
      className={`terminal-sidebar${mobileOpen ? " terminal-sidebar--mobile-open" : ""}`}
      aria-label="Main navigation"
    >
      <div className="sidebar-brand">
        <div className="sidebar-brand-icon" aria-hidden="true">BTC</div>
        <div className="sidebar-brand-text">
          <span className="sidebar-brand-name">BTC-PILOT</span>
          <span className="sidebar-brand-sub">Sovereign</span>
        </div>
      </div>

      <div style={{ flex: 1, overflowY: "auto", padding: "8px 0" }}>
        {NAV_SECTIONS.map((section, si) => (
          <div key={si} className="sidebar-section">
            {section.label && (
              <div className="sidebar-section-label">{section.label}</div>
            )}
            {section.items.map((item) => {
              const active = isNavItemActive(pathname, item);
              return (
                <Link
                  key={`${item.href}-${item.label}`}
                  href={item.href}
                  className={`sidebar-nav-item${active ? " active" : ""}`}
                  aria-current={active ? "page" : undefined}
                  onClick={onNavigate}
                >
                  {item.icon}
                  <span>{item.label}</span>
                  {item.badge != null && (
                    <span className="sidebar-nav-badge">{item.badge}</span>
                  )}
                  {item.label === "Execution" && openPositions != null && openPositions > 0 && (
                    <span className="sidebar-nav-badge">{openPositions}</span>
                  )}
                </Link>
              );
            })}
          </div>
        ))}
      </div>

      <div className="sidebar-footer">
        <div
          className="sidebar-live-status"
          style={!liveConnected ? {
            background: "var(--red-dim)",
            borderColor: "rgba(239,83,80,0.2)",
          } : {}}
        >
          <div
            className="sidebar-live-dot"
            style={!liveConnected ? {
              background: "var(--red)",
              animationName: "pulse-red",
            } : {}}
          />
          <span
            className="sidebar-live-text"
            style={!liveConnected ? { color: "var(--red)" } : {}}
          >
            {liveConnected ? "Feed Live" : "Offline"}
          </span>
        </div>
      </div>
    </nav>
  );
}

export { TERMINAL_ROUTES };
