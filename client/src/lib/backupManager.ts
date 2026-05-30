/**
 * Backup, restore, and file-rotation manager.
 *
 * Backups are single .json.gz files containing a JSON manifest + all data
 * from each category. Compression uses Node.js built-in zlib.
 *
 * Retention policy (enforced after each backup run):
 *   Daily   — 30 files
 *   Weekly  — 52 files
 *   Monthly — 12 files
 */

import fs from "fs";
import path from "path";
import zlib from "zlib";
import { promisify } from "util";
import {
  getDataRoot,
  DATA_DIRS,
  initLocalStorage,
  sha256Buffer,
  readNdjson,
  readJson,
  listFiles,
  type DataCategory,
} from "./localStorageService";

const gzip = promisify(zlib.gzip);
const gunzip = promisify(zlib.gunzip);

// ── Types ──────────────────────────────────────────────────────────────────

export type BackupPeriod = "daily" | "weekly" | "monthly";

export interface BackupManifest {
  version: number;
  period: BackupPeriod;
  createdAt: string;
  dataRoot: string;
  categories: string[];
  recordCounts: Record<string, number>;
  checksum: string; // sha256 of the payload JSON before compression
}

export interface BackupResult {
  ok: boolean;
  period: BackupPeriod;
  filename: string;
  filePath: string;
  sizeBytes: number;
  recordCounts: Record<string, number>;
  createdAt: string;
  error?: string;
}

export interface RestoreResult {
  ok: boolean;
  filename: string;
  recordsRestored: Record<string, number>;
  warnings: string[];
  error?: string;
}

export interface BackupListEntry {
  period: BackupPeriod;
  filename: string;
  filePath: string;
  sizeBytes: number;
  createdAt: string;
}

const RETENTION: Record<BackupPeriod, number> = {
  daily: 30,
  weekly: 52,
  monthly: 12,
};

// ── Create Backup ──────────────────────────────────────────────────────────

export async function createBackup(period: BackupPeriod): Promise<BackupResult> {
  initLocalStorage();
  const root = getDataRoot();
  const now = new Date();
  const timestamp = now.toISOString().replace(/[:.]/g, "-").slice(0, 19);
  const filename = `backup-${period}-${timestamp}.json.gz`;
  const filePath = path.join(root, DATA_DIRS.backups, period, filename);

  const categories: DataCategory[] = [
    "trades", "mockTrades", "positions", "signals",
    "research", "risk", "equity", "metrics", "audit", "configs",
  ];

  const payload: Record<string, unknown[]> = {};
  const recordCounts: Record<string, number> = {};

  // Collect data from each category
  for (const cat of categories) {
    const files = await listFiles(cat);
    const records: unknown[] = [];
    for (const file of files) {
      if (file.endsWith(".ndjson")) {
        const lines = await readNdjson(file);
        records.push(...lines);
      } else if (file.endsWith(".json")) {
        const data = await readJson(file);
        if (Array.isArray(data)) records.push(...data);
        else if (data != null) records.push(data);
      }
    }
    payload[cat] = records;
    recordCounts[cat] = records.length;
  }

  const payloadJson = JSON.stringify(payload);
  const checksum = sha256Buffer(Buffer.from(payloadJson, "utf8"));

  const manifest: BackupManifest = {
    version: 1,
    period,
    createdAt: now.toISOString(),
    dataRoot: root,
    categories,
    recordCounts,
    checksum,
  };

  const bundle = JSON.stringify({ manifest, data: payload });
  let compressed: Buffer;
  try {
    compressed = await gzip(Buffer.from(bundle, "utf8"));
  } catch (err) {
    return {
      ok: false, period, filename, filePath,
      sizeBytes: 0, recordCounts, createdAt: now.toISOString(),
      error: `Compression failed: ${err instanceof Error ? err.message : String(err)}`,
    };
  }

  const tmp = `${filePath}.tmp`;
  try {
    await fs.promises.writeFile(tmp, compressed);
    await fs.promises.rename(tmp, filePath);
  } catch (err) {
    return {
      ok: false, period, filename, filePath,
      sizeBytes: 0, recordCounts, createdAt: now.toISOString(),
      error: `Write failed: ${err instanceof Error ? err.message : String(err)}`,
    };
  }

  const stat = await fs.promises.stat(filePath);
  await enforceRetention(period);

  return {
    ok: true, period, filename, filePath,
    sizeBytes: stat.size, recordCounts, createdAt: now.toISOString(),
  };
}

// ── Restore from Backup ────────────────────────────────────────────────────

