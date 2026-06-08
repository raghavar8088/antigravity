"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { isNavItemActive, isPaperDeskRoute, paperDeskHref } from "@/lib/navRoutes";

type NavItem = {
  href: string;
  label: string;
  icon: React.ReactNode;
  badge?: string | number;
  exactMatch?: boolean;
  routeMatcher?: (pathname: string) => boolean;
};

type NavSection = {
  label?: string;
  items: NavItem[];
};

function IconDashboard() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1" y="1" width="6" height="6" rx="1.5" />
      <rect x="9" y="1" width="6" height="6" rx="1.5" />
      <rect x="1" y="9" width="6" height="6" rx="1.5" />
      <rect x="9" y="9" width="6" height="6" rx="1.5" />
    </svg>
  );
}

function IconTrade() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="1,11 5,7 8,10 15,3" />
      <polyline points="11,3 15,3 15,7" />
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

function IconBTC() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <text x="3" y="13" fontSize="13" fontWeight="800" fill="currentColor" stroke="none" fontFamily="monospace">₿</text>
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

function IconHistory() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M1.5 8A6.5 6.5 0 0 1 9.5 1.7" />
      <polyline points="1.5,4 1.5,8 5.5,8" />
      <circle cx="9" cy="9" r="5.5" />
      <line x1="9" y1="6.5" x2="9" y2="9.5" />
      <line x1="9" y1="9.5" x2="11" y2="11.5" />
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

function IconPortfolio() {
  return (
    <svg className="sidebar-nav-item-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1.5" y="4.5" width="13" height="9.5" rx="1.5" />
      <path d="M5 4.5V3a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v1.5" />
      <line x1="1.5" y1="8.5" x2="14.5" y2="8.5" />
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

const NAV_SECTIONS: NavSection[] = [
  {
    items: [
      {
        href: "/",
        label: "Dashboard",
        icon: <IconDashboard />,
        exactMatch: true,
      },
      {
        href: "/mock-trading",
        label: "Mock Trading",
        icon: <IconTrade />,
      },
      {
        href: paperDeskHref(),
        label: "Paper Desk",
        icon: <IconExecution />,
        routeMatcher: isPaperDeskRoute,
      },
      {
        href: "/terminal",
        label: "Terminal V2",
        icon: <IconBTC />,
      },
    ],
  },
  {
    label: "Quantitative Lab",
    items: [
      {
        href: "/mock-trading",
        label: "Research",
        icon: <IconResearch />,
      },
      {
        href: "/mock-trading",
        label: "Strategies",
        icon: <IconStrategy />,
      },
      {
        href: "/mock-trading",
        label: "Portfolio",
        icon: <IconPortfolio />,
      },
      {
        href: "/mock-trading",
        label: "Risk",
        icon: <IconRisk />,
      },
      {
        href: "/mock-trading",
        label: "Analytics",
        icon: <IconAnalytics />,
      },
    ],
  },
  {
    label: "Execution & Logs",
    items: [
      {
        href: paperDeskHref("trades"),
        label: "Trade History",
        icon: <IconHistory />,
      },
      {
        href: "/mock-trading",
        label: "Diagnostics",
        icon: <IconDiagnostics />,
      },
    ],
  },
  {
    label: "System",
    items: [
      {
        href: "/mock-trading",
        label: "Settings",
        icon: <IconSettings />,
      },
    ],
  },
];

type SidebarProps = {
  liveConnected?: boolean;
  openPositions?: number;
  paperDeskOpenPositions?: number;
  mobileOpen?: boolean;
  onNavigate?: () => void;
};

export function Sidebar({
  liveConnected = false,
  openPositions,
  paperDeskOpenPositions,
  mobileOpen = false,
  onNavigate,
}: SidebarProps) {
  const pathname = usePathname();

  return (
    <nav
      className={`terminal-sidebar${mobileOpen ? " terminal-sidebar--mobile-open" : ""}`}
      aria-label="Main navigation"
    >
      {/* Brand */}
      <div className="sidebar-brand">
        <div className="sidebar-brand-icon" aria-hidden="true">₿</div>
        <div className="sidebar-brand-text">
          <span className="sidebar-brand-name">BTC Terminal</span>
          <span className="sidebar-brand-sub">Research Platform</span>
        </div>
      </div>

      {/* Navigation sections */}
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
                  {item.label === "Mock Trading" && openPositions != null && openPositions > 0 && (
                    <span className="sidebar-nav-badge">{openPositions}</span>
                  )}
                  {item.label === "Paper Desk" && paperDeskOpenPositions != null && paperDeskOpenPositions > 0 && (
                    <span className="sidebar-nav-badge">{paperDeskOpenPositions}</span>
                  )}
                </Link>
              );
            })}
          </div>
        ))}
      </div>

      {/* Footer */}
      <div className="sidebar-footer">
        <div className={`sidebar-live-status${liveConnected ? "" : ""}`}
          style={!liveConnected ? {
            background: "var(--red-dim)",
            borderColor: "rgba(239,83,80,0.2)",
          } : {}}>
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
