import { COMMAND_CENTER_NAV, TERMINAL_ROUTES } from "@/lib/utils/navRoutes";

export type CommandPaletteItem = {
  id: string;
  label: string;
  group: string;
  href?: string;
  keywords?: string[];
  action?: () => void;
};

export const COMMAND_PALETTE_ITEMS: CommandPaletteItem[] = [
  ...COMMAND_CENTER_NAV.map((item) => ({
    id: item.href,
    label: item.label,
    group: item.section === "trading" ? "Trading" : "Navigate",
    href: item.href,
    keywords: [
      item.label.toLowerCase(),
      item.href,
      ...(item.label.includes("Engine") ? ["engine", "trade engine"] : []),
    ],
  })),
  {
    id: "toggle-theme",
    label: "Toggle light / dark theme",
    group: "Actions",
    keywords: ["theme", "dark", "light", "appearance"],
  },
  {
    id: "search-trades",
    label: "Open Trade Engine (execution)",
    group: "Trading",
    href: TERMINAL_ROUTES["trade-engine"],
    keywords: ["trades", "orders", "execution", "paper", "desk"],
  },
  {
    id: TERMINAL_ROUTES.observability,
    label: "Observability",
    group: "Navigate",
    href: TERMINAL_ROUTES.observability,
    keywords: ["metrics", "latency", "monitoring"],
  },
  {
    id: TERMINAL_ROUTES.journal,
    label: "Trade Journal",
    group: "Navigate",
    href: TERMINAL_ROUTES.journal,
    keywords: ["journal", "notes", "history"],
  },
  {
    id: "/terminal/design-system",
    label: "Design System",
    group: "Navigate",
    href: "/terminal/design-system",
    keywords: ["design", "tokens", "components", "m3", "preview"],
  },
];

export const PAGE_TITLES: Record<string, string> = {
  [TERMINAL_ROUTES.home]: "Command Center",
  [TERMINAL_ROUTES["trade-engine"]]: "Trade Engine",
  [TERMINAL_ROUTES["mock-engine"]]: "Trade Engine",
  [TERMINAL_ROUTES["grade-1"]]: "Trade Engine",
  [TERMINAL_ROUTES["grade-2"]]: "Trade Engine",
  [TERMINAL_ROUTES["grade-3"]]: "Trade Engine",
  [TERMINAL_ROUTES["grade-4"]]: "Trade Engine",
  [TERMINAL_ROUTES["grade-5"]]: "Trade Engine",
  [TERMINAL_ROUTES.execution]: "Execution",
  [TERMINAL_ROUTES["strategy-authority"]]: "Strategy Authority",
  [TERMINAL_ROUTES["portfolio-intelligence"]]: "Portfolio Intelligence",
  [TERMINAL_ROUTES["retired-strategies"]]: "Retired Strategies",
  [TERMINAL_ROUTES["main-engine"]]: "Trade Engine",
  [TERMINAL_ROUTES.strategies]: "Strategies",
  [TERMINAL_ROUTES.portfolio]: "Portfolio",
  [TERMINAL_ROUTES.risk]: "Risk",
  [TERMINAL_ROUTES.analytics]: "Analytics",
  [TERMINAL_ROUTES.research]: "Research",
  [TERMINAL_ROUTES.events]: "Events",
  [TERMINAL_ROUTES.health]: "Health",
  [TERMINAL_ROUTES.diagnostics]: "Diagnostics",
  [TERMINAL_ROUTES.observability]: "Observability",
  [TERMINAL_ROUTES.journal]: "Journal",
  [TERMINAL_ROUTES.settings]: "Settings",
  "/terminal/design-system": "Design System",
  "/mock-trading": "Trade Engine",
  [TERMINAL_ROUTES.backtest]: "Backtest Lab",
  [TERMINAL_ROUTES["pre-live-engine"]]: "Pre-Live Engine",
  [TERMINAL_ROUTES["pre-live-strategies"]]: "Pre-Live Strategies",
};

export function resolvePageTitle(pathname: string): string {
  if (PAGE_TITLES[pathname]) return PAGE_TITLES[pathname];
  const match = Object.entries(PAGE_TITLES).find(([path]) => pathname.startsWith(`${path}/`));
  return match?.[1] ?? "Command Center";
}
