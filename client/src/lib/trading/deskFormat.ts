/**
 * Fixed en-US formatting for desk UI.
 * All functions use a hard-coded locale so SSR and client output always match
 * (avoids hydration mismatches from system locale differences).
 */

import { fmtISTClockShort } from "@/lib/istTime";

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

/** Integer count (contracts, trades, etc.) with thousand-separator. */
export function formatDeskContracts(n: number): string {
  if (!Number.isFinite(n)) return "0";
  return n.toLocaleString(LOCALE, { maximumFractionDigits: 0 });
}

/** "hh:mm AM/PM" IST time extracted from an ISO-8601 string. */
export function formatShortTime(iso: string): string {
  return fmtISTClockShort(iso);
}

/** "Mon D" short date from an ISO-8601 string. */
export function formatShortDate(iso: string): string {
  return new Date(iso).toLocaleDateString(LOCALE, { month: "short", day: "numeric" });
}

/** "Mon D, YYYY" full date from an ISO-8601 string. */
export function formatFullDate(iso: string): string {
  return new Date(iso).toLocaleDateString(LOCALE, { year: "numeric", month: "short", day: "numeric" });
}
