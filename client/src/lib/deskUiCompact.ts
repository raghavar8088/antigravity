/**
 * UI-20 compact layout — env gate for Operator Command Center.
 * Default ON when profit mode is ON unless explicitly disabled.
 */
export function deskUiCompactFromEnv(profitModeEnabled = false): boolean {
  const raw = process.env.NEXT_PUBLIC_DESK_UI_COMPACT?.trim();
  if (raw === "0") return false;
  if (raw === "1") return true;
  return profitModeEnabled;
}
