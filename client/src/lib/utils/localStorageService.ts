/**
 * Filesystem persistence layer.
 *
 * DATA_ROOT (env: LOCAL_DATA_DIR) defaults to <repo-root>/data/.
 * When Next.js runs from client/, process.cwd() is client/, so
 * path.resolve(cwd, '..', 'data') lands at the repo root.
 *
 * All writes are atomic: data is written to <file>.tmp then renamed
 * to the target path so partial writes never corrupt existing files.
 *
 * NDJSON files rotate when they exceed MAX_NDJSON_BYTES (50 MB).
 */

import crypto from "crypto";
import fs from "fs";
import path from "path";

// ── Root directory ─────────────────────────────────────────────────────────

export function getDataRoot(): string {
  return (
    process.env.LOCAL_DATA_DIR?.trim() ||
    path.resolve(process.cwd(), "..", "data")
  );
}

// ── Sub-directory constants ────────────────────────────────────────────────

export const DATA_DIRS = {
  trades: "trades",
  mockTrades: "mock-trades",
  positions: "positions",
  signals: "signals",
  research: "research",
  risk: "risk",
  equity: "equity",
  metrics: "metrics",
  audit: "audit",
  configs: "configs",
  reports: "reports",
  backups: "backups",
} as const;

export type DataCategory = keyof typeof DATA_DIRS;

const MAX_NDJSON_BYTES = 50 * 1024 * 1024; // 50 MB

// ── Initialization ─────────────────────────────────────────────────────────

let initialized = false;

export function initLocalStorage(): void {
  if (initialized) return;
  const root = getDataRoot();
  for (const dir of Object.values(DATA_DIRS)) {
    fs.mkdirSync(path.join(root, dir), { recursive: true });
  }
  // Backup sub-folders
  for (const period of ["daily", "weekly", "monthly"] as const) {
    fs.mkdirSync(path.join(root, DATA_DIRS.backups, period), { recursive: true });
  }
  // Report sub-folders
  for (const period of ["daily", "weekly", "monthly"] as const) {
    fs.mkdirSync(path.join(root, DATA_DIRS.reports, period), { recursive: true });
  }
  initialized = true;
}

// ── Path helpers ───────────────────────────────────────────────────────────

export function datePath(category: DataCategory, filename: string): string {
  const safe = sanitizeFilename(filename);
  return path.join(getDataRoot(), DATA_DIRS[category], safe);
}

export function todayPrefix(): string {
  return new Date().toISOString().slice(0, 10); // "2026-05-31"
}

export function todayFilePath(category: DataCategory, suffix: string): string {
  return datePath(category, `${todayPrefix()}-${suffix}`);
}

/** Prevent path traversal: strip any directory separators from the filename. */
function sanitizeFilename(name: string): string {
  return path.basename(name).replace(/[^a-zA-Z0-9._\-]/g, "_");
}

// ── Atomic JSON write ──────────────────────────────────────────────────────

export async function atomicWriteJson(filePath: string, data: unknown): Promise<void> {
  initLocalStorage();
  const tmp = `${filePath}.tmp`;
  const json = JSON.stringify(data, null, 2);
  await fs.promises.writeFile(tmp, json, "utf8");
  await fs.promises.rename(tmp, filePath);
}

// ── Atomic JSON read ───────────────────────────────────────────────────────

export async function readJson<T = unknown>(filePath: string): Promise<T | null> {
  try {
    const raw = await fs.promises.readFile(filePath, "utf8");
    return JSON.parse(raw) as T;
  } catch {
    return null;
  }
}

// ── Append NDJSON ──────────────────────────────────────────────────────────

export async function appendNdjson(filePath: string, record: unknown): Promise<void> {
  initLocalStorage();
  await rotateNdjsonIfNeeded(filePath);
  const line = JSON.stringify(record) + "\n";
  await fs.promises.appendFile(filePath, line, "utf8");
}

