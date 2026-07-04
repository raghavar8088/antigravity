type NavIconName =
  | "Trade Engine"
  | "Pre-Live Engine"
  | "Pre-Live Strategies"
  | "Backtest Lab"
  | "Trade Thresholds"
  | "Command Center"
  | "Execution"
  | "Strategy Authority"
  | "Portfolio Intelligence"
  | "Strategies"
  | "Portfolio"
  | "Risk"
  | "Analytics"
  | "Research"
  | "Events"
  | "Health"
  | "Diagnostics"
  | "Settings"
  | "menu"
  | "collapse"
  | "light"
  | "dark"
  | "reset"
  | "logout";

export function NavIcon({ name }: { name: NavIconName | string }) {
  const props = {
    className: "m3-nav-svg",
    viewBox: "0 0 20 20",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.5,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
  };

  switch (name) {
    case "menu":
      return (
        <svg {...props}>
          <path d="M3 5h14M3 10h14M3 15h14" />
        </svg>
      );
    case "collapse":
      return (
        <svg {...props}>
          <path d="M7 4L3 10l4 6M13 4l4 6-4 6" />
        </svg>
      );
    case "light":
      return (
        <svg {...props}>
          <circle cx="10" cy="10" r="3.5" />
          <path d="M10 2v2M10 16v2M2 10h2M16 10h2M4.2 4.2l1.4 1.4M14.4 14.4l1.4 1.4M4.2 15.8l1.4-1.4M14.4 5.6l1.4-1.4" />
        </svg>
      );
    case "dark":
      return (
        <svg {...props}>
          <path d="M15.5 11.5a6 6 0 0 1-8-8 7 7 0 1 0 8 8z" />
        </svg>
      );
    case "reset":
      return (
        <svg {...props}>
          <path d="M15.5 6.5A6 6 0 1 0 16 10" />
          <path d="M16 3v4h-4" />
        </svg>
      );
    case "logout":
      return (
        <svg {...props}>
          <path d="M8 17H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3M13 14l4-4-4-4M17 10H7" />
        </svg>
      );
    case "Strategy Authority":
      return (
        <svg {...props}>
          <path d="M10 2l2 4h4l-3 3 1 4-4-2-4 2 1-4L4 6h4z" />
          <path d="M10 14v4" strokeWidth={1.5} />
        </svg>
      );
    case "Execution":
      return (
        <svg {...props}>
          <rect x="2" y="3" width="16" height="14" rx="2" />
          <path d="M6 8h8M6 11h5" />
        </svg>
      );
    case "Strategies":
    case "Research":
      return (
        <svg {...props}>
          <circle cx="10" cy="10" r="7" />
          <path d="M10 5v5l3 2" />
        </svg>
      );
    case "Portfolio Intelligence":
      return (
        <svg {...props}>
          <circle cx="10" cy="10" r="7" />
          <path d="M10 5v5l2.5-2.5" />
          <path d="M5 13.5h10" />
          <path d="M7 11.5l3-3 3 3" />
        </svg>
      );
    case "Portfolio":
      return (
        <svg {...props}>
          <rect x="2" y="5" width="16" height="12" rx="2" />
          <path d="M7 5V3h6v2" />
        </svg>
      );
    case "Risk":
      return (
        <svg {...props}>
          <path d="M10 3l8 14H2L10 3z" />
          <path d="M10 8v4" />
        </svg>
      );
    case "Analytics":
      return (
        <svg {...props}>
          <rect x="2" y="11" width="3" height="7" rx="0.5" />
          <rect x="8" y="6" width="3" height="12" rx="0.5" />
          <rect x="14" y="2" width="4" height="16" rx="0.5" />
        </svg>
      );
    case "Events":
      return (
        <svg {...props}>
          <path d="M4 4h12v12H4z" />
          <path d="M7 8h6M7 11h4" />
        </svg>
      );
    case "Health":
      return (
        <svg {...props}>
          <path d="M10 17s-6-4.5-6-9a4 4 0 0 1 8 0c0 4.5-6 9-6 9z" />
        </svg>
      );
    case "Diagnostics":
    case "Settings":
      return (
        <svg {...props}>
          <circle cx="10" cy="10" r="2.5" />
          <path d="M10 2v2M10 16v2M2 10h2M16 10h2M4.6 4.6l1.4 1.4M14 14l1.4 1.4M4.6 15.4l1.4-1.4M14 6l1.4-1.4" />
        </svg>
      );
    case "Trade Engine":
      return (
        <svg {...props}>
          <rect x="3" y="4" width="14" height="12" rx="2" />
          <path d="M7 9h6M7 12h4" />
          <circle cx="14" cy="7" r="2" fill="currentColor" stroke="none" />
        </svg>
      );
    case "Pre-Live Engine":
      return (
        <svg {...props}>
          <circle cx="10" cy="10" r="3" />
          <path d="M10 2v2M10 16v2M2 10h2M16 10h2M4.9 4.9l1.4 1.4M13.7 13.7l1.4 1.4M4.9 15.1l1.4-1.4M13.7 6.3l1.4-1.4" />
        </svg>
      );
    case "Pre-Live Strategies":
      return (
        <svg {...props}>
          <path d="M10 2l7 3.5v5c0 4-3 7.5-7 8.5-4-1-7-4.5-7-8.5v-5L10 2z" />
          <path d="M7.5 10l2 2 3.5-4" />
        </svg>
      );
    case "Backtest Lab":
      return (
        <svg {...props}>
          <path d="M3 3v14h14" />
          <path d="M6 13l3-4 3 2 4-6" />
        </svg>
      );
    case "Trade Thresholds":
      return (
        <svg {...props}>
          <path d="M4 6h12M4 10h12M4 14h12" />
          <circle cx="7" cy="6" r="1.4" fill="currentColor" stroke="none" />
          <circle cx="13" cy="10" r="1.4" fill="currentColor" stroke="none" />
          <circle cx="9" cy="14" r="1.4" fill="currentColor" stroke="none" />
        </svg>
      );
    default:
      return (
        <svg {...props}>
          <rect x="2" y="2" width="7" height="7" rx="1.5" />
          <rect x="11" y="2" width="7" height="7" rx="1.5" />
          <rect x="2" y="11" width="7" height="7" rx="1.5" />
          <rect x="11" y="11" width="7" height="7" rx="1.5" />
        </svg>
      );
  }
}