export async function restoreFromBackup(
  filePath: string,
  targetDir?: string,
): Promise<RestoreResult> {
  const warnings: string[] = [];
  const filename = path.basename(filePath);

  let compressed: Buffer;
  try {
    compressed = await fs.promises.readFile(filePath);
  } catch (err) {
    return { ok: false, filename, recordsRestored: {}, warnings, error: `Cannot read file: ${err instanceof Error ? err.message : String(err)}` };
  }

  let bundle: { manifest: BackupManifest; data: Record<string, unknown[]> };
  try {
    const decompressed = await gunzip(compressed);
    bundle = JSON.parse(decompressed.toString("utf8"));
  } catch (err) {
    return { ok: false, filename, recordsRestored: {}, warnings, error: `Decompression/parse failed: ${err instanceof Error ? err.message : String(err)}` };
  }

  const { manifest, data } = bundle;

  // Integrity check
  const payloadJson = JSON.stringify(data);
  const actualChecksum = sha256Buffer(Buffer.from(payloadJson, "utf8"));
  if (actualChecksum !== manifest.checksum) {
    warnings.push(`Checksum mismatch — expected ${manifest.checksum}, got ${actualChecksum}. Proceeding with caution.`);
  }

  const restoreRoot = targetDir ?? path.join(getDataRoot(), "restored", manifest.createdAt.slice(0, 10));
  const recordsRestored: Record<string, number> = {};

  for (const [cat, records] of Object.entries(data)) {
    if (!Array.isArray(records) || records.length === 0) continue;
    const dir = path.join(restoreRoot, cat);
    fs.mkdirSync(dir, { recursive: true });
    const outPath = path.join(dir, `restored-${manifest.createdAt.slice(0, 10)}.json`);
    const tmp = `${outPath}.tmp`;
    await fs.promises.writeFile(tmp, JSON.stringify(records, null, 2), "utf8");
    await fs.promises.rename(tmp, outPath);
    recordsRestored[cat] = records.length;
  }

  // Write restore report
  const reportPath = path.join(restoreRoot, "restore-report.json");
  await fs.promises.writeFile(
    reportPath,
    JSON.stringify({ restoredFrom: filename, manifest, recordsRestored, warnings, restoredAt: new Date().toISOString() }, null, 2),
    "utf8",
  );

  return { ok: true, filename, recordsRestored, warnings };
}

// ── Validate Backup Integrity ──────────────────────────────────────────────

export async function validateBackup(filePath: string): Promise<{ ok: boolean; manifest?: BackupManifest; error?: string }> {
  let compressed: Buffer;
  try {
    compressed = await fs.promises.readFile(filePath);
  } catch (err) {
    return { ok: false, error: `Cannot read: ${err instanceof Error ? err.message : String(err)}` };
  }
  try {
    const decompressed = await gunzip(compressed);
    const bundle: { manifest: BackupManifest; data: Record<string, unknown[]> } = JSON.parse(decompressed.toString("utf8"));
    const actualChecksum = sha256Buffer(Buffer.from(JSON.stringify(bundle.data), "utf8"));
    if (actualChecksum !== bundle.manifest.checksum) {
      return { ok: false, manifest: bundle.manifest, error: "Checksum mismatch" };
    }
    return { ok: true, manifest: bundle.manifest };
  } catch (err) {
    return { ok: false, error: `Validation failed: ${err instanceof Error ? err.message : String(err)}` };
  }
}

// ── List Backups ───────────────────────────────────────────────────────────

export async function listBackups(period?: BackupPeriod): Promise<BackupListEntry[]> {
  const root = getDataRoot();
  const periods: BackupPeriod[] = period ? [period] : ["daily", "weekly", "monthly"];
  const result: BackupListEntry[] = [];

  for (const p of periods) {
    const dir = path.join(root, DATA_DIRS.backups, p);
    try {
      const entries = await fs.promises.readdir(dir);
      for (const entry of entries) {
        if (!entry.endsWith(".json.gz")) continue;
        const filePath = path.join(dir, entry);
        const stat = await fs.promises.stat(filePath);
        // Parse date from filename: backup-daily-2026-05-31T12-00-00
        const match = entry.match(/(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2})/);
        const createdAt = match ? match[1].replace(/T(\d{2})-(\d{2})-(\d{2})/, "T$1:$2:$3") : stat.mtime.toISOString();
        result.push({ period: p, filename: entry, filePath, sizeBytes: stat.size, createdAt });
      }
    } catch {
      // Directory may not exist yet
    }
  }

  return result.sort((a, b) => b.createdAt.localeCompare(a.createdAt));
}

// ── Retention enforcement ──────────────────────────────────────────────────

async function enforceRetention(period: BackupPeriod): Promise<void> {
  const root = getDataRoot();
  const dir = path.join(root, DATA_DIRS.backups, period);
  const keep = RETENTION[period];
  try {
    const files = (await fs.promises.readdir(dir))
      .filter((f) => f.endsWith(".json.gz"))
      .map((f) => ({ name: f, full: path.join(dir, f) }))
      .sort((a, b) => b.name.localeCompare(a.name)); // newest first

    const toDelete = files.slice(keep);
    await Promise.all(toDelete.map((f) => fs.promises.unlink(f.full).catch(() => undefined)));
  } catch {
    /* ignore */
  }
}

// ── Determine Backup Period ────────────────────────────────────────────────

export function determinePeriod(): BackupPeriod[] {
  const now = new Date();
  const periods: BackupPeriod[] = ["daily"];
  if (now.getDay() === 0) periods.push("weekly");           // Sunday
  if (now.getDate() === 1) periods.push("monthly");          // 1st of month
  return periods;
}
