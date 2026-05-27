/**
 * Summarize an AiAppTrackerSnapshot into severity + summary + recommendations.
 * Pure function — no I/O.
 *
 * Hard invariant: never recommend lowering the signal threshold.
 */

import type { AiAppTrackerSnapshot, AiTrackerSeverity } from "./types";

// ── Recommendation map keyed on warning substring patterns ────────────────────

type RecommendationRule = {
  match: (warning: string) => boolean;
  recommendation: string;
};

const RECOMMENDATION_RULES: RecommendationRule[] = [
  {
    match: (w) => w.includes("Worker heartbeat is stale") || w.includes("stale"),
    recommendation:
      "Restart the VPS pm2 worker: `pm2 restart btc-ft-paper-worker` on the AWS Lightsail instance. The Vercel cron fallback fires every 60s as a backup.",
  },
  {
    match: (w) => w.includes("Balance drifted"),
    recommendation:
      "Run `computeSessionEquityFromProduction()` to verify drift is real PnL and not probe/bootstrap trade contamination. Check `isProbeOrBootstrapTrade()` coverage.",
  },
  {
    match: (w) => w.includes("Entries are paused"),
    recommendation:
      "Entries are paused. Set `pauseEntries: false` in paper_state via the desk UI or MongoDB Atlas to resume trading.",
  },
  {
    match: (w) => w.includes("No entry candidates") || w.includes("Dominant blocker"),
    recommendation:
      "Check the Entry Funnel Card in the browser UI. Common causes: signal not met (correct behavior), regime chop, rotation suspended, or session window closed.",
  },
  {
    match: (w) => w.includes("DESK_WORKER_ACCOUNT_KEY is not set"),
    recommendation:
      "Set the DESK_WORKER_ACCOUNT_KEY environment variable to the account key used by the paper desk. This is required for all desk state reads.",
  },
  {
    match: (w) => w.includes("No paper_state found"),
    recommendation:
      "No paper_state exists for this account yet. Ensure the VPS worker has run at least one tick, or POST to /api/paper-state to initialize.",
  },
  {
    match: (w) => w.includes("MongoDB is not configured"),
    recommendation:
      "Set the MONGODB_URI environment variable to a valid Atlas M0 connection string. All desk state is stored in MongoDB.",
  },
  {
    match: (w) => w.includes("funnel snapshot is stale"),
    recommendation:
      "Entry funnel snapshot is stale. Reload the desk page (browser poll) or restart the VPS worker (`pm2 restart btc-ft-paper-worker`).",
  },
];

// ── Severity rules (first match wins) ────────────────────────────────────────

function computeSeverity(snapshot: AiAppTrackerSnapshot): AiTrackerSeverity {
  // danger: worker stale with no opened positions
  if (snapshot.worker.stale && snapshot.paperState.openPositions === 0) {
    return "danger";
  }
  // danger: very large balance drift (> $100)
  if (
    snapshot.paperState.balanceDriftUsd != null &&
    Math.abs(snapshot.paperState.balanceDriftUsd) > 100
  ) {
    return "danger";
  }
  // danger: no account key
  if (!snapshot.accountKeySuffix) {
    return "danger";
  }
  // warning: any warnings present
  if (snapshot.warnings.length > 0) {
    return "warning";
  }
  return "info";
}

// ── Summary text ──────────────────────────────────────────────────────────────

function buildSummary(snapshot: AiAppTrackerSnapshot, severity: AiTrackerSeverity): string {
  if (severity === "info") {
    const openStr =
      snapshot.paperState.openPositions > 0
        ? `, ${snapshot.paperState.openPositions} open position${snapshot.paperState.openPositions !== 1 ? "s" : ""}`
        : "";
    const blocker = snapshot.entryFunnel.dominantBlocker ?? "none";
    const v = (snapshot as any).verificationSummary;
    const vLine = v ? ` Verification: blocker=${v.latestRootCause ?? blocker} opened=${v.opened ?? 0} closed=${v.closed ?? 0} worker=${snapshot.worker.stale ? "STALE" : "LIVE"}` : "";
    return `Desk healthy. Worker ${snapshot.worker.stale ? "stale" : "live"}${openStr}. Blocker: ${blocker}.${vLine}`;
  }
  if (severity === "danger") {
    return `DANGER: ${snapshot.warnings[0] ?? "Critical desk issue — see warnings."}`;
  }
  return `Warning: ${snapshot.warnings[0] ?? "Desk needs attention — see warnings."}`;
}

// ── Recommendations ───────────────────────────────────────────────────────────

function buildRecommendations(snapshot: AiAppTrackerSnapshot): string[] {
  if (snapshot.warnings.length === 0) {
    return [
      "Desk looks healthy. Monitor the Entry Funnel Card for the next dominant blocker.",
      "For AI agents: run `npm run ai:summary` from client/ before making changes.",
    ];
  }

  const seen = new Set<string>();
  const recs: string[] = [];

  for (const warning of snapshot.warnings) {
    for (const rule of RECOMMENDATION_RULES) {
      if (rule.match(warning) && !seen.has(rule.recommendation)) {
        seen.add(rule.recommendation);
        recs.push(rule.recommendation);
        break;
      }
    }
  }

  recs.push("For AI agents: run `npm run ai:summary` from client/ before making changes.");
  return recs;
}

// ── Main export ───────────────────────────────────────────────────────────────

export function summarizeTrackerSnapshot(snapshot: AiAppTrackerSnapshot): {
  severity: AiTrackerSeverity;
  summary: string;
  recommendations: string[];
} {
  const severity = computeSeverity(snapshot);
  const summary = buildSummary(snapshot, severity);
  const recommendations = buildRecommendations(snapshot);
  return { severity, summary, recommendations };
}
