/**
 * RAIG Time Utilities
 * Robust date parsing and formatting to prevent "Invalid Date" errors.
 */

import { fmtISTClock } from "@/lib/istTime";

export function safeFormatDate(input: string | number | Date | null | undefined): string {
  if (!input) return "—";
  
  const date = new Date(input);
  if (isNaN(date.getTime())) return "—";
  
  return date.toISOString();
}

export function formatShortTime(input: string | number | Date | null | undefined): string {
  if (!input) return "—";
  
  const date = new Date(input);
  if (isNaN(date.getTime())) return "—";
  
  return fmtISTClock(date);
}

export function formatShortDate(input: string | number | Date | null | undefined): string {
  if (!input) return "—";
  
  const date = new Date(input);
  if (isNaN(date.getTime())) return "—";
  
  return date.toLocaleDateString([], { 
    month: "short", 
    day: "numeric" 
  });
}

export function formatElapsed(seconds: number): string {
  if (seconds < 0) return "0s";
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return mins > 0 ? `${mins}m ${secs}s` : `${secs}s`;
}
