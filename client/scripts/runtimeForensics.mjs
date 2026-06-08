// Runtime forensics — comprehensive MongoDB writer map for Phase 31 certification.
// Run: node scripts/runtimeForensics.mjs
import { MongoClient } from "mongodb";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));

function loadEnv() {
  for (const rel of ["../.env.local", "../../.env"]) {
    try {
      const raw = readFileSync(join(__dirname, rel), "utf8");
      for (const line of raw.split(/\r?\n/)) {
        const m = line.match(/^([A-Z0-9_]+)=(.*)$/);
        if (m && !process.env[m[1]]) {
          process.env[m[1]] = m[2].replace(/^["']|["']$/g, "");
        }
      }
    } catch {}
  }
}
loadEnv();

const URI = process.env.MONGODB_URI;
const DB = process.env.MONGODB_DB || "loop_trades";

const PHASE31 = [
  "paper_state",
  "paper_positions",
  "paper_orders",
  "paper_trades",
  "equity_curve",
  "strategy_health",
];

const LEGACY = [
  "mock_trades",
  "mock_account_snapshots",
  "mock_trade_logs",
  "paper_oms_orders",
  "desk_worker_lease",
  "auth_sessions",
];

const TS_CANDIDATES = [
  "updated_at",
  "closed_at",
  "opened_at",
  "recorded_at",
  "snapped_at",
  "ts",
  "computed_at",
  "created_at",
  "sealed_at",
  "_id",
];

function pickTsField(docs) {
  if (!docs?.length) return null;
  const sample = docs[0];
  for (const f of TS_CANDIDATES) {
    if (sample[f] !== undefined) return f;
  }
  return "_id";
}

function fmtTs(v) {
  if (!v) return "—";
  if (v instanceof Date) return v.toISOString();
  if (typeof v === "number") return new Date(v).toISOString();
  if (v?.$date) return new Date(v.$date).toISOString();
  if (v?.getTimestamp) return v.getTimestamp().toISOString();
  return String(v);
}

async function auditCollection(db, name) {
  const out = { name, exists: true, count: 0, account_keys: [], newest: null, oldest: null, ts_field: null, sample: null, error: null };
  try {
    const cols = await db.listCollections({ name }).toArray();
    if (!cols.length) {
      out.exists = false;
      return out;
    }
    const c = db.collection(name);
    out.count = await c.estimatedDocumentCount();
    out.account_keys = await c.distinct("account_key").catch(() => []);
    if (out.count > 0) {
      const newestDoc = await c.find({}).sort({ _id: -1 }).limit(1).next();
      const oldestDoc = await c.find({}).sort({ _id: 1 }).limit(1).next();
      out.ts_field = pickTsField([newestDoc]);
      if (out.ts_field && out.ts_field !== "_id") {
        const n = await c.find({}).sort({ [out.ts_field]: -1 }).limit(1).next();
        const o = await c.find({}).sort({ [out.ts_field]: 1 }).limit(1).next();
        out.newest = fmtTs(n?.[out.ts_field]);
        out.oldest = fmtTs(o?.[out.ts_field]);
      } else {
        out.newest = fmtTs(newestDoc?._id);
        out.oldest = fmtTs(oldestDoc?._id);
      }
      const sample = await c.findOne({});
      out.sample = JSON.stringify(sample, null, 0).slice(0, 500);
    }
  } catch (e) {
    out.error = e.message;
  }
  return out;
}

async function main() {
  if (!URI) {
    console.error("MONGODB_URI not set");
    process.exit(1);
  }
  const client = new MongoClient(URI, { serverSelectionTimeoutMS: 10000 });
  await client.connect();
  const db = client.db(DB);

  console.log(`\n${"=".repeat(72)}`);
  console.log(`RUNTIME FORENSICS — MongoDB db=${DB} @ ${new Date().toISOString()}`);
  console.log(`${"=".repeat(72)}\n`);

  const allNames = [...PHASE31, ...LEGACY];
  const results = [];
  for (const n of allNames) {
    results.push(await auditCollection(db, n));
  }

  console.log("── PHASE 31 CANONICAL COLLECTIONS ──\n");
  for (const r of results.filter((x) => PHASE31.includes(x.name))) {
    printRow(r);
  }

  console.log("\n── LEGACY / PARALLEL COLLECTIONS ──\n");
  for (const r of results.filter((x) => LEGACY.includes(x.name))) {
    printRow(r);
  }

  // paper_state deep dive per account
  console.log("\n── paper_state PER ACCOUNT ──\n");
  const states = await db.collection("paper_state").find({}).toArray();
  for (const s of states) {
    console.log(`  account_key: ${s.account_key}`);
    console.log(`    balance: ${s.balance}`);
    console.log(`    positions[]: ${Array.isArray(s.positions) ? s.positions.length : "MISSING"}`);
    console.log(`    snapped_at: ${fmtTs(s.snapped_at)}`);
    console.log(`    updated_at: ${fmtTs(s.updated_at)}`);
    console.log(`    _id ts: ${fmtTs(s._id)}`);
    console.log("");
  }

  // Recent activity across all collections
  console.log("── RECENT WRITER ACTIVITY (by _id order, last doc) ──\n");
  const allColls = await db.listCollections().toArray();
  const writerMap = [];
  for (const { name } of allColls) {
    if (!name.startsWith("paper") && !name.startsWith("mock") && !name.startsWith("equity") && !name.startsWith("strategy") && !name.startsWith("desk")) continue;
    const c = db.collection(name);
    const count = await c.estimatedDocumentCount();
    if (count === 0) continue;
    const last = await c.find({}).sort({ _id: -1 }).limit(1).next();
    const ak = last?.account_key ?? last?.userId ?? "(no account_key)";
    const ts = last?._id?.getTimestamp?.() ?? null;
    writerMap.push({ collection: name, count, account_key: ak, last_write: ts?.toISOString() ?? "?", sample_id: String(last?._id) });
  }
  writerMap.sort((a, b) => (b.last_write > a.last_write ? 1 : -1));
  for (const w of writerMap) {
    console.log(`  ${w.collection.padEnd(24)} count=${String(w.count).padEnd(6)} acct=${w.account_key} last=${w.last_write}`);
  }

  await client.close();
  console.log(`\n${"=".repeat(72)}\n`);
}

function printRow(r) {
  if (!r.exists) {
    console.log(`[${r.name}] MISSING — collection does not exist`);
    return;
  }
  console.log(`[${r.name}]`);
  console.log(`  count      : ${r.count}`);
  console.log(`  ts_field   : ${r.ts_field ?? "—"}`);
  console.log(`  newest     : ${r.newest ?? "—"}`);
  console.log(`  oldest     : ${r.oldest ?? "—"}`);
  console.log(`  acct_keys  : ${r.account_keys.length ? r.account_keys.join(", ") : "(none)"}`);
  if (r.sample) console.log(`  sample     : ${r.sample}...`);
  if (r.error) console.log(`  error      : ${r.error}`);
  console.log("");
}

main().catch((e) => {
  console.error("FORENSICS FAILED:", e);
  process.exit(1);
});
