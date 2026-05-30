/**
 * POST /api/storage/restore
 *   body: { filename: string; period: "daily"|"weekly"|"monthly"; targetDir?: string }
 *   Validates a backup then restores it to targetDir (or data/restored/<date>/).
 *
 * GET /api/storage/restore
 *   Lists available backups (same as GET /api/storage/backup).
 */

import { NextResponse } from "next/server";
import { z } from "zod";
import path from "path";
import { restoreFromBackup, validateBackup, listBackups } from "@/lib/backupManager";
import { getDataRoot, DATA_DIRS } from "@/lib/localStorageService";
import { dps } from "@/lib/dualPersistenceService";

export const dynamic = "force-dynamic";

const restoreBodySchema = z.object({
  filename: z.string().min(1),
  period: z.enum(["daily", "weekly", "monthly"]),
  targetDir: z.string().optional(),
  validateOnly: z.boolean().optional().default(false),
});

export async function POST(req: Request) {
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, code: "INVALID_JSON", error: "Invalid JSON body" }, { status: 400 });
  }

  const parsed = restoreBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Invalid request", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  const { filename, period, targetDir, validateOnly } = parsed.data;

  // Prevent path traversal
  const safeFilename = path.basename(filename);
  if (!safeFilename.endsWith(".json.gz")) {
    return NextResponse.json({ ok: false, code: "INVALID_FILENAME", error: "Must be a .json.gz backup file" }, { status: 400 });
  }

  const filePath = path.join(getDataRoot(), DATA_DIRS.backups, period, safeFilename);

  // Always validate first
  const validation = await validateBackup(filePath);
  if (!validation.ok) {
    return NextResponse.json({
      ok: false,
      code: "INTEGRITY_CHECK_FAILED",
      error: validation.error,
      manifest: validation.manifest ?? null,
    }, { status: 422 });
  }

  if (validateOnly) {
    return NextResponse.json({
      ok: true,
      validated: true,
      manifest: validation.manifest,
    });
  }

  dps.emitAuditEvent({
    type: "RESTORE_STARTED",
    timestamp: new Date().toISOString(),
    message: `Restore initiated from ${safeFilename}`,
    payload: { period, filename: safeFilename },
  });

  const result = await restoreFromBackup(filePath, targetDir);

  dps.emitAuditEvent({
    type: "RESTORE_COMPLETED",
    timestamp: new Date().toISOString(),
    message: result.ok ? `Restore completed from ${safeFilename}` : `Restore failed: ${result.error}`,
    payload: { period, filename: safeFilename, recordsRestored: result.recordsRestored, warnings: result.warnings },
  });

  return NextResponse.json({
    ok: result.ok,
    manifest: validation.manifest,
    recordsRestored: result.recordsRestored,
    warnings: result.warnings,
    error: result.error,
  }, { status: result.ok ? 200 : 500 });
}

export async function GET() {
  const backups = await listBackups();
  return NextResponse.json({ ok: true, count: backups.length, backups });
}
