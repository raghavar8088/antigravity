/**
 * Build a complete AiTrackerReport from a snapshot.
 * Pure function — no I/O. Call insertAiTrackerReport() to persist.
 */

import { randomUUID } from "crypto";
import type { AiTrackerReport, AiTrackerSnapshot } from "./types";
import { TRACKER_MODULE } from "./trackerConstants";
import { computeReportSeverity, buildReportSummary, buildRecommendations } from "./summarizeTrackerReport";

export function buildTrackerReport(snapshot: AiTrackerSnapshot): AiTrackerReport {
  const severity = computeReportSeverity(snapshot);
  const summary = buildReportSummary(snapshot, severity);
  const recommendations = buildRecommendations(snapshot);

  return {
    report_id: randomUUID(),
    created_at: snapshot.capturedAt,
    app_build_sha: snapshot.buildSha,
    account_key_suffix: snapshot.accountKeySuffix,
    module: TRACKER_MODULE,
    severity,
    summary,
    snapshot,
    recommendations,
  };
}
