/** Institutional Command Center — canonical operator surface. */
export const COMMAND_CENTER_PATH = "/terminal";

export const TERMINAL_ROUTES = {
  home: "/terminal",
  execution: "/terminal/execution",
  strategies: "/terminal/strategies",
  portfolio: "/terminal/portfolio",
  risk: "/terminal/risk",
  analytics: "/terminal/analytics",
  research: "/terminal/research",
  events: "/terminal/events",
  health: "/terminal/health",
  diagnostics: "/terminal/diagnostics",
  observability: "/terminal/observability",
  journal: "/terminal/journal",
  settings: "/terminal/settings",
} as const;

export type TerminalRouteKey = keyof typeof TERMINAL_ROUTES;

/** @deprecated Legacy path — UI redirects to Command Center. APIs remain at /api/paper-desk/*. */
export const PAPER_DESK_PATH = "/paper-desk";

const PAPER_DESK_TAB_KEYS = ["positions", "trades", "orders", "equity", "strategies"] as const;
export type PaperDeskTabKey = (typeof PAPER_DESK_TAB_KEYS)[number];

const LEGACY_TAB_REDIRECTS: Record<PaperDeskTabKey, string> = {
  positions: TERMINAL_ROUTES.execution,
  trades: TERMINAL_ROUTES.journal,
  orders: TERMINAL_ROUTES.events,
  equity: TERMINAL_ROUTES.analytics,
  strategies: TERMINAL_ROUTES.strategies,
};

export function isPaperDeskTabKey(value: string | null | undefined): value is PaperDeskTabKey {
  return PAPER_DESK_TAB_KEYS.includes(value as PaperDeskTabKey);
}

/** True for /paper-desk, /paper-desk/*, /paperdesk, /paperdesk/* */
export function isPaperDeskRoute(pathname: string): boolean {
  return (
    pathname === PAPER_DESK_PATH ||
    pathname.startsWith(`${PAPER_DESK_PATH}/`) ||
    pathname === "/paperdesk" ||
    pathname.startsWith("/paperdesk/")
  );
}

export function isTerminalRoute(pathname: string): boolean {
  return pathname === COMMAND_CENTER_PATH || pathname.startsWith(`${COMMAND_CENTER_PATH}/`);
}

/** Resolve legacy Paper Desk deep links to Command Center routes. */
export function legacyPaperDeskRedirect(tab?: string | null): string {
  if (tab && isPaperDeskTabKey(tab)) return LEGACY_TAB_REDIRECTS[tab];
  return TERMINAL_ROUTES.home;
}

/** @deprecated Use TERMINAL_ROUTES — retained for API-layer references only. */
export function paperDeskHref(tab?: PaperDeskTabKey): string {
  return legacyPaperDeskRedirect(tab);
}

type NavActiveItem = {
  href: string;
  exactMatch?: boolean;
  routeMatcher?: (pathname: string) => boolean;
};

/** Sidebar active-state resolver shared by desktop and mobile nav. */
export function isNavItemActive(pathname: string, item: NavActiveItem): boolean {
  if (item.routeMatcher) return item.routeMatcher(pathname);
  if (item.exactMatch) return pathname === item.href;
  return pathname === item.href || pathname.startsWith(`${item.href}/`);
}

export const COMMAND_CENTER_NAV = [
  { href: TERMINAL_ROUTES.home, label: "Command Center", exactMatch: true },
  { href: TERMINAL_ROUTES.execution, label: "Execution" },
  { href: TERMINAL_ROUTES.strategies, label: "Strategies" },
  { href: TERMINAL_ROUTES.portfolio, label: "Portfolio" },
  { href: TERMINAL_ROUTES.risk, label: "Risk" },
  { href: TERMINAL_ROUTES.analytics, label: "Analytics" },
  { href: TERMINAL_ROUTES.research, label: "Research" },
  { href: TERMINAL_ROUTES.events, label: "Events" },
  { href: TERMINAL_ROUTES.health, label: "Health" },
  { href: TERMINAL_ROUTES.diagnostics, label: "Diagnostics" },
  { href: TERMINAL_ROUTES.settings, label: "Settings" },
] as const;
