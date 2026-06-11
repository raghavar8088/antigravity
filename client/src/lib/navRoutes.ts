/** Institutional Command Center — monitoring surface (read-only). */
export const COMMAND_CENTER_PATH = "/terminal";

/** Primary execution surface — mock trading is the sole trade authority. */
export const MOCK_TRADING_PATH = "/mock-trading";

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

/** @deprecated Paper desk removed — redirects to mock trading. */
export const PAPER_DESK_PATH = "/paper-desk";

const PAPER_DESK_TAB_KEYS = ["positions", "trades", "orders", "equity", "strategies"] as const;
export type PaperDeskTabKey = (typeof PAPER_DESK_TAB_KEYS)[number];

const LEGACY_TAB_REDIRECTS: Record<PaperDeskTabKey, string> = {
  positions: MOCK_TRADING_PATH,
  trades: MOCK_TRADING_PATH,
  orders: MOCK_TRADING_PATH,
  equity: MOCK_TRADING_PATH,
  strategies: MOCK_TRADING_PATH,
};

export function isPaperDeskTabKey(value: string | null | undefined): value is PaperDeskTabKey {
  return PAPER_DESK_TAB_KEYS.includes(value as PaperDeskTabKey);
}

/** True for retired paper desk paths (redirect targets). */
export function isPaperDeskRoute(pathname: string): boolean {
  return (
    pathname === PAPER_DESK_PATH ||
    pathname.startsWith(`${PAPER_DESK_PATH}/`) ||
    pathname === "/paperdesk" ||
    pathname.startsWith("/paperdesk/") ||
    pathname === "/btc-future-trading" ||
    pathname.startsWith("/btc-future-trading/")
  );
}

export function isTerminalRoute(pathname: string): boolean {
  return pathname === COMMAND_CENTER_PATH || pathname.startsWith(`${COMMAND_CENTER_PATH}/`);
}

export function isMockTradingRoute(pathname: string): boolean {
  return pathname === MOCK_TRADING_PATH || pathname.startsWith(`${MOCK_TRADING_PATH}/`);
}

/** Resolve legacy Paper Desk deep links to mock trading. */
export function legacyPaperDeskRedirect(tab?: string | null): string {
  if (tab && isPaperDeskTabKey(tab)) return LEGACY_TAB_REDIRECTS[tab];
  return MOCK_TRADING_PATH;
}

/** @deprecated Use MOCK_TRADING_PATH or TERMINAL_ROUTES. */
export function paperDeskHref(tab?: PaperDeskTabKey): string {
  return legacyPaperDeskRedirect(tab);
}

type NavActiveItem = {
  href: string;
  exactMatch?: boolean;
  routeMatcher?: (pathname: string) => boolean;
};

export function isNavItemActive(pathname: string, item: NavActiveItem): boolean {
  if (item.routeMatcher) return item.routeMatcher(pathname);
  if (item.exactMatch) return pathname === item.href;
  return pathname === item.href || pathname.startsWith(`${item.href}/`);
}

export const COMMAND_CENTER_NAV = [
  { href: MOCK_TRADING_PATH, label: "Mock Trading", exactMatch: true },
  { href: TERMINAL_ROUTES.home, label: "Command Center" },
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