async function rotateNdjsonIfNeeded(filePath: string): Promise<void> {
  try {
    const stat = await fs.promises.stat(filePath);
    if (stat.size < MAX_NDJSON_BYTES) return;
    const rotated = `${filePath}.${Date.now()}.rotated`;
    await fs.promises.rename(filePath, rotated);
  } catch {
    // File does not exist yet — nothing to rotate.
  }
}

// ── Read NDJSON ────────────────────────────────────────────────────────────

export async function readNdjson<T = unknown>(filePath: string): Promise<T[]> {
  try {
    const raw = await fs.promises.readFile(filePath, "utf8");
    return raw
      .split("\n")
      .filter(Boolean)
      .map((line) => JSON.parse(line) as T);
  } catch {
    return [];
  }
}

// ── Append CSV ─────────────────────────────────────────────────────────────

export async function appendCsv(
  filePath: string,
  headers: string[],
  row: (string | number | boolean | null | undefined)[],
): Promise<void> {
  initLocalStorage();
  let writeHeader = false;
  try {
    const stat = await fs.promises.stat(filePath);
    writeHeader = stat.size === 0;
  } catch {
    writeHeader = true; // File doesn't exist yet.
  }
  const lines: string[] = [];
  if (writeHeader) lines.push(headers.map(csvEscape).join(","));
  lines.push(row.map((v) => csvEscape(String(v ?? ""))).join(","));
  await fs.promises.appendFile(filePath, lines.join("\n") + "\n", "utf8");
}

function csvEscape(value: string): string {
  if (/[",\n\r]/.test(value)) return `"${value.replace(/"/g, '""')}"`;
  return value;
}

// ── Checksum ───────────────────────────────────────────────────────────────

export function sha256File(filePath: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash("sha256");
    const stream = fs.createReadStream(filePath);
    stream.on("data", (chunk) => hash.update(chunk));
    stream.on("end", () => resolve(hash.digest("hex")));
    stream.on("error", reject);
  });
}

export function sha256Buffer(buf: Buffer): string {
  return crypto.createHash("sha256").update(buf).digest("hex");
}

// ── Directory stats ────────────────────────────────────────────────────────

export interface StorageDirStats {
  fileCount: number;
  totalBytes: number;
  oldestMs: number | null;
  newestMs: number | null;
}

export async function dirStats(dirPath: string): Promise<StorageDirStats> {
  const result: StorageDirStats = { fileCount: 0, totalBytes: 0, oldestMs: null, newestMs: null };
  try {
    const entries = await fs.promises.readdir(dirPath, { withFileTypes: true });
    for (const entry of entries) {
      if (!entry.isFile()) continue;
      const stat = await fs.promises.stat(path.join(dirPath, entry.name));
      result.fileCount++;
      result.totalBytes += stat.size;
      const ms = stat.mtimeMs;
      if (result.oldestMs === null || ms < result.oldestMs) result.oldestMs = ms;
      if (result.newestMs === null || ms > result.newestMs) result.newestMs = ms;
    }
  } catch {
    // Directory may not exist yet.
  }
  return result;
}

export async function totalStorageBytes(): Promise<number> {
  const root = getDataRoot();
  let total = 0;
  async function walk(dir: string): Promise<void> {
    try {
      const entries = await fs.promises.readdir(dir, { withFileTypes: true });
      await Promise.all(
        entries.map(async (e) => {
          const full = path.join(dir, e.name);
          if (e.isDirectory()) return walk(full);
          if (e.isFile()) {
            const stat = await fs.promises.stat(full);
            total += stat.size;
          }
        }),
      );
    } catch {
      /* ignore */
    }
  }
  await walk(root);
  return total;
}

// ── List files in a category ───────────────────────────────────────────────

export async function listFiles(category: DataCategory): Promise<string[]> {
  const dir = path.join(getDataRoot(), DATA_DIRS[category]);
  try {
    const entries = await fs.promises.readdir(dir);
    return entries
      .filter((e) => !e.endsWith(".tmp"))
      .map((e) => path.join(dir, e));
  } catch {
    return [];
  }
}
