/**
 * Build a complete AiAppTrackerReport from a snapshot.
 * Pure function — no I/O. Call insertAiTrackerReport() to persist.
 */

import { randomUUID } from "crypto";
import type { AiAppTrackerReport, AiAppTrackerSnapshot } from "./types";
import { TRACKER_MODULE } from "./trackerConstants";
import { summarizeTrackerSnapshot } from "./summarizeTrackerReport";

export function buildTrackerReport(snapshot: AiAppTrackerSnapshot): AiAppTrackerReport {
  const { severity, summary, recommendations } = summarizeTrackerSnapshot(snapshot);

  return {
    report_id: randomUUID(),
    created_at: snapshot.createdAt,
    app_build_sha: snapshot.appBuildSha,
    account_key_suffix: snapshot.accountKeySuffix,
    module: TRACKER_MODULE,
    severity,
    summary,
    snapshot,
    recommendations,
  };
}
