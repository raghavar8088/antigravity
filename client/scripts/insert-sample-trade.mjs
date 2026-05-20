// Insert one realistic sample paper trade into Atlas to verify end-to-end persistence.
// Run from client/ dir:  node --env-file=.env.local scripts/insert-sample-trade.mjs
import { MongoClient } from "mongodb";
import { randomUUID } from "node:crypto";

const uri = process.env.MONGODB_URI;
const dbName = process.env.MONGODB_DB || "loop_trades";
if (!uri) { console.error("MONGODB_URI missing"); process.exit(1); }

const now = new Date();
const opened = new Date(now.getTime() - 15 * 60_000); // 15 min ago
const sample = {
  account_key: "SAMPLE_TEST_USER",
  client_trade_id: randomUUID(),
  opened_at: opened.toISOString(),
  closed_at: now.toISOString(),
  symbol: "BTCUSDT",
  strategy_id: 91,
  strategy_name: "BTC FT — Trend EMA20/50 LONG",
  side: "LONG",
  entry_price: 95_420.5,
  exit_price: 95_810.2,
  contracts: 0.01,
  notional: 954.205,
  margin_used: 47.71,
  gross_pnl: 3.897,
  fees: 0.477,
  funding_costs: 0.012,
  net_pnl: 3.408,
  exit_reason: "TP",
  payload: {
    note: "Manually inserted sample to verify MongoDB persistence pipeline",
    netPnlPct: 0.357,
    priceMovePct: 0.408,
    liquidationPrice: 86_320.0,
    liquidationDistancePct: 9.55,
  },
  created_at: now.toISOString(),
};

const client = new MongoClient(uri, { serverSelectionTimeoutMS: 8000 });
try {
  await client.connect();
  const col = client.db(dbName).collection("paper_trades");
  // Create the same indexes the app would create on first real write
  await Promise.all([
    col.createIndex({ client_trade_id: 1 }, { unique: true, name: "uniq_client_trade_id" }),
    col.createIndex({ account_key: 1, closed_at: -1 }, { name: "by_account_closed" }),
    col.createIndex({ account_key: 1, strategy_id: 1, closed_at: -1 }, { name: "by_account_strat_closed" }),
  ]);
  const r = await col.updateOne(
    { client_trade_id: sample.client_trade_id },
    { $setOnInsert: sample },
    { upsert: true },
  );
  console.log("Inserted:", r.upsertedId ? "YES — _id " + r.upsertedId.toString() : "no (already existed)");
  const total = await col.countDocuments();
  const recent = await col.find({}).sort({ closed_at: -1 }).limit(1).toArray();
  console.log("Total docs in paper_trades:", total);
  console.log("Most recent row:");
  console.log(JSON.stringify(recent[0], null, 2));
} catch (e) {
  console.error("FAIL:", e.message);
  process.exit(1);
} finally {
  await client.close();
}
