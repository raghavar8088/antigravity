// Phase 31C — MongoDB audit for Go Engine paper-trading collections.
// Run: node scripts/paperDeskAudit.mjs
import { MongoClient } from "mongodb";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));

// Minimal .env.local loader (no dotenv dependency).
function loadEnv() {
  try {
    const raw = readFileSync(join(__dirname, "..", ".env.local"), "utf8");
    for (const line of raw.split(/\r?\n/)) {
      const m = line.match(/^([A-Z0-9_]+)=(.*)$/);
      if (m && !process.env[m[1]]) {
        process.env[m[1]] = m[2].replace(/^["']|["']$/g, "");
      }
    }
  } catch {}
}
loadEnv();

const URI = process.env.MONGODB_URI;
const DB = process.env.MONGODB_DB || process.env.MONGODB_DB_NAME || "loop_trades";

const COLLECTIONS = [
  "paper_trades",
  "paper_positions",
  "paper_orders",
  "paper_state",
  "equity_curve",
  "daily_pnl_history",
  "strategy_scores",
  "strategy_health",
];

const TS_FIELD = {
  paper_trades: "closed_at",
  paper_positions: "opened_at",
  paper_orders: "recorded_at",
  paper_state: "snapped_at",
  equity_curve: "ts",
  daily_pnl_history: "sealed_at",
  strategy_scores: "computed_at",
  strategy_health: "computed_at",
};

async function main() {
  if (!URI) {
    console.error("MONGODB_URI not set — aborting audit");
    process.exit(1);
  }
  const client = new MongoClient(URI, { serverSelectionTimeoutMS: 8000 });
  await client.connect();
  const db = client.db(DB);
  console.log(`\n=== PHASE 31C MONGODB AUDIT — db=${DB} ===\n`);

  for (const col of COLLECTIONS) {
    const c = db.collection(col);
    const tsf = TS_FIELD[col];
    let count = 0;
    let newest = null;
    let oldest = null;
    let indexes = [];
    let accountKeys = [];
    try {
      count = await c.estimatedDocumentCount();
      if (count > 0) {
        const newestDoc = await c.find({}).sort({ [tsf]: -1 }).limit(1).next();
        const oldestDoc = await c.find({}).sort({ [tsf]: 1 }).limit(1).next();
        newest = newestDoc?.[tsf] ?? null;
        oldest = oldestDoc?.[tsf] ?? null;
      }
      indexes = (await c.indexes()).map((i) => i.name);
      accountKeys = await c.distinct("account_key");
    } catch (e) {
      console.log(`[${col}] ERROR: ${e.message}`);
      continue;
    }
    console.log(`── ${col} ────────────────────────────────`);
    console.log(`  documents : ${count}`);
    console.log(`  newest    : ${newest ? new Date(newest).toISOString() : "—"} (${tsf})`);
    console.log(`  oldest    : ${oldest ? new Date(oldest).toISOString() : "—"} (${tsf})`);
    console.log(`  indexes   : ${indexes.join(", ")}`);
    console.log(`  acct_keys : ${accountKeys.length ? accountKeys.join(", ") : "(none)"}`);
    console.log("");
  }

  // Spot-check a paper_state doc shape.
  const st = await db.collection("paper_state").findOne({});
  if (st) {
    console.log("── paper_state sample (field presence) ──");
    const fields = ["balance", "equity", "unrealized_pnl", "realized_pnl", "win_rate", "total_trades", "open_position_count", "total_exposure_btc", "max_drawdown"];
    for (const f of fields) {
      console.log(`  ${f.padEnd(22)}: ${st[f] === undefined ? "MISSING" : JSON.stringify(st[f])}`);
    }
  } else {
    console.log("── paper_state: no document (engine has not snapshotted yet) ──");
  }

  await client.close();
  console.log("\n=== AUDIT COMPLETE ===\n");
}

main().catch((e) => {
  console.error("AUDIT FAILED:", e.message);
  process.exit(1);
});
