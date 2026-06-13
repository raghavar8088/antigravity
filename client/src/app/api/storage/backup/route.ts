/**
 * POST /api/storage/backup
 *   body: { period?: "daily" | "weekly" | "monthly" | "all" }
 *   Triggers an immediate backup for the given period(s).
 *
 * GET /api/storage/backup
 *   Lists all existing backups with metadata.
 */

import { NextResponse } from "next/server";
import { z } from "zod";
import {
  createBackup,
  listBackups,
  validateBackup,
  determinePeriod,
  type BackupPeriod,
} from "@/lib/portfolio/backupManager.server";
import { generateDailyReport, generateWeeklyReport, generateMonthlyReport } from "@/lib/analytics/analyticsReporter";
import { dps } from "@/lib/portfolio/dualPersistenceService";

export const dynamic = "force-dynamic";

const backupBodySchema = z.object({
  period: z.enum(["daily", "weekly", "monthly", "all"]).optional().default("daily"),
  includeReports: z.boolean().optional().default(true),
});

export async function POST(req: Request) {
  let body: unknown;
  try {
    body = await req.json().catch(() => ({}));
  } catch {
    body = {};
  }

  const parsed = backupBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Invalid request body", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  const { period, includeReports } = parsed.data;
  const periods: BackupPeriod[] =
    period === "all" ? ["daily", "weekly", "monthly"] : [period as BackupPeriod];

  const results = [];
  for (const p of periods) {
    const result = await createBackup(p);
    results.push(result);
    dps.emitAuditEvent({
      type: result.ok ? "BACKUP_CREATED" : "BACKUP_FAILED",
      timestamp: new Date().toISOString(),
      message: result.ok ? `${p} backup created: ${result.filename}` : result.error,
      payload: { period: p, sizeBytes: result.sizeBytes, recordCounts: result.recordCounts },
    });
  }

  // Optionally generate analytics reports alongside the backup
  if (includeReports) {
    try {
      await Promise.all([
        generateDailyReport(),
        ...(periods.includes("weekly") ? [generateWeeklyReport()] : []),
        ...(periods.includes("monthly") ? [generateMonthlyReport()] : []),
      ]);
    } catch (err) {
      console.error("[backup/route] Report generation error:", err);
    }
  }

  const allOk = results.every((r) => r.ok);
  return NextResponse.json({
    ok: allOk,
    results,
    autoDetectedPeriods: determinePeriod(),
  }, { status: allOk ? 200 : 207 });
}

export async function GET(req: Request) {
  const url = new URL(req.url);
  const period = url.searchParams.get("period") as BackupPeriod | null;
  const validateAll = url.searchParams.get("validate") === "1";

  const backups = await listBackups(period ?? undefined);

  let enriched = backups;
  if (validateAll) {
    enriched = await Promise.all(
      backups.map(async (b) => {
        const validation = await validateBackup(b.filePath);
        return { ...b, integrity: validation.ok, integrityError: validation.error };
      }),
    );
  }

  return NextResponse.json({
    ok: true,
    count: enriched.length,
    backups: enriched,
  });
}
