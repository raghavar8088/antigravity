/** Fixed en-US formatting for desk UI (consistent SSR + client). */

const LOCALE = "en-US";

export function formatDeskUsd(
  value: number,
  opts: { signed?: boolean; decimals?: number } = {},
): string {
  const { signed = false, decimals = 2 } = opts;
  if (!Number.isFinite(value)) return "$0.00";
  const abs = Math.abs(value).toLocaleString(LOCALE, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
  if (signed) return `${value >= 0 ? "+" : "-"}$${abs}`;
  return `$${abs}`;
}

export function formatDeskPct(
  value: number,
  opts: { signed?: boolean; decimals?: number } = {},
): string {
  const { signed = false, decimals = 2 } = opts;
  if (!Number.isFinite(value)) return "0.00%";
  const prefix = signed ? (value >= 0 ? "+" : "-") : "";
  return `${prefix}${Math.abs(value).toFixed(decimals)}%`;
}

export function formatDeskInteger(value: number): string {
  if (!Number.isFinite(value)) return "0";
  return value.toLocaleString(LOCALE, { maximumFractionDigits: 0 });
}

export function formatDeskInr(
  value: number,
  opts: { signed?: boolean; decimals?: number } = {},
): string {
  const { signed = false, decimals = 2 } = opts;
  if (!Number.isFinite(value)) return "₹0.00";
  const abs = Math.abs(value).toLocaleString(LOCALE, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
  if (signed) return `${value >= 0 ? "+" : "-"}₹${abs}`;
  return `₹${abs}`;
}

export function pnlToneClass(value: number): "desk-pnl-positive" | "desk-pnl-negative" | "desk-pnl-neutral" {
  if (value > 0) return "desk-pnl-positive";
  if (value < 0) return "desk-pnl-negative";
  return "desk-pnl-neutral";
}
