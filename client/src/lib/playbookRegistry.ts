/**
 * Registry of the four signal/confirmation playbook overlays.
 *
 * Playbooks are NOT separate desks — they filter the active strategy roster
 * and adjust signal threshold, hold multiplier, and cooldown multiplier within
 * a time-horizon desk. Selecting "All" (no playbook) runs the full desk roster.
 *
 *   trend     — HTF-aligned continuation; adds holdMul to stay in winners
 *   range     — Mean-reversion / Wyckoff / VWAP; requires low ADX context
 *   breakout  — Volume-confirmed structure break; tighter hold, faster cooldown
 *   momentum  — RSI/MACD acceleration; no hold change, slightly shorter cooldown
 */

import type { PlaybookId } from "@/lib/futuresStratTypes";
import type { PlaybookProfile } from "@/lib/tradingStyleTypes";

export const PLAYBOOK_PROFILES: Readonly<Record<PlaybookId, PlaybookProfile>> = {
  trend: {
    id: "trend",
    label: "Trend",
    categoryAllowlist: ["Trend", "MTF Trend", "MTF MACD", "MTF ADX"],
    signalDelta: 0,
    holdMul: 1.1,
    cooldownMul: 1.0,
    description: "HTF-aligned trend continuation with trailing exits",
  },
  range: {
    id: "range",
    label: "Range",
    categoryAllowlist: [
      "Wyckoff",
      "VWAP MR",
      "VWAP",
      "MeanRev",
      "PREMIUM VWAP Reject",
    ],
    signalDelta: 0,
    holdMul: 1.0,
    cooldownMul: 1.2,
    description: "Mean-reversion inside identified range; low ADX context preferred",
  },
  breakout: {
    id: "breakout",
    label: "Breakout",
    categoryAllowlist: [
      "Breakout",
      "MTF Break",
      "Order Flow",
      "Session",
    ],
    signalDelta: 2,
    holdMul: 0.9,
    cooldownMul: 0.8,
    description: "Volume-confirmed range break; SL inside prior range, faster cooldown",
  },
  momentum: {
    id: "momentum",
    label: "Momentum",
    categoryAllowlist: [
      "Smart Money",
      "Order Flow",
      "Session",
      "PREMIUM Vol Divergence",
      "Momentum",
    ],
    signalDelta: 0,
    holdMul: 1.0,
    cooldownMul: 0.9,
    description: "RSI/MACD acceleration confirms impulse follow-through",
  },
};

export function getPlaybookProfile(id: PlaybookId): PlaybookProfile {
  return PLAYBOOK_PROFILES[id];
}

/** All four playbook IDs in display order. */
export const PLAYBOOK_IDS: readonly PlaybookId[] = [
  "trend",
  "range",
  "breakout",
  "momentum",
];

/**
 * Returns true when a strategy's category passes the playbook's allowlist.
 * When the allowlist is empty, all categories pass.
 */
export function categoryPassesPlaybook(category: string, profile: PlaybookProfile): boolean {
  if (profile.categoryAllowlist.length === 0) return true;
  return profile.categoryAllowlist.includes(category);
}
