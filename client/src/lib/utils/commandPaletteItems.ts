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
    group: item.section === "trading" ? "Trading Pipeline" : "Navigate",
    href: item.href,
    keywords: [
      item.label.toLowerCase(),
      item.href,
      ...(item.label.includes("Grade") ? ["grade", "mock trading", "pipeline"] : []),
      ...(item.label.includes("Engine") ? ["engine", "main engine", "mock trading"] : []),
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
    label: "Open Mock Trading Desk (execution)",
    group: "Trading",
    href: "/mock-trading",
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
  [TERMINAL_ROUTES["mock-engine"]]: "Mock Trading Engine",
  [TERMINAL_ROUTES["grade-1"]]: "Mock Trading Grade 1",
  [TERMINAL_ROUTES["grade-2"]]: "Mock Trading Grade 2",
  [TERMINAL_ROUTES["grade-3"]]: "Mock Trading Grade 3",
  [TERMINAL_ROUTES["grade-4"]]: "Mock Trading Grade 4",
  [TERMINAL_ROUTES["grade-5"]]: "Mock Trading Grade 5",
  [TERMINAL_ROUTES.execution]: "Execution",
  [TERMINAL_ROUTES["strategy-authority"]]: "Strategy Authority",
  [TERMINAL_ROUTES["portfolio-intelligence"]]: "Portfolio Intelligence",
  [TERMINAL_ROUTES["retired-strategies"]]: "Retired Strategies",
  [TERMINAL_ROUTES["main-engine"]]: "Main Engine",
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
  "/mock-trading": "Mock Trading Desk",
};

export function resolvePageTitle(pathname: string): string {
  if (PAGE_TITLES[pathname]) return PAGE_TITLES[pathname];
  const match = Object.entries(PAGE_TITLES).find(([path]) => pathname.startsWith(`${path}/`));
  return match?.[1] ?? "Command Center";
}
