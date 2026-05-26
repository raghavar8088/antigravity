/**
 * Summarize an AiTrackerSnapshot into severity + summary string + recommendations.
 * Pure function — no I/O.
 */

import type { AiTrackerSnapshot, AiTrackerSeverity, AiTrackerWarningCode } from "./types";

const WARNING_RECOMMENDATIONS: Record<AiTrackerWarningCode, string> = {
  stale_worker:
    "Restart the pm2 worker: `pm2 restart btc-ft-paper-worker` on VPS. The Vercel cron fallback fires every 60s as backup.",
  stale_deployment:
    "A newer build SHA is available. Force-redeploy on Vercel or push an empty commit to trigger CI.",
  no_trades_30min:
    "Check the dominant blocker in the Entry Funnel Card. Common causes: signal threshold not met (correct), regime chop, rotation suspended, or roster empty.",
  balance_drift:
    "Run `computeSessionEquityFromProduction()` to verify drift is real PnL and not probe-trade contamination.",
  probe_dominant:
    "Ensure `isProbeOrBootstrapTrade()` filters are applied upstream of all metric computations. Check paper_trades for unexpected PAPER_ENTRY_CANARY or PAPER_BOOTSTRAP_PROBE documents.",
  high_fee_gross:
    "Fee drag is high relative to gross. Consider raising NEXT_PUBLIC_BTC_FT_MIN_MOVE_K_MUL to tighten the ATR/fee gate.",
  no_strategy_candidates:
    "All strategies are suspended by rotation. Inspect `rotation_report` in MongoDB `paper_state`. Wait for performance recovery or manually restore a strategy via the Rotation panel.",
  no_open_positions_long:
    "No open positions for an extended period. Check entry gate diagnostics; verify market session is active.",
  data_gap:
    "Funnel snapshot is stale. Browser poll loop or worker may have stopped. Reload the desk page or restart the worker.",
};

const SEVERITY_RULES: Array<{
  test: (snap: AiTrackerSnapshot) => boolean;
  severity: AiTrackerSeverity;
}> = [
  // Danger: worker completely stale + no trades
  {
    test: (s) =>
      s.warnings.some((w) => w.code === "stale_worker") &&
      s.warnings.some((w) => w.code === "no_trades_30min"),
    severity: "danger",
  },
  // Danger: no strategy candidates (all suspended)
  {
    test: (s) => s.warnings.some((w) => w.code === "no_strategy_candidates"),
    severity: "danger",
  },
  // Danger: balance drift > $100
  {
    test: (s) => s.balanceDriftUsd != null && Math.abs(s.balanceDriftUsd) > 100,
    severity: "danger",
  },
  // Warning: any single warning code
  {
    test: (s) => s.warnings.length > 0,
    severity: "warning",
  },
];

export function computeReportSeverity(snapshot: AiTrackerSnapshot): AiTrackerSeverity {
  for (const rule of SEVERITY_RULES) {
    if (rule.test(snapshot)) return rule.severity;
  }
  return "info";
}

export function buildReportSummary(snapshot: AiTrackerSnapshot, severity: AiTrackerSeverity): string {
  if (severity === "info") {
    const openStr = snapshot.openPositionsCount > 0
      ? `, ${snapshot.openPositionsCount} open position${snapshot.openPositionsCount !== 1 ? "s" : ""}`
      : "";
    return `Desk healthy. Blocker: ${snapshot.dominantBlocker}${openStr}. ${snapshot.closedTradesCount} trades closed this session.`;
  }

  const warnLabels = snapshot.warnings.map((w) => w.code).join(", ");
  if (severity === "danger") {
    return `DANGER: ${snapshot.warnings[0]?.message ?? "Critical desk issue"}. Warnings: ${warnLabels}.`;
  }
  return `Warning: ${snapshot.warnings[0]?.message ?? "Desk needs attention"}. Flags: ${warnLabels}.`;
}

export function buildRecommendations(snapshot: AiTrackerSnapshot): string[] {
  if (snapshot.warnings.length === 0) {
    return ["Desk looks healthy. Monitor the Entry Funnel Card for the next dominant blocker."];
  }

  const seen = new Set<string>();
  const recs: string[] = [];
  for (const w of snapshot.warnings) {
    const rec = WARNING_RECOMMENDATIONS[w.code];
    if (rec && !seen.has(rec)) {
      seen.add(rec);
      recs.push(rec);
    }
  }

  // Always append the "read first" pointer for AI agents
  recs.push("For AI agents: run `npm run ai:summary` from client/ before making changes.");

  return recs;
}
