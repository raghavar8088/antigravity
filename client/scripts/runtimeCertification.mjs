// Phase 31 runtime certification — negative balance + writer forensics
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
        if (m && !process.env[m[1]]) process.env[m[1]] = m[2].replace(/^["']|["']$/g, "");
      }
    } catch {}
  }
}
loadEnv();

const URI = process.env.MONGODB_URI;
const DB = process.env.MONGODB_DB || "loop_trades";
const ANON = "anon_e7da5e39-2185-4006-a990-62b5b357feb1";
const OWNER = "mock_trading_main";

async function main() {
  const client = new MongoClient(URI);
  await client.connect();
  const db = client.db(DB);

  console.log("\n=== NEGATIVE BALANCE FORENSICS ===\n");
  const anonState = await db.collection("paper_state").findOne({ account_key: ANON });
  console.log("anon paper_state:", JSON.stringify(anonState, null, 2));

  // Trades in any collection for anon
  for (const col of ["paper_trades", "mock_trades", "paper_trades_archive"]) {
    try {
      const n = await db.collection(col).countDocuments({ account_key: ANON });
      const n2 = await db.collection(col).countDocuments({ account_key: { $regex: "^anon_" } });
      console.log(`${col}: anon_exact=${n} anon_prefix=${n2}`);
      if (n > 0) {
        const docs = await db.collection(col).find({ account_key: ANON }).sort({ _id: -1 }).limit(3).toArray();
        docs.forEach((d) => console.log("  sample:", JSON.stringify(d).slice(0, 300)));
      }
    } catch (e) {
      console.log(`${col}: ${e.message}`);
    }
  }

  // Owner mock_trades fee trace for negative balance comparison
  const mockTrades = await db.collection("mock_trades").find({ account_key: OWNER }).sort({ closed_at: -1 }).limit(7).toArray();
  let totalFees = 0, totalPnl = 0;
  for (const t of mockTrades) {
    totalFees += t.fees ?? 0;
    totalPnl += t.realized_pnl ?? t.pnl ?? 0;
    console.log(`mock_trade ${t.trade_id}: pnl=${t.realized_pnl ?? t.pnl} fees=${t.fees} margin=${t.margin_used}`);
  }
  console.log(`mock_trades sum: fees=${totalFees} pnl=${totalPnl}`);

  console.log("\n=== OWNER paper_state positions ===\n");
  const ownerState = await db.collection("paper_state").findOne({ account_key: OWNER });
  console.log("balance:", ownerState?.balance);
  console.log("positions:", JSON.stringify(ownerState?.positions, null, 2)?.slice(0, 800));

  console.log("\n=== paper_oms_orders writer fingerprint ===\n");
  const oms = await db.collection("paper_oms_orders").find({}).sort({ _id: -1 }).limit(3).toArray();
  oms.forEach((o) => console.log(JSON.stringify(o).slice(0, 400)));

  console.log("\n=== ALL COLLECTIONS WITH anon_ keys ===\n");
  const colls = await db.listCollections().toArray();
  for (const { name } of colls) {
    try {
      const keys = await db.collection(name).distinct("account_key", { account_key: { $regex: "^anon_" } });
      if (keys.length) console.log(`${name}: ${keys.join(", ")}`);
    } catch {}
  }

  await client.close();
}

main().catch((e) => { console.error(e); process.exit(1); });
