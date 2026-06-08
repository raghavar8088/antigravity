/** Paper Desk first-class route — canonical path for the Go Engine dashboard. */
export const PAPER_DESK_PATH = "/paper-desk";

const PAPER_DESK_TAB_KEYS = ["positions", "trades", "orders", "equity", "strategies"] as const;
export type PaperDeskTabKey = (typeof PAPER_DESK_TAB_KEYS)[number];

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

export function paperDeskHref(tab?: PaperDeskTabKey): string {
  return tab ? `${PAPER_DESK_PATH}?tab=${tab}` : PAPER_DESK_PATH;
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
