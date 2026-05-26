/**
 * Re-export shim — canonical implementation moved to lib/aiAppTracker/aiAppTrackerMongo.ts
 * Keep this shim so existing import paths (@/lib/aiAppTrackerMongo) continue to resolve.
 */
export {
  insertAiTrackerReport,
  listAiTrackerReports,
  getLatestAiTrackerReport,
} from "@/lib/aiAppTracker/aiAppTrackerMongo";
