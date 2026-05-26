#!/usr/bin/env tsx
/**
 * update-ai-mindmap.ts
 *
 * Scans key project folders, counts files, updates timestamps in:
 *   - client/docs/ai-application-mindmap.json  (updatedAt field)
 *   - client/docs/AI_APPLICATION_MINDMAP.md     (Last Updated section)
 *
 * Does NOT rewrite the full documents — only patches the metadata fields.
 * Human-written sections are preserved.
 *
 * Usage:
 *   cd client
 *   npm run ai:mindmap
 */

import { readFileSync, writeFileSync, readdirSync, statSync } from "fs";
import { join, resolve } from "path";

const CLIENT_ROOT = resolve(__dirname, "..");
const JSON_PATH = join(CLIENT_ROOT, "docs/ai-application-mindmap.json");
const MD_PATH = join(CLIENT_ROOT, "docs/AI_APPLICATION_MINDMAP.md");

function countFiles(dir: string, ext: string): number {
  try {
    const entries = readdirSync(dir, { withFileTypes: true });
    let n = 0;
    for (const e of entries) {
      if (e.isDirectory()) {
        n += countFiles(join(dir, e.name), ext);
      } else if (e.name.endsWith(ext)) {
        n++;
      }
    }
    return n;
  } catch {
    return 0;
  }
}

function latestMtime(dir: string): number {
  try {
    const entries = readdirSync(dir, { withFileTypes: true });
    let latest = 0;
    for (const e of entries) {
      if (e.isDirectory()) {
        latest = Math.max(latest, latestMtime(join(dir, e.name)));
      } else {
        const s = statSync(join(dir, e.name));
        latest = Math.max(latest, s.mtimeMs);
      }
    }
    return latest;
  } catch {
    return 0;
  }
}

function main() {
  const now = new Date().toISOString().slice(0, 10);
  const timestamp = new Date().toISOString();

  // ── Scan key dirs ────────────────────────────────────────────────────────────
  const srcDir = join(CLIENT_ROOT, "src");
  const libDir = join(CLIENT_ROOT, "src/lib");
  const hooksDir = join(CLIENT_ROOT, "src/hooks");
  const scriptsDir = join(CLIENT_ROOT, "scripts");

  const tsxCount = countFiles(srcDir, ".tsx");
  const tsCount = countFiles(libDir, ".ts") + countFiles(hooksDir, ".ts");
  const scriptCount = countFiles(scriptsDir, ".ts");
  const testCount = countFiles(join(libDir, "tests"), ".ts");

  // ── Update JSON ───────────────────────────────────────────────────────────────
  let jsonChanged = false;
  try {
    const raw = readFileSync(JSON_PATH, "utf-8");
    const doc = JSON.parse(raw) as Record<string, unknown>;
    const prevDate = doc.updatedAt as string;
    doc.updatedAt = now;
    if (prevDate !== now) {
      writeFileSync(JSON_PATH, JSON.stringify(doc, null, 2) + "\n", "utf-8");
      jsonChanged = true;
      console.log(`[mindmap] JSON updated: updatedAt ${prevDate} → ${now}`);
    } else {
      console.log(`[mindmap] JSON already up-to-date (${now})`);
    }
  } catch (err) {
    console.error(`[mindmap] Could not update JSON: ${err instanceof Error ? err.message : err}`);
  }

  // ── Update MD last-updated block ─────────────────────────────────────────────
  let mdChanged = false;
  try {
    let md = readFileSync(MD_PATH, "utf-8");
    const datePattern = /^Date:\s+\S+/m;
    const newDateLine = `Date:    ${now}`;
    if (datePattern.test(md)) {
      const updated = md.replace(datePattern, newDateLine);
      if (updated !== md) {
        writeFileSync(MD_PATH, updated, "utf-8");
        mdChanged = true;
        console.log(`[mindmap] MD date updated to ${now}`);
      } else {
        console.log(`[mindmap] MD already up-to-date (${now})`);
      }
    } else {
      console.warn("[mindmap] Could not find 'Date:' line in MD — no patch applied");
    }
  } catch (err) {
    console.error(`[mindmap] Could not update MD: ${err instanceof Error ? err.message : err}`);
  }

  // ── Summary ──────────────────────────────────────────────────────────────────
  console.log(`\n[mindmap] Scan summary (${timestamp})`);
  console.log(`  .tsx files in src/          : ${tsxCount}`);
  console.log(`  .ts  files in lib+hooks     : ${tsCount}`);
  console.log(`  .ts  scripts                : ${scriptCount}`);
  console.log(`  .ts  test files             : ${testCount}`);
  console.log(`  JSON changed                : ${jsonChanged}`);
  console.log(`  MD changed                  : ${mdChanged}`);
  console.log("\nDone. Human-written sections were preserved.");
}

main();
