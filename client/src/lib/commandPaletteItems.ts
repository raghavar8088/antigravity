import { COMMAND_CENTER_NAV, TERMINAL_ROUTES } from "@/lib/navRoutes";

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
    group: "Navigate",
    href: item.href,
    keywords: [item.label.toLowerCase(), item.href],
  })),
  {
    id: "toggle-theme",
    label: "Toggle light / dark theme",
    group: "Actions",
    keywords: ["theme", "dark", "light", "appearance"],
  },
  {
    id: "search-trades",
    label: "Open Mock Trading (trades)",
    group: "Trading",
    href: "/mock-trading",
    keywords: ["trades", "orders", "execution", "paper"],
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
];

export const PAGE_TITLES: Record<string, string> = {
  [TERMINAL_ROUTES.home]: "Command Center",
  [TERMINAL_ROUTES.execution]: "Execution",
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
  "/mock-trading": "Mock Trading",
};

export function resolvePageTitle(pathname: string): string {
  if (PAGE_TITLES[pathname]) return PAGE_TITLES[pathname];
  const match = Object.entries(PAGE_TITLES).find(([path]) => pathname.startsWith(`${path}/`));
  return match?.[1] ?? "Command Center";
}
