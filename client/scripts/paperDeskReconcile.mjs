// Phase 31C — account_key reconciliation: what will the dashboard (mock_trading_default) actually show?
import { MongoClient } from "mongodb";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
const __dirname = dirname(fileURLToPath(import.meta.url));
function loadEnv() {
  try {
    const raw = readFileSync(join(__dirname, "..", ".env.local"), "utf8");
    for (const line of raw.split(/\r?\n/)) {
      const m = line.match(/^([A-Z0-9_]+)=(.*)$/);
      if (m && !process.env[m[1]]) process.env[m[1]] = m[2].replace(/^["']|["']$/g, "");
    }
  } catch {}
}
loadEnv();
const URI = process.env.MONGODB_URI;
const DB = process.env.MONGODB_DB || "loop_trades";
const KEY = "mock_trading_default"; // session userId / OWNER_ACCOUNT_KEY
const ENGINE_FALLBACK = "owner_admin";

const cols = ["paper_trades","paper_positions","paper_orders","paper_state","equity_curve","daily_pnl_history","strategy_scores","strategy_health"];
const mockCols = ["mock_trades","mock_account_snapshots","mock_trade_logs"];

async function safeCount(db, col, filter) {
  try { return await db.collection(col).countDocuments(filter); } catch { return -1; }
}

async function main() {
  const client = new MongoClient(URI, { serverSelectionTimeoutMS: 8000 });
  await client.connect();
  const db = client.db(DB);
  console.log(`\n=== RECONCILE — dashboard key="${KEY}" vs engine-fallback="${ENGINE_FALLBACK}" ===\n`);
  console.log("collection            | docs(mock_trading_default) | docs(owner_admin) | total");
  console.log("----------------------|----------------------------|-------------------|------");
  for (const c of cols) {
    const a = await safeCount(db, c, { account_key: KEY });
    const b = await safeCount(db, c, { account_key: ENGINE_FALLBACK });
    const t = await safeCount(db, c, {});
    const fmt = (n) => (n === -1 ? "n/a(no ns)" : String(n));
    console.log(`${c.padEnd(21)} | ${fmt(a).padStart(26)} | ${fmt(b).padStart(17)} | ${fmt(t)}`);
  }
  console.log("\n--- Mock Trading Dashboard collections (must remain SEPARATE / untouched) ---");
  for (const c of mockCols) {
    const t = await safeCount(db, c, {});
    console.log(`  ${c.padEnd(24)}: ${t === -1 ? "(no ns)" : t} docs`);
  }
  await client.close();
  console.log("\n=== RECONCILE COMPLETE ===\n");
}
main().catch((e) => { console.error("FAILED:", e.message); process.exit(1); });
