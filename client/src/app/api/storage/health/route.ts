/**
 * GET /api/storage/health
 *
 * Returns combined health of MongoDB + local filesystem storage.
 * Powers the StorageHealthPanel in the dashboard.
 */

import { NextResponse } from "next/server";
import { isMongoConfigured, pingMongo } from "@/lib/mongoTradesClient";
import { dps } from "@/lib/dualPersistenceService";
import { writeQueue } from "@/lib/writeQueue";

export const dynamic = "force-dynamic";

export async function GET() {
  const mongoConfigured = isMongoConfigured();
  let mongoPingOk = false;
  if (mongoConfigured) {
    mongoPingOk = await pingMongo().catch(() => false);
  }

  const [localHealth] = await Promise.all([
    dps.getStorageHealth().catch((err) => ({
      ok: false as const,
      error: err instanceof Error ? err.message : "Unknown error",
      dataRoot: "",
      totalBytes: 0,
      totalMb: 0,
      filesWrittenToday: 0,
      backupCount: 0,
      lastBackupAt: null,
      auditFileCount: 0,
      writeFailures: 0,
      recentFailures: [],
      queueStats: writeQueue.getStats(),
      uptimeMs: 0,
    })),
  ]);

  return NextResponse.json({
    ok: true,
    serverTime: new Date().toISOString(),
    mongo: {
      configured: mongoConfigured,
      pingOk: mongoPingOk,
      db: process.env.MONGODB_DB?.trim() || "loop_trades",
      status: mongoConfigured && mongoPingOk ? "ONLINE" : mongoConfigured ? "UNREACHABLE" : "NOT_CONFIGURED",
    },
    localStorage: {
      ok: localHealth.ok,
      dataRoot: localHealth.dataRoot,
      totalMb: localHealth.totalMb,
      totalBytes: localHealth.totalBytes,
      filesWrittenToday: localHealth.filesWrittenToday,
      backupCount: localHealth.backupCount,
      lastBackupAt: localHealth.lastBackupAt,
      auditFileCount: localHealth.auditFileCount,
      writeFailures: localHealth.writeFailures,
      recentFailures: localHealth.recentFailures,
      queueStats: localHealth.queueStats,
      uptimeMs: localHealth.uptimeMs,
      status: localHealth.ok ? "HEALTHY" : "DEGRADED",
    },
    syncStatus: {
      mongoOnline: mongoConfigured && mongoPingOk,
      localStorageHealthy: localHealth.ok,
      dualPersistenceActive: mongoConfigured && mongoPingOk && localHealth.ok,
      failedWrites: localHealth.writeFailures,
    },
  });
}
