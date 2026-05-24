/** Pure tab keys for Operator Command Center (UI-20). */
export const DESK_COMMAND_CENTER_TABS = [
  "today",
  "health",
  "gates",
  "advanced",
] as const;

export type DeskCommandCenterTab = (typeof DESK_COMMAND_CENTER_TABS)[number];

export const DEFAULT_COMMAND_CENTER_TAB: DeskCommandCenterTab = "today";

export function commandCenterTabLabel(tab: DeskCommandCenterTab): string {
  switch (tab) {
    case "today":
      return "Today";
    case "health":
      return "Health";
    case "gates":
      return "Gates";
    case "advanced":
      return "Advanced";
    default:
      return tab;
  }
}
