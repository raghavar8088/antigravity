import type { StrategyStatus } from "@/lib/strategyAuthority/types";

/** Institutional Command Center — monitoring surface (read-only). */
export const COMMAND_CENTER_PATH = "/terminal";

/** Primary execution surface — mock trading is the sole trade authority. */
export const MOCK_TRADING_PATH = "/mock-trading";

export const TERMINAL_ROUTES = {
  home: "/terminal",
  "pre-live-engine": "/terminal/pre-live-engine",
  "pre-live-strategies": "/terminal/pre-live-strategies",
  "trade-engine": "/terminal/trade-engine",
  /** @deprecated Old route redirects to /terminal/trade-engine. */
  "mock-engine": "/terminal/mock-engine",
  /** @deprecated Old route redirects to /terminal/trade-engine. */
  "grade-1": "/terminal/grade-1",
  /** @deprecated Old route redirects to /terminal/trade-engine. */
  "grade-2": "/terminal/grade-2",
  /** @deprecated Old route redirects to /terminal/trade-engine. */
  "grade-3": "/terminal/grade-3",
  /** @deprecated Old route redirects to /terminal/trade-engine. */
  "grade-4": "/terminal/grade-4",
  /** @deprecated Old route redirects to /terminal/trade-engine. */
  "grade-5": "/terminal/grade-5",
  execution: "/terminal/execution",
  "strategy-authority": "/terminal/strategy-authority",
  "portfolio-intelligence": "/terminal/portfolio-intelligence",
  "retired-strategies": "/terminal/retired-strategies",
  "main-engine": "/terminal/main-engine",
  strategies: "/terminal/strategies",
  "threshold-config": "/terminal/strategies/threshold-config",
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
  trading: "/terminal/trading",
  "strategy-ops": "/terminal/strategy-ops",
  backtest: "/terminal/backtest",
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

export type CommandCenterNavItem = {
  href: string;
  label: string;
  section: "trading" | "monitor";
  /** MongoDB strategy_authority_profiles current_status for sidebar count badge. */
  countStatus?: StrategyStatus;
  exactMatch?: boolean;
};

/** Institutional strategy pipeline — TRADING section (mandatory order). */
export const TRADING_NAV: CommandCenterNavItem[] = [
  { href: TERMINAL_ROUTES["pre-live-engine"], label: "Pre-Live Engine", section: "trading" },
  { href: TERMINAL_ROUTES["pre-live-strategies"], label: "Pre-Live Strategies", section: "trading" },
  { href: TERMINAL_ROUTES["trade-engine"], label: "Trade Engine", section: "trading" },
  { href: TERMINAL_ROUTES.backtest, label: "Backtest Lab", section: "trading" },
  { href: TERMINAL_ROUTES.strategies, label: "Strategies", section: "trading" },
  { href: TERMINAL_ROUTES["threshold-config"], label: "Trade Thresholds", section: "trading" },
];

/** MONITOR section — mandatory order. */
export const MONITOR_NAV: CommandCenterNavItem[] = [
  { href: TERMINAL_ROUTES.home, label: "Command Center", section: "monitor" },
  { href: TERMINAL_ROUTES.portfolio, label: "Portfolio", section: "monitor" },
  { href: TERMINAL_ROUTES.risk, label: "Risk", section: "monitor" },
  { href: TERMINAL_ROUTES.analytics, label: "Analytics", section: "monitor" },
  { href: TERMINAL_ROUTES.execution, label: "Execution", section: "monitor" },
  { href: TERMINAL_ROUTES.events, label: "Events", section: "monitor" },
  { href: TERMINAL_ROUTES.health, label: "Health", section: "monitor" },
  { href: TERMINAL_ROUTES.diagnostics, label: "Diagnostics", section: "monitor" },
  { href: TERMINAL_ROUTES.settings, label: "Settings", section: "monitor" },
];

/** Full ICC navigation registry — trading pipeline first, then monitor. */
export const COMMAND_CENTER_NAV: CommandCenterNavItem[] = [...TRADING_NAV, ...MONITOR_NAV];

/** Map grade route paths to ISPAP strategy status. */
export const GRADE_ROUTE_STATUS: Record<string, StrategyStatus> = {
  [TERMINAL_ROUTES["grade-5"]]: "GRADE_5",
  [TERMINAL_ROUTES["grade-4"]]: "GRADE_4",
  [TERMINAL_ROUTES["grade-3"]]: "GRADE_3",
  [TERMINAL_ROUTES["grade-2"]]: "GRADE_2",
  [TERMINAL_ROUTES["grade-1"]]: "GRADE_1",
  [TERMINAL_ROUTES["mock-engine"]]: "MAIN_ENGINE",
};

export function gradeStatusFromPath(pathname: string): StrategyStatus | null {
  return GRADE_ROUTE_STATUS[pathname] ?? null;
}
